//go:build live

package live

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// What a question quotes of a post to say which it means, when the post's
// message says nothing (mm-mcp#79).

// A question about a post an integration made with no message quotes what
// Mattermost shows in its place, its blocks where it draws them and otherwise
// its attachments, and names it as Mattermost shows it; a post with only files
// is named by them.
func TestAQuestionQuotesWhatAPostShowsWhenItsMessageIsEmpty(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	config, _, err := admin.GetClientConfig(t.Context(), "")
	check(t, err)
	user, owner := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, owner)
	channel := seedChannel(t, admin, team, user, owner)
	hook := seedWebhook(t, admin, channel, owner.Id)
	postToWebhook(t, hook, map[string]any{"username": "Alertmanager", "attachments": []map[string]any{{
		"color": "danger", "title": "CPU high on db-1", "fields": []map[string]any{{"title": "Severity", "value": "critical"}},
		"fallback": "[FIRING:1] CPU high",
	}}})
	postToWebhook(t, hook, map[string]any{"username": "Statuspage",
		"attachments": []map[string]any{{"title": "Payments degraded, for clients without blocks"}},
		"props": map[string]any{"blocks": []any{
			map[string]any{"type": "header", "text": map[string]any{"type": "plain_text", "text": "Incident 312"}},
		}},
	})
	list, _, err := admin.GetPostsForChannel(t.Context(), channel.Id, 0, 200, "", false, false)
	check(t, err)
	var posts []*model.Post
	for _, id := range list.Order {
		if post := list.Posts[id]; !post.IsSystemMessage() {
			posts = append(posts, post)
		}
	}
	if len(posts) != 2 {
		t.Fatalf("the channel holds %d posts; want the two the integration made", len(posts))
	}
	statuspage, alert := posts[0], posts[1]
	by := func(name string) string {
		if config["EnablePostUsernameOverride"] != "true" {
			return "the post by @" + owner.Username
		}
		return "the post by “" + name + "” (an integration posting as @" + owner.Username + ")"
	}
	asking := func(name string, arguments map[string]any) string {
		t.Helper()
		session, questions := writingSession(t, admin, user, accept)
		callTool(t, session, &mcp.CallToolParams{Name: name, Arguments: arguments})
		return questions.only(t).Message
	}

	mustContain(t, "reaction's question", asking("add_reaction", map[string]any{"post_id": alert.Id, "emoji": "eyes"}),
		by("Alertmanager"), "“CPU high on db-1 · Severity · critical”")
	reply := map[string]any{"root_id": alert.Id, "message": "Looking into it", "reaches": members(t, channel.Id)}
	mustContain(t, "reply's question", asking("create_post", reply), "the thread started by "+by("Alertmanager")[len("the post by "):], "“CPU high on db-1")
	shown := "“Payments degraded, for clients without blocks”"
	if config["FeatureFlagMmBlocksEnabled"] == "true" {
		shown = "“# Incident 312”"
	}
	mustContain(t, "pin's question", asking("pin_post", map[string]any{"post_id": statuspage.Id}), by("Statuspage"), shown)

	own := clientAs(t, user)
	uploaded, _, err := own.UploadFile(t.Context(), []byte("the notes"), channel.Id, "notes.txt")
	check(t, err)
	files, _, err := own.CreatePost(t.Context(), &model.Post{ChannelId: channel.Id, FileIds: []string{uploaded.FileInfos[0].Id}})
	check(t, err)
	mustContain(t, "deletion's question", asking("delete_post", map[string]any{"post_id": files.Id, "deletes": 1}), "no text, 1 file: “notes.txt”")
}
