//go:build live

package live

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// What a call takes away, stated in the call and checked against mm-mcp's own
// count just before it acts: the characters an edit or a draft's deletion
// discards, and the posts a deletion removes (ADR-031). The count binds the
// call to what the model read, so these run through a server that does not
// ask, where no question's fingerprint does.

// unaskedSession is a writing session through a server that leaves asking to
// the MCP client.
func unaskedSession(t *testing.T, admin *model.Client4, user *model.User) *mcp.ClientSession {
	t.Helper()
	return mcpWith(t, config.Config{
		URL: liveURL, Token: personalAccessToken(t, admin, user.Id).Token, AllowWrites: true,
	}, nil)
}

func TestAnEditStatesTheWholeTextItReplaces(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	own := clientAs(t, user)
	post := postAs(t, own, channel.Id, "", "Lunch at noon")
	session := unaskedSession(t, admin, user)
	edit := func(old any) map[string]any {
		arguments := map[string]any{"post_id": post.Id, "message": "Lunch at one", "reaches": members(t, channel.Id)}
		if old != nil {
			arguments["old_message"] = old
		}
		return arguments
	}

	refusedSaying(t, session, "update_post", edit(nil), "give old_message", "your post in ~"+channel.DisplayName, "“Lunch at noon”")
	refusedSaying(t, session, "update_post", edit("Lunch"), "differs from the current text", "“Lunch at noon”")

	// The person adds to the post in Mattermost after the model read it.
	_, _, err := own.PatchPost(t.Context(), post.Id, &model.PostPatch{Message: model.NewPointer("Lunch at noon, bring snacks")})
	check(t, err)
	refusedSaying(t, session, "update_post", edit("Lunch at noon"), "differs from the current text", "“Lunch at noon, bring snacks”", "Consider whether")
	stored, _, err := admin.GetPost(t.Context(), post.Id, "")
	check(t, err)
	if stored.Message != "Lunch at noon, bring snacks" {
		t.Fatalf("a refused edit changed the post: %q", stored.Message)
	}

	callTool(t, session, &mcp.CallToolParams{Name: "update_post", Arguments: edit("Lunch at noon, bring snacks")})
	stored, _, err = admin.GetPost(t.Context(), post.Id, "")
	check(t, err)
	if stored.Message != "Lunch at one" {
		t.Errorf("the post reads %q", stored.Message)
	}
}

// Reactions and replies answer a post as it read; an edit makes them answers
// to the new text, which Mattermost tells nobody about.
func TestAnEditOfAnAnsweredPostStatesHowManyAnswerIt(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, brent, ann := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, brent, ann)
	channel := seedChannel(t, admin, team, user, brent, ann)
	const asked = "Give me a thumbs up if you like cat memes"
	post := postAs(t, clientAs(t, user), channel.Id, "", asked)
	for _, who := range []*model.User{brent, ann} {
		_, _, err := clientAs(t, who).SaveReaction(t.Context(), &model.Reaction{UserId: who.Id, PostId: post.Id, EmojiName: "thumbsup"})
		check(t, err)
	}
	postAs(t, clientAs(t, ann), channel.Id, post.Id, "always")
	session := unaskedSession(t, admin, user)

	var read server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": post.Id, "include_thread": false}}), &read)
	if read.Post == nil || read.Post.Responses != 3 {
		t.Fatalf("read_post says %+v answer the post; want 3", read.Post)
	}
	edit := map[string]any{"post_id": post.Id, "message": "Let's drop that idea Brent had yesterday", "old_message": asked, "reaches": members(t, channel.Id)}
	refusedSaying(t, session, "update_post", edit,
		"3 reactions and replies respond to your post", ":thumbsup: ×2", "@"+brent.Username, "1 reply in its thread",
		"leaves them under the replacing text", "reply in the thread instead", "responses: 3")
	stored, _, err := admin.GetPost(t.Context(), post.Id, "")
	check(t, err)
	if stored.Message != asked {
		t.Fatalf("a refused edit changed the post: %q", stored.Message)
	}

	edit["responses"] = read.Post.Responses
	callTool(t, session, &mcp.CallToolParams{Name: "update_post", Arguments: edit})
	stored, _, err = admin.GetPost(t.Context(), post.Id, "")
	check(t, err)
	if stored.Message != "Let's drop that idea Brent had yesterday" {
		t.Errorf("the post reads %q", stored.Message)
	}
}

func TestADeletionStatesThePostsThatGo(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	root := postAs(t, clientAs(t, user), channel.Id, "", "Who is in for Friday?")
	replier := clientAs(t, other)
	postAs(t, replier, channel.Id, root.Id, "me")
	session := unaskedSession(t, admin, user)

	refusedSaying(t, session, "delete_post", map[string]any{"post_id": root.Id},
		"left the number out", "2 posts", "the 1 reply in its thread, other people's included", "Consider whether", "deletes: 2")
	refusedSaying(t, session, "delete_post", map[string]any{"post_id": root.Id, "deletes": 1}, "said 1", "deletes: 2")

	// A reply that arrived since the model read the thread.
	postAs(t, replier, channel.Id, root.Id, "me too")
	refusedSaying(t, session, "delete_post", map[string]any{"post_id": root.Id, "deletes": 2}, "said 2", "deletes: 3")
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 3 {
		t.Fatalf("a refused deletion took posts: %d left", len(posts))
	}

	callTool(t, session, &mcp.CallToolParams{Name: "delete_post", Arguments: map[string]any{"post_id": root.Id, "deletes": 3}})
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Errorf("%d posts are left", len(posts))
	}
}

func TestDeletingADraftStatesTheTextItDiscards(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	writer, _ := writingSession(t, admin, user, accept)
	saveDraft(t, writer, map[string]any{"channel_id": channel.Id, "message": "half a thought"})
	session := unaskedSession(t, admin, user)

	refusedSaying(t, session, "delete_draft", map[string]any{"channel_id": channel.Id},
		"left the number out", "14 characters", "the unsent draft in", "“half a thought", "Consider whether", "discards: 14")

	// The person types on in Mattermost after the model read the draft.
	_, _, err := clientAs(t, user).UpsertDraft(t.Context(), &model.Draft{UserId: user.Id, ChannelId: channel.Id, Message: "half a thought, finished"})
	check(t, err)
	refusedSaying(t, session, "delete_draft", map[string]any{"channel_id": channel.Id, "discards": 14}, "said 14", "discards: 24")
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; got != "half a thought, finished" {
		t.Fatalf("a refused deletion changed the draft: %q", got)
	}

	callTool(t, session, &mcp.CallToolParams{Name: "delete_draft", Arguments: map[string]any{"channel_id": channel.Id, "discards": 24}})
	if got := draftsOf(t, user, team.Id)[channel.Id+"/"]; got != "" {
		t.Errorf("the draft is still there: %q", got)
	}
}
