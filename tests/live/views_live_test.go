//go:build live

package live

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// MCP Apps views (ADR-022): show, and the tool only views call, from a client
// that renders views and from one that does not.

// viewsSession is a session with token from a client that renders MCP Apps
// views.
func viewsSession(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	return viewsSessionNamed(t, "live", token)
}

// viewsSessionNamed is viewsSession from a client that gives name as its own.
func viewsSessionNamed(t *testing.T, name, token string) *mcp.ClientSession {
	t.Helper()
	capabilities := &mcp.ClientCapabilities{}
	capabilities.AddExtension(views.Extension, map[string]any{"mimeTypes": []string{views.MIMEType}})
	return mcpNamed(t, name, config.Config{URL: liveURL, Token: token}, &mcp.ClientOptions{Capabilities: capabilities})
}

// sameAnswer fails the test when two answers differ.
func sameAnswer(t *testing.T, what string, got, want server.PostWithThread) {
	t.Helper()
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("%s answered\n%s\nwhere read_post answered\n%s", what, gotJSON, wantJSON)
	}
}

func TestShowAnswersAsReadPostAndTheViewPagesOnAsItDoes(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other, third := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other, third)
	channel := seedChannel(t, admin, team, user, other)
	author := clientAs(t, other)
	root := postAs(t, author, channel.Id, "", "The release moved to **Thursday**, @"+user.Username+" ~"+channel.Name)
	_, _, err := clientAs(t, user).SaveReaction(t.Context(), &model.Reaction{UserId: user.Id, PostId: root.Id, EmojiName: "tada"})
	check(t, err)
	uploaded, _, err := author.UploadFile(t.Context(), picture(t), channel.Id, "chart.png")
	check(t, err)
	_, _, err = author.CreatePost(t.Context(), &model.Post{ChannelId: channel.Id, RootId: root.Id, Message: "the chart", FileIds: []string{uploaded.FileInfos[0].Id}})
	check(t, err)
	for i := range 12 {
		postAs(t, author, channel.Id, root.Id, fmt.Sprintf("reply %d", i+1))
	}
	session := viewsSession(t, personalAccessToken(t, admin, user.Id).Token)

	// show answers with what read_post answers for the same post and page,
	// and tells the model only what it showed.
	result := callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": root.Id, "limit": 5}})
	var shown server.PostWithThread
	structured(t, result, &shown)
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.HasPrefix(text, "Shown to the person") || !strings.Contains(text, "5 of its 14 posts") || strings.Contains(text, "Thursday") {
		t.Errorf("show told the model %q", text)
	}
	var read server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": root.Id, "limit": 5}}), &read)
	sameAnswer(t, "show", shown, read)
	if len(shown.Thread) != 5 || shown.NextCursor == "" || shown.Thread[0].ReplyCount != 13 || len(shown.Thread[0].Reactions["tada"]) != 1 {
		t.Fatalf("show answered %+v", shown)
	}

	// More of the thread, as the view asks for it, is read_post's next page.
	var more, next server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "view_action", Arguments: map[string]any{
		"action": "more", "post_id": root.Id, "cursor": shown.NextCursor, "limit": 5,
	}}), &more)
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": root.Id, "cursor": shown.NextCursor, "limit": 5}}), &next)
	sameAnswer(t, "view_action", more, next)
	if len(more.Thread) != 5 || more.Thread[0].Message != "reply 4" {
		t.Errorf("the next page is %+v", more.Thread)
	}

	// The pictures the view draws are resources it reads: the author's, and
	// the image's preview.
	if _, err := readResource(t, session, "picture", liveResource("/users/"+shown.Thread[0].AuthorID+"/image")); err != nil {
		t.Errorf("the author's picture: %v", err)
	}
	image := shown.Thread[1].Files
	if len(image) != 1 || image[0].PreviewURI == "" {
		t.Fatalf("the reply with the chart has %+v", image)
	}
	if _, err := readResource(t, session, "preview", image[0].PreviewURI); err != nil {
		t.Errorf("the chart's preview: %v", err)
	}

	// In a group message, which mm-mcp names after the people in it, a post
	// alone and a thread's next page are what read_post answers too.
	group, _, err := clientAs(t, user).CreateGroupChannel(t.Context(), []string{user.Id, other.Id, third.Id})
	check(t, err)
	question := postAs(t, author, group.Id, "", "lunch?")
	postAs(t, author, group.Id, question.Id, "at noon")
	postAs(t, author, group.Id, question.Id, "or one")
	var alone, readAlone server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": question.Id, "include_thread": false}}), &alone)
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": question.Id, "include_thread": false}}), &readAlone)
	sameAnswer(t, "show of a post alone", alone, readAlone)
	if alone.Post == nil || len(alone.Thread) != 0 || !strings.Contains(alone.Post.Channel, third.Username) {
		t.Errorf("show of a post alone answered %+v", alone)
	}
	var groupPage server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": question.Id, "limit": 2}}), &groupPage)
	var groupMore, groupNext server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "view_action", Arguments: map[string]any{
		"action": "more", "post_id": question.Id, "cursor": groupPage.NextCursor, "limit": 2,
	}}), &groupMore)
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": question.Id, "cursor": groupPage.NextCursor, "limit": 2}}), &groupNext)
	sameAnswer(t, "view_action in a group message", groupMore, groupNext)
	if len(groupMore.Thread) != 1 || groupMore.Thread[0].Message != "or one" {
		t.Errorf("the group message's next page is %+v", groupMore.Thread)
	}

	// The page is there for the host to read.
	page, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: views.URI})
	check(t, err)
	if len(page.Contents) != 1 || page.Contents[0].MIMEType != views.MIMEType || !strings.Contains(page.Contents[0].Text, liveResource("")) {
		t.Errorf("the page reads as %d contents", len(page.Contents))
	}
}

func TestAClientWithoutViewsIsShownNothingAndMattermostIsNotRead(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	root := postAs(t, clientAs(t, user), channel.Id, "", "nothing to see")
	session := sessionFor(t, admin, user)

	result := callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": root.Id}})
	if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "renders no MCP Apps views") {
		t.Errorf("show answered %+v", result.Content)
	}
	if reached := lastReached(t, session); len(reached) != 0 {
		t.Errorf("show read %v for a client that renders no views", reached)
	}
	listed, err := session.ListTools(t.Context(), nil)
	check(t, err)
	for _, tool := range listed.Tools {
		if tool.Name == "view_action" {
			t.Error("a client that renders no views is offered view_action")
		}
	}
}

func TestDiagnoseSaysWhetherTheClientRendersViews(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	token := personalAccessToken(t, admin, user.Id).Token
	if got := checkNamed(t, diagnosed(t, mcpAs(t, token), token, false), "views"); !strings.Contains(got.Detail, "renders no MCP Apps views") {
		t.Errorf("without views: %+v", got)
	}
	if got := checkNamed(t, diagnosed(t, viewsSession(t, token), token, false), "views"); !strings.Contains(got.Detail, "renders MCP Apps views, so show") {
		t.Errorf("with views: %+v", got)
	}
}

// A post that does not exist, or that the user cannot see, is refused by
// read_post and by show alike, alone or with its thread.
func TestAPostThatIsNotThereIsRefusedAloneAndWithItsThread(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	session := viewsSession(t, personalAccessToken(t, admin, user.Id).Token)
	missing := model.NewId()
	for _, params := range []*mcp.CallToolParams{
		{Name: "read_post", Arguments: map[string]any{"post_id": missing, "include_thread": false}},
		{Name: "read_post", Arguments: map[string]any{"post_id": missing}},
		{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": missing}},
	} {
		result := callTool(t, session, params)
		if !result.IsError {
			t.Errorf("%s %v answered %+v", params.Name, params.Arguments, result.StructuredContent)
		}
	}
}

// Every tool that posts shows the post as posted (ADR-022): it links the page,
// and answers with the post, under where it went, which the view draws.
func TestEveryToolThatPostsShowsThePostAsPosted(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, first, second := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, first, second)
	channel := seedChannel(t, admin, team, user, first)
	session := writingViewsSession(t, personalAccessToken(t, admin, user.Id).Token)

	listed, err := session.ListTools(t.Context(), nil)
	check(t, err)
	linked := map[string]bool{}
	for _, tool := range listed.Tools {
		ui, _ := tool.Meta["ui"].(map[string]any)
		linked[tool.Name] = ui["resourceUri"] == views.URI
	}
	for _, c := range []struct {
		tool      string
		arguments map[string]any
		channel   string
	}{
		{"create_post", map[string]any{"channel_id": channel.Id, "message": "The notes are ready.", "reaches": members(t, channel.Id)}, channel.DisplayName},
		{"dm", map[string]any{"username": first.Username, "message": "A word in private."}, first.Username},
		{"group_message", map[string]any{"usernames": []string{first.Username, second.Username}, "message": "Kickoff at ten."}, ""},
	} {
		if !linked[c.tool] {
			t.Errorf("%s does not link the page", c.tool)
		}
		var posted server.Post
		structured(t, callTool(t, session, &mcp.CallToolParams{Name: c.tool, Arguments: c.arguments}), &posted)
		if posted.ID == "" || posted.AuthorID != user.Id || posted.URL == "" || posted.ChannelURL == "" || posted.Message != c.arguments["message"] {
			t.Errorf("%s answered %+v; the view draws the post under where it went", c.tool, posted)
		}
		if c.channel != "" && !strings.Contains(posted.Channel, c.channel) {
			t.Errorf("%s names the channel %q; want %q", c.tool, posted.Channel, c.channel)
		}
	}
}

// A view reads its pictures as resources, through the host, and no result a
// view is drawn from carries a picture's bytes: some clients hand a result to
// their model, whose context megabytes of an image would fill (#39, ADR-022).
func TestNoAnswerAViewDrawsCarriesAPicturesBytes(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	author := clientAs(t, other)
	_, err := author.SetProfileImage(t.Context(), other.Id, picture(t))
	check(t, err)
	root, _ := attachAs(t, author, channel.Id, "chart.png", picture(t))
	postAs(t, author, channel.Id, root.Id, "and the numbers")
	session := writingViewsSession(t, personalAccessToken(t, admin, user.Id).Token)

	png := base64.StdEncoding.EncodeToString(picture(t))[:24]
	for _, call := range []*mcp.CallToolParams{
		{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": root.Id}},
		{Name: "show", Arguments: map[string]any{"kind": "unreads"}},
		{Name: "view_action", Arguments: map[string]any{"action": "thread", "post_id": root.Id}},
	} {
		result := callTool(t, session, call)
		if result.IsError {
			t.Fatalf("%s failed: %s", call.Name, errorText(result))
		}
		// Text, and links to the files a post holds, which a client reads as
		// resources when it chooses.
		for _, content := range result.Content {
			switch content.(type) {
			case *mcp.TextContent, *mcp.ResourceLink:
			default:
				t.Errorf("%s %v answers with %T; a view's answer is text, links and structured data", call.Name, call.Arguments, content)
			}
		}
		raw, err := json.Marshal(result)
		check(t, err)
		if strings.Contains(string(raw), png) || strings.Contains(string(raw), `"blob"`) {
			t.Errorf("%s %v carries a picture's bytes", call.Name, call.Arguments)
		}
	}
}

// A view shows a screenful, and says when Mattermost has more: a thread's
// first 30 posts, and at most 20 channels with something unread, however
// many there are, so a busy Mattermost cannot make a view read without end
// (#39, ADR-022).
func TestAViewShowsAScreenfulAndSaysMattermostHasMore(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	own, author := clientAs(t, user), clientAs(t, other)
	var channels []*model.Channel
	for range 22 {
		channel := seedChannel(t, admin, team, user, other)
		_, _, err := own.ViewChannel(t.Context(), user.Id, &model.ChannelView{ChannelId: channel.Id})
		check(t, err)
		channels = append(channels, channel)
	}
	for _, channel := range channels {
		postAs(t, author, channel.Id, "", "news in "+channel.Name)
	}
	root := postAs(t, author, channels[0].Id, "", "A long thread")
	for i := range 34 {
		postAs(t, author, channels[0].Id, root.Id, fmt.Sprintf("reply %d", i+1))
	}
	session := viewsSession(t, personalAccessToken(t, admin, user.Id).Token)

	var unreads server.Unreads
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "unreads"}}), &unreads)
	if len(unreads.Channels.Channels) != 20 || unreads.Channels.NextCursor == "" {
		t.Errorf("the unread view lists %d of 22 channels, saying Mattermost has more: %v", len(unreads.Channels.Channels), unreads.Channels.NextCursor != "")
	}
	var thread server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": root.Id}}), &thread)
	if len(thread.Thread) != 30 || thread.NextCursor == "" {
		t.Errorf("the thread view shows %d of 35 posts, saying Mattermost has more: %v", len(thread.Thread), thread.NextCursor != "")
	}
}
