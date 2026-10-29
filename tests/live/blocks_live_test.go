//go:build live

package live

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// What integrations post as blocks or cards, carried where Mattermost draws
// them: on the newest release, and not on the ESR, which does not have them
// (mm-mcp#75).

// blockPayloads are what three integrations send an incoming webhook, each
// in its format, each with a message that mentions the reader.
func blockPayloads(mention string) map[string]map[string]any {
	return map[string]map[string]any{
		"mm_blocks": {"text": "Deploy " + mention, "props": map[string]any{"mm_blocks": []any{
			map[string]any{"type": "container", "accent_color": "danger", "content": []any{
				map[string]any{"type": "text", "text": "**Deploy 4821 failed** on api"},
				map[string]any{"type": "collapsible", "collapsed": true,
					"header":  []any{map[string]any{"type": "text", "text": "Log tail"}},
					"content": []any{map[string]any{"type": "text", "text": "relation users already exists"}}},
			}},
		}}},
		"blocks": {"text": "Incident " + mention, "props": map[string]any{"blocks": []any{
			map[string]any{"type": "header", "text": map[string]any{"type": "plain_text", "text": "Incident 312"}},
			map[string]any{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "*Payments* are degraded"}},
		}}},
		"cards": {"text": "Merge " + mention, "props": map[string]any{"cards": []any{
			map[string]any{"type": "AdaptiveCard", "version": "1.4", "body": []any{
				map[string]any{"type": "TextBlock", "text": "Pull request 88 merged"},
				map[string]any{"type": "FactSet", "facts": []any{map[string]any{"title": "Not", "value": "drawn"}}},
			}},
		}}},
	}
}

// blocksSaid is what a model reads of each post with marker in its message:
// its first word, and the text of its blocks.
func blocksSaid(posts []server.Post, marker string) []string {
	var out []string
	for _, post := range posts {
		if !strings.Contains(post.Message, marker) {
			continue
		}
		var texts []string
		for _, block := range post.Blocks {
			if block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		out = append(out, strings.Fields(post.Message)[0]+": "+strings.Join(texts, " / "))
	}
	return out
}

func TestEveryReadToolCarriesTheBlocksAnIntegrationPostedWhereMattermostDrawsThem(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	config, _, err := admin.GetClientConfig(t.Context(), "")
	check(t, err)
	drawsBlocks := config["FeatureFlagMmBlocksEnabled"] == "true"
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	hook := seedWebhook(t, admin, channel, "")
	marker := word(t)
	payloads := blockPayloads("@" + user.Username + " " + marker)
	for _, format := range []string{"mm_blocks", "blocks", "cards"} {
		postToWebhook(t, hook, payloads[format])
	}
	session := sessionFor(t, admin, user)

	want := []string{
		"Deploy: **Deploy 4821 failed** on api / Log tail / relation users already exists",
		"Incident: # Incident 312 / *Payments* are degraded",
		"Merge: Pull request 88 merged",
	}
	if !drawsBlocks {
		// The ESR draws the message alone.
		want = []string{"Deploy: ", "Incident: ", "Merge: "}
	}
	newestFirst := []string{want[2], want[1], want[0]}

	if got := blocksSaid(readChannel(t, session, map[string]any{"channel_id": channel.Id}).Posts, marker); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("read_channel: %q\nwant %q", got, want)
	}
	deploy := ""
	for _, post := range readChannel(t, session, map[string]any{"channel_id": channel.Id}).Posts {
		if strings.HasPrefix(post.Message, "Deploy ") {
			deploy = post.ID
		}
	}
	var thread server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": deploy, "include_thread": false}}), &thread)
	if got := blocksSaid([]server.Post{*thread.Post}, marker); fmt.Sprint(got) != fmt.Sprint(want[:1]) {
		t.Errorf("read_post: %q", got)
	}
	if drawsBlocks && (len(thread.Post.Blocks) != 4 || thread.Post.Blocks[0].AccentColor != "danger" || !thread.Post.Blocks[2].Collapsed) {
		t.Errorf("read_post draws the deploy as %+v", thread.Post.Blocks)
	}
	var unread server.UnreadPosts
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_unread", Arguments: map[string]any{"channel_id": channel.Id}}), &unread)
	if got := blocksSaid(unread.Posts, marker); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("read_unread: %q", got)
	}
	eventually(t, 30*time.Second, func() (bool, string) {
		got := blocksSaid(searchPosts(t, session, map[string]any{"terms": marker}).Posts, marker)
		return fmt.Sprint(got) == fmt.Sprint(newestFirst), fmt.Sprintf("search_posts: %q", got)
	})
	eventually(t, 30*time.Second, func() (bool, string) {
		got := blocksSaid(listMentions(t, session, map[string]any{"in": channel.Id}).Posts, marker)
		return fmt.Sprint(got) == fmt.Sprint(newestFirst), fmt.Sprintf("list_mentions: %q", got)
	})
}
