//go:build live

package live

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The unread view (ADR-022): show with kind unreads answers with what the
// three tools it draws from answer, and what a person does in it, opening a
// channel or a thread in place and marking a channel read, answers as the
// tools that do the same.

// writingViewsSession is a session with token from a client that renders
// MCP Apps views, on a server that allows writes.
func writingViewsSession(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	capabilities := &mcp.ClientCapabilities{}
	capabilities.AddExtension(views.Extension, map[string]any{"mimeTypes": []string{views.MIMEType}})
	return mcpWith(t, config.Config{URL: liveURL, Token: token, AllowWrites: true}, &mcp.ClientOptions{Capabilities: capabilities})
}

// answer calls a tool and answers with its structured answer, as JSON.
func answer(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) string {
	t.Helper()
	result := callTool(t, session, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if result.IsError {
		t.Fatalf("%s failed: %s", name, errorText(result))
	}
	data, err := json.Marshal(result.StructuredContent)
	check(t, err)
	return string(data)
}

func TestShowUnreadsAnswersAsTheToolsItDrawsFrom(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	quiet, busy := seedChannel(t, admin, team, user, other), seedChannel(t, admin, team, user, other)
	tag := word(t)
	own, author := clientAs(t, user), clientAs(t, other)
	// The user has read both channels; what follows is unread.
	for _, channel := range []*model.Channel{quiet, busy} {
		_, _, err := own.ViewChannel(t.Context(), user.Id, &model.ChannelView{ChannelId: channel.Id})
		check(t, err)
	}
	root := postAs(t, own, quiet.Id, "", "Who takes the "+tag+" notes?")
	postAs(t, author, quiet.Id, root.Id, "I do, "+tag)
	postAs(t, author, busy.Id, "", "@"+user.Username+" the "+tag+" build is red")
	postAs(t, author, busy.Id, "", "and the "+tag+" deploy waits")
	direct, _, err := author.CreateDirectChannel(t.Context(), other.Id, user.Id)
	check(t, err)
	postAs(t, author, direct.Id, "", "a word about "+tag)
	session := writingViewsSession(t, personalAccessToken(t, admin, user.Id).Token)

	// Search is indexed asynchronously: wait for the mention to be found.
	eventually(t, 30*time.Second, func() (bool, string) {
		found := answer(t, session, "list_mentions", map[string]any{"limit": 10})
		return strings.Contains(found, tag+" build is red"), found
	})

	result := callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "unreads"}})
	var shown server.Unreads
	structured(t, result, &shown)
	mustContain(t, "what show tells the model", errorText(result), "Shown to the person", "with something unread", "followed thread")
	// part is a part of the answer as the tools' answers read, through JSON.
	part := func(value any) string {
		data, err := json.Marshal(value)
		check(t, err)
		var decoded any
		check(t, json.Unmarshal(data, &decoded))
		data, err = json.Marshal(decoded)
		check(t, err)
		return string(data)
	}
	for _, c := range []struct {
		what, got, want string
	}{
		{"its channels", part(shown.Channels), answer(t, session, "get_user_channels", map[string]any{"unread_only": true, "limit": 20})},
		{"its mentions", part(shown.Mentions), answer(t, session, "list_mentions", map[string]any{"limit": 10})},
		{"its threads", part(shown.Threads), answer(t, session, "list_threads", map[string]any{"unread_only": true, "limit": 10})},
	} {
		if c.got != c.want {
			t.Errorf("show's unread view answered for %s\n%s\nwhere the tool answered\n%s", c.what, c.got, c.want)
		}
	}
	listed := map[string]server.Channel{}
	for _, channel := range shown.Channels.Channels {
		listed[channel.ID] = channel
	}
	for _, channel := range []*model.Channel{quiet, busy, direct} {
		if found, ok := listed[channel.Id]; !ok || found.Unread == nil || *found.Unread == 0 {
			t.Errorf("the unread view lists %s as %+v", channel.Name, found)
		}
	}
	if found := listed[busy.Id]; found.Mentions == nil || *found.Mentions != 1 {
		t.Errorf("the busy channel counts %v mentions; want 1", found.Mentions)
	}
	if !slices.ContainsFunc(shown.Threads.Threads, func(thread server.ThreadSummary) bool { return thread.RootID == root.Id }) {
		t.Errorf("the unread view lists no thread %s: %+v", root.Id, shown.Threads.Threads)
	}

	// A channel opened in place reads as read_unread reads it, and a thread
	// as read_post does, a view's page at a time.
	if got, want := answer(t, session, "view_action", map[string]any{"action": "unread", "channel_id": busy.Id}),
		answer(t, session, "read_unread", map[string]any{"channel_id": busy.Id}); got != want {
		t.Errorf("a channel opened in place answered\n%s\nwhere read_unread answered\n%s", got, want)
	}
	if got, want := answer(t, session, "view_action", map[string]any{"action": "thread", "post_id": root.Id}),
		answer(t, session, "read_post", map[string]any{"post_id": root.Id, "limit": 30}); got != want {
		t.Errorf("a thread opened in place answered\n%s\nwhere read_post answered\n%s", got, want)
	}

	// Mark read marks the channel read as mark_channel_read does.
	if marked := callTool(t, session, &mcp.CallToolParams{Name: "view_change", Arguments: map[string]any{"action": "mark_read", "channel_id": busy.Id}}); marked.IsError {
		t.Fatalf("Mark read failed: %s", errorText(marked))
	}
	var after server.Channels
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "get_user_channels", Arguments: map[string]any{"unread_only": true}}), &after)
	if slices.ContainsFunc(after.Channels, func(channel server.Channel) bool { return channel.ID == busy.Id }) {
		t.Errorf("the channel marked read from the view is still unread: %+v", after.Channels)
	}

	// A thread is shown for a post only.
	refused := callTool(t, session, &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread"}})
	if !refused.IsError || !strings.Contains(errorText(refused), "give post_id") {
		t.Errorf("show without a post answered %s", errorText(refused))
	}
}

// Mark read is offered only where writes are allowed.
func TestTheViewsChangeIsOfferedOnlyWhereWritesAre(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	token := personalAccessToken(t, admin, user.Id).Token
	for _, c := range []struct {
		session *mcp.ClientSession
		writes  bool
	}{{viewsSession(t, token), false}, {writingViewsSession(t, token), true}} {
		listed, err := c.session.ListTools(t.Context(), nil)
		check(t, err)
		offered := slices.ContainsFunc(listed.Tools, func(tool *mcp.Tool) bool { return tool.Name == "view_change" })
		if offered != c.writes {
			t.Errorf("writes allowed %v: view_change offered %v", c.writes, offered)
		}
	}
}
