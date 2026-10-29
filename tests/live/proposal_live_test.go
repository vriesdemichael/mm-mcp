//go:build live

package live

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// A proposal's view (ADR-035): propose_post from a client that renders
// views, and what the person presses in the view, through view_change, read
// back from Mattermost as the person.

// proposingSession is a session acting as user, from a client that renders
// views, on a server that allows writes, marks what it posts as written with
// AI and posts from a view, unless posts says otherwise.
func proposingSession(t *testing.T, admin *model.Client4, user *model.User, posts bool) *mcp.ClientSession {
	t.Helper()
	capabilities := &mcp.ClientCapabilities{}
	capabilities.AddExtension(views.Extension, map[string]any{"mimeTypes": []string{views.MIMEType}})
	return mcpWith(t, config.Config{
		URL: liveURL, Token: personalAccessToken(t, admin, user.Id).Token, AllowWrites: true, MarkAIGenerated: true, PostFromViews: posts,
	}, &mcp.ClientOptions{Capabilities: capabilities})
}

// pressed calls view_change, as the view does for a press, and answers with
// what came of it.
func pressed(t *testing.T, session *mcp.ClientSession, arguments map[string]any) server.DraftChange {
	t.Helper()
	var change server.DraftChange
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "view_change", Arguments: arguments}), &change)
	return change
}

func TestAProposalIsShownAndPostedOnThePersonsPress(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other, third := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other, third)
	channel := seedChannel(t, admin, team, user, other)
	root := postAs(t, clientAs(t, other), channel.Id, "", "Who takes the on-call shift?")
	direct, _, err := clientAs(t, user).CreateDirectChannel(t.Context(), user.Id, other.Id)
	check(t, err)
	group, _, err := clientAs(t, user).CreateGroupChannel(t.Context(), []string{user.Id, other.Id, third.Id})
	check(t, err)
	session := proposingSession(t, admin, user, true)

	result := proposePost(t, session, map[string]any{"root_id": root.Id, "message": "I can take it"})
	if text := errorText(result); !strings.Contains(text, "shown to them as it will be posted") {
		t.Errorf("a client that renders views is told %q", text)
	}
	var proposal server.Proposal
	structured(t, result, &proposal)
	if !proposal.Saved || proposal.RootID != root.Id || proposal.Theirs != nil {
		t.Fatalf("the proposal is %+v", proposal)
	}

	// Post: the draft as the view shows it, marked as written with AI, and
	// the draft gone with it.
	posted := pressed(t, session, map[string]any{"action": "post", "channel_id": channel.Id, "root_id": root.Id, "shown": "I can take it"})
	if posted.Status != "posted" || posted.Post.Message != "I can take it" || posted.Post.RootID != root.Id || !posted.Post.AIGenerated {
		t.Fatalf("Post answered %+v", posted)
	}
	stored, _, err := admin.GetPost(t.Context(), posted.Post.ID, "")
	check(t, err)
	if stored.GetProp("ai_generated_by") != user.Id || stored.GetProp("mm_mcp_proposal") != nil {
		t.Errorf("the post carries %v; want the AI marker alone", stored.GetProps())
	}
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"+root.Id]; got != "" {
		t.Errorf("the posted draft is still there: %q", got)
	}
	// A second press finds nothing, and posts nothing.
	if again := pressed(t, session, map[string]any{"action": "post", "channel_id": channel.Id, "root_id": root.Id, "shown": "I can take it"}); again.Status != "gone" {
		t.Errorf("a second press answered %+v", again)
	}
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 2 {
		t.Errorf("the channel holds %d posts; want the question and the one reply", len(posts))
	}

	// Discard, in a direct message.
	proposePost(t, session, map[string]any{"channel_id": direct.Id, "message": "see you at standup"})
	if discarded := pressed(t, session, map[string]any{"action": "discard", "channel_id": direct.Id, "shown": "see you at standup"}); discarded.Status != "discarded" {
		t.Errorf("Discard answered %+v", discarded)
	}
	if got := draftsOf(t, user, team.Id)[direct.Id+"/"]; got != "" {
		t.Errorf("the discarded draft is still there: %q", got)
	}
	if posts := messagesIn(t, admin, direct.Id); len(posts) != 0 {
		t.Errorf("discarding posted %d messages", len(posts))
	}

	// Post, in a group message, named after the others in it.
	proposePost(t, session, map[string]any{"channel_id": group.Id, "message": "kickoff at ten"})
	grouped := pressed(t, session, map[string]any{"action": "post", "channel_id": group.Id, "shown": "kickoff at ten"})
	if grouped.Status != "posted" || !strings.Contains(grouped.Post.Channel, other.Username) || !strings.Contains(grouped.Post.Channel, third.Username) {
		t.Errorf("Post in a group message answered %+v", grouped)
	}
}

func TestAPressActsOnlyOnTheDraftTheViewShows(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel, attached, theirs := seedChannel(t, admin, team, user), seedChannel(t, admin, team, user), seedChannel(t, admin, team, user)
	own := clientAs(t, user)
	session := proposingSession(t, admin, user, true)
	at := func(more map[string]any) map[string]any {
		arguments := map[string]any{"channel_id": channel.Id}
		for key, value := range more {
			arguments[key] = value
		}
		return arguments
	}

	// Typed on in Mattermost while the view showed it, the draft is the
	// person's: no press reaches it, whatever text it names, and nothing is
	// posted. It answers with the draft as it is now.
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "Lunch at noon"})
	proposal := draftIn(t, user, team.Id, channel.Id, "")
	proposal.Message = "Lunch at noon, and dessert"
	_, _, err := own.UpsertDraft(t.Context(), proposal)
	check(t, err)
	for _, press := range []map[string]any{
		{"action": "post", "shown": "Lunch at noon"},
		{"action": "post", "shown": "Lunch at noon, and dessert"},
		{"action": "edit", "shown": "Lunch at noon, and dessert", "message": "Lunch at one"},
		{"action": "discard", "shown": "Lunch at noon, and dessert"},
	} {
		if left := pressed(t, session, at(press)); left.Status != "elsewhere" || left.Draft.Message != "Lunch at noon, and dessert" {
			t.Errorf("%v on a draft typed on in Mattermost answered %+v", press, left)
		}
	}
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Fatalf("a press on a draft typed on in Mattermost posted %d messages", len(posts))
	}
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; got != "Lunch at noon, and dessert" {
		t.Fatalf("the draft typed on in Mattermost reads %q", got)
	}

	// A newer proposal than the view showed is drawn again as it is.
	_, _, err = own.DeleteDraft(t.Context(), user.Id, channel.Id, "")
	check(t, err)
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "Lunch at noon"})
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "Lunch at one"})
	if changed := pressed(t, session, at(map[string]any{"action": "post", "shown": "Lunch at noon"})); changed.Status != "changed" || changed.Draft.Message != "Lunch at one" {
		t.Errorf("a press on a newer proposal answered %+v", changed)
	}

	// Edited in the view, the draft is the person's, which a proposal over it
	// leaves for their choice, and still the view's, which its presses reach.
	edited := pressed(t, session, at(map[string]any{"action": "edit", "shown": "Lunch at one", "message": "Lunch at half past one"}))
	if edited.Status != "saved" || edited.Draft.Message != "Lunch at half past one" {
		t.Fatalf("Edit answered %+v", edited)
	}
	if mark, _ := draftIn(t, user, team.Id, channel.Id, "").GetProps()["mm_mcp_proposal"].(string); !strings.HasPrefix(mark, "edited ") {
		t.Errorf("the edited draft is marked %q", mark)
	}
	var choice server.Proposal
	structured(t, proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "Lunch at two"}), &choice)
	if choice.Saved || choice.Theirs == nil || choice.Theirs.Message != "Lunch at half past one" || choice.Message != "Lunch at two" {
		t.Fatalf("a proposal over the person's draft answered %+v", choice)
	}
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; got != "Lunch at half past one" {
		t.Fatalf("a proposal over the person's draft wrote %q", got)
	}

	// Replace mine puts the proposal in its place, as mm-mcp's own again.
	replaced := pressed(t, session, at(map[string]any{"action": "replace", "shown": "Lunch at half past one", "message": "Lunch at two"}))
	if replaced.Status != "saved" || replaced.Draft.Message != "Lunch at two" {
		t.Fatalf("Replace mine answered %+v", replaced)
	}
	if mark, _ := draftIn(t, user, team.Id, channel.Id, "").GetProps()["mm_mcp_proposal"].(string); mark == "" || strings.HasPrefix(mark, "edited ") {
		t.Errorf("the proposal put in place is marked %q", mark)
	}
	// Discard binds to the text too.
	if stale := pressed(t, session, at(map[string]any{"action": "discard", "shown": "Lunch at one"})); stale.Status != "changed" {
		t.Errorf("a discard of text the draft no longer holds answered %+v", stale)
	}

	// A draft the person wrote in Mattermost was never the view's: a press
	// naming its text leaves it as it is. A proposal of the same text asks
	// them in the view.
	_, _, err = own.UpsertDraft(t.Context(), &model.Draft{UserId: user.Id, ChannelId: theirs.Id, Message: "my own words"})
	check(t, err)
	for _, action := range []string{"post", "discard"} {
		if left := pressed(t, session, map[string]any{"action": action, "channel_id": theirs.Id, "shown": "my own words"}); left.Status != "elsewhere" {
			t.Errorf("%s on the person's own draft answered %+v", action, left)
		}
	}
	structured(t, proposePost(t, session, map[string]any{"channel_id": theirs.Id, "message": "my own words"}), &choice)
	if choice.Saved || choice.Theirs == nil || choice.Theirs.Message != "my own words" {
		t.Errorf("a proposal of the person's own words answered %+v", choice)
	}
	if posts := messagesIn(t, admin, theirs.Id); len(posts) != 0 || draftsOf(t, user, team.Id)[theirs.Id+"/"] != "my own words" {
		t.Errorf("a press on the person's own draft posted %d messages, and left %q", len(posts), draftsOf(t, user, team.Id)[theirs.Id+"/"])
	}

	// A draft given a file in Mattermost is sent from there.
	proposePost(t, session, map[string]any{"channel_id": attached.Id, "message": "the plan"})
	draft := draftIn(t, user, team.Id, attached.Id, "")
	uploaded, _, err := own.UploadFile(t.Context(), []byte("x"), attached.Id, "plan.txt")
	check(t, err)
	draft.FileIds = []string{uploaded.FileInfos[0].Id}
	_, _, err = own.UpsertDraft(t.Context(), draft)
	check(t, err)
	if elsewhere := pressed(t, session, map[string]any{"action": "post", "channel_id": attached.Id, "shown": "the plan"}); elsewhere.Status != "elsewhere" {
		t.Errorf("a press on a draft with a file answered %+v", elsewhere)
	}
	if posts := messagesIn(t, admin, attached.Id); len(posts) != 0 {
		t.Errorf("a press on a draft with a file posted %d messages", len(posts))
	}
	// So is one made another kind of post there, as a plugin makes one.
	draft = draftIn(t, user, team.Id, attached.Id, "")
	draft.FileIds, draft.Message, draft.Type = nil, "the plan", "custom_plan"
	_, _, err = own.UpsertDraft(t.Context(), draft)
	check(t, err)
	if elsewhere := pressed(t, session, map[string]any{"action": "post", "channel_id": attached.Id, "shown": "the plan"}); elsewhere.Status != "elsewhere" {
		t.Errorf("a press on a draft of another kind answered %+v", elsewhere)
	}
}

func TestAPostThatAlertsAWholeChannelIsConfirmedAtThePress(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	others := []*model.User{seedUser(t, admin), seedUser(t, admin), seedUser(t, admin), seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)}
	team := seedTeam(t, admin, append([]*model.User{user}, others...)...)
	channel := seedChannel(t, admin, team, append([]*model.User{user}, others...)...)
	session := proposingSession(t, admin, user, true)

	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "@channel lunch is here"})
	// Mattermost alerts every member but the sender.
	confirm := pressed(t, session, map[string]any{"action": "post", "channel_id": channel.Id, "shown": "@channel lunch is here"})
	if confirm.Status != "confirm" || confirm.Notifies == nil || confirm.Notifies.People != members(t, channel.Id)-1 || strings.Join(confirm.Notifies.Mentions, " ") != "@channel" {
		t.Fatalf("a press on a message to the whole channel answered %+v, %+v", confirm, confirm.Notifies)
	}
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Fatalf("an unconfirmed press posted %d messages", len(posts))
	}
	posted := pressed(t, session, map[string]any{"action": "post", "channel_id": channel.Id, "shown": "@channel lunch is here", "confirmed": true})
	if posted.Status != "posted" || posted.Post.Message != "@channel lunch is here" {
		t.Fatalf("the confirmed press answered %+v", posted)
	}
}

func TestAPressIsCheckedAgainAndPostsOnlyWhereTheServerPostsFromViews(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	session := proposingSession(t, admin, user, true)

	// An edit in the view mentions someone nobody is: the press says so.
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "thanks @" + other.Username})
	slipped := "thanks @" + slipOf(other.Username)
	pressed(t, session, map[string]any{"action": "edit", "channel_id": channel.Id, "shown": "thanks @" + other.Username, "message": slipped})
	refusedSaying(t, session, "view_change", map[string]any{"action": "post", "channel_id": channel.Id, "shown": slipped}, "nobody is called", other.Username)

	// A server that does not post from a view refuses Post, and still edits.
	off := proposingSession(t, admin, user, false)
	refusedSaying(t, off, "view_change", map[string]any{"action": "post", "channel_id": channel.Id, "shown": slipped}, config.EnvPostFromViews, "from Mattermost")
	if edited := pressed(t, off, map[string]any{"action": "edit", "channel_id": channel.Id, "shown": slipped, "message": "thanks all"}); edited.Status != "saved" {
		t.Errorf("an edit where posting from views is off answered %+v", edited)
	}
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Errorf("the refused presses posted %d messages", len(posts))
	}
	// A client that renders no views is refused the tool.
	refusedSaying(t, unaskedSession(t, admin, user), "view_change", map[string]any{"action": "discard", "channel_id": channel.Id, "shown": "thanks all"}, "views alone")
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; got != "thanks all" {
		t.Errorf("the draft reads %q", fmt.Sprint(got))
	}
}

func TestWhatAProposalOrAPressCannotReachIsRefused(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	session := proposingSession(t, admin, user, true)
	nowhere := strings.Repeat("q", 26)

	refusedSaying(t, session, "propose_post", map[string]any{"message": "somewhere"}, "give channel_id")
	// A slash command would run when the person sent it.
	refusedSaying(t, session, "propose_post", map[string]any{"channel_id": channel.Id, "message": " /leave"}, "slash command", "create_post")
	if result := proposePost(t, session, map[string]any{"channel_id": nowhere, "message": "nowhere"}); !result.IsError {
		t.Error("a proposal in a channel that is not there was saved")
	}
	for _, c := range []struct {
		arguments map[string]any
		says      string
	}{
		{map[string]any{"action": "edit", "channel_id": channel.Id, "shown": "x", "message": "  "}, "would be empty"},
		{map[string]any{"action": "edit", "channel_id": channel.Id, "shown": "x", "message": strings.Repeat("x", 16384)}, "characters"},
		{map[string]any{"action": "replace", "channel_id": channel.Id, "shown": "x", "message": "/kick @someone"}, "slash command"},
	} {
		refusedSaying(t, session, "view_change", c.arguments, c.says)
	}
	for _, arguments := range []map[string]any{
		{"action": "post", "channel_id": nowhere, "shown": "x"},
	} {
		if result := callTool(t, session, &mcp.CallToolParams{Name: "view_change", Arguments: arguments}); !result.IsError {
			t.Errorf("a press on a channel that is not there answered %s", errorText(result))
		}
	}

	// Replace mine where the person's draft went meanwhile writes nothing.
	if replaced := pressed(t, session, map[string]any{"action": "replace", "channel_id": channel.Id, "shown": "gone meanwhile", "message": "the proposal"}); replaced.Status != "gone" {
		t.Errorf("Replace mine over no draft answered %+v", replaced)
	}
	if _, there := draftsOf(t, user, team.Id)[channel.Id+"/"]; there {
		t.Error("Replace mine over no draft wrote one")
	}
	// @here in a channel of two alerts too few for Mattermost to ask.
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "the proposal"})
	pressed(t, session, map[string]any{"action": "edit", "channel_id": channel.Id, "shown": "the proposal", "message": "@here the proposal"})
	if posted := pressed(t, session, map[string]any{"action": "post", "channel_id": channel.Id, "shown": "@here the proposal"}); posted.Status != "posted" {
		t.Errorf("@here in a channel of %d answered %+v", members(t, channel.Id), posted)
	}
}

func TestAPressInAChannelArchivedSincePostsNothing(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	session := proposingSession(t, admin, user, true)
	proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": "before it closed"})
	_, err := admin.DeleteChannel(t.Context(), channel.Id)
	check(t, err)

	result := callTool(t, session, &mcp.CallToolParams{Name: "view_change", Arguments: map[string]any{"action": "post", "channel_id": channel.Id, "shown": "before it closed"}})
	if !result.IsError || !strings.Contains(errorText(result), "deleted channel") {
		t.Errorf("a press in an archived channel answered %s", errorText(result))
	}
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Errorf("a press in an archived channel posted %d messages", len(posts))
	}
}

// Presses on one draft at once, as from views of the same proposal, post it
// once, and every press after the first finds it gone. Mattermost's own
// deduplication by a post's pending id does not hold for presses that reach it
// together: deduplicateCreatePost reads the id and records it in two steps, so
// each can find it unseen. Presses meet there now and then, so one proposal
// after another is pressed in every view at once.
func TestPressesOnOneDraftAtOncePostItOnce(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	sessions := make([]*mcp.ClientSession, 6)
	for i := range sessions {
		sessions[i] = proposingSession(t, admin, user, true)
	}
	for round := range 10 {
		message := fmt.Sprintf("once only, %d", round+1)
		// Each view shows the proposal: the first saves it, and the rest find
		// it there already.
		for _, session := range sessions {
			proposePost(t, session, map[string]any{"channel_id": channel.Id, "message": message})
		}
		var wait sync.WaitGroup
		answers := make([]*mcp.CallToolResult, len(sessions))
		for i, session := range sessions {
			wait.Go(func() {
				answers[i], _ = tryTool(t, session, &mcp.CallToolParams{Name: "view_change", Arguments: map[string]any{"action": "post", "channel_id": channel.Id, "shown": message}})
			})
		}
		wait.Wait()
		var posted []string
		for _, post := range messagesIn(t, admin, channel.Id) {
			if post.Message == message {
				posted = append(posted, post.Id)
			}
		}
		if len(posted) != 1 {
			t.Fatalf("presses at once on proposal %d of 10 posted it %d times", round+1, len(posted))
		}
		var said []string
		for _, answer := range answers {
			if answer == nil || answer.IsError {
				t.Fatalf("a press on proposal %d of 10 was refused: %v", round+1, answer)
			}
			var change server.DraftChange
			structured(t, answer, &change)
			switch {
			case change.Status == "posted" && change.Post.ID == posted[0]:
				said = append(said, "posted")
			case change.Status == "gone":
				said = append(said, "gone")
			default:
				t.Errorf("a press on proposal %d of 10 answered %s with post %s, not the one posted, %s", round+1, change.Status, change.Post.ID, posted[0])
			}
		}
		if slices.Sort(said); strings.Join(said, " ") != "gone gone gone gone gone posted" {
			t.Errorf("the presses on proposal %d of 10 answered %v; want one posted, and the draft gone for the rest", round+1, said)
		}
	}
}

// A proposal that comes while a press posts the draft before it is not
// deleted with that draft: it waits for the press. Whatever the order, a
// proposal the model is told was saved waits in Mattermost.
func TestAProposalDuringAPressIsNotDeletedWithTheDraftItPosts(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	view, proposing := proposingSession(t, admin, user, true), proposingSession(t, admin, user, true)
	for round := range 10 {
		shown, next := fmt.Sprintf("shown, %d", round+1), fmt.Sprintf("next, %d", round+1)
		proposePost(t, view, map[string]any{"channel_id": channel.Id, "message": shown})
		var wait sync.WaitGroup
		var press, proposal *mcp.CallToolResult
		wait.Go(func() {
			press, _ = tryTool(t, view, &mcp.CallToolParams{Name: "view_change", Arguments: map[string]any{"action": "post", "channel_id": channel.Id, "shown": shown}})
		})
		wait.Go(func() {
			proposal, _ = tryTool(t, proposing, &mcp.CallToolParams{Name: "propose_post", Arguments: map[string]any{"channel_id": channel.Id, "message": next}})
		})
		wait.Wait()
		if press == nil || press.IsError || proposal == nil || proposal.IsError {
			t.Fatalf("on proposal %d of 10, the press answered %v and the proposal %v", round+1, press, proposal)
		}
		var saved server.Proposal
		structured(t, proposal, &saved)
		if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; saved.Saved && got != next {
			t.Fatalf("proposal %d of 10 was answered as saved, but the draft holds %q", round+1, got)
		}
	}
}
