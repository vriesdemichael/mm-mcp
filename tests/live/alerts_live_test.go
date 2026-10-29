//go:build live

package live

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// Posts an integration made of message attachments, as Alertmanager posts to
// a channel through an incoming webhook (mm-mcp#71).

// alertPayload is what Alertmanager sends a Slack-compatible webhook about
// one alert: its state in the title, the bar's color and a field.
func alertPayload(text, state, color, description string) map[string]any {
	return map[string]any{
		"text": text,
		"attachments": []map[string]any{{
			"fallback":   fmt.Sprintf("[%s] DiskFull db-1", state),
			"color":      color,
			"title":      fmt.Sprintf("[%s] DiskFull db-1 (critical)", state),
			"title_link": "https://alertmanager.example.com/#/alerts?receiver=mattermost",
			"text":       description,
			"fields": []map[string]any{
				{"title": "severity", "value": "critical", "short": true},
				{"title": "status", "value": strings.ToLower(state), "short": true},
			},
			"footer": "Alertmanager",
		}},
	}
}

// alertState is what a post says of its alert, as a model reads it: the
// alert, its state and the color of its bar.
func alertState(post server.Post) string {
	if len(post.MessageAttachments) != 1 {
		return fmt.Sprintf("%d attachments", len(post.MessageAttachments))
	}
	attachment := post.MessageAttachments[0]
	status := ""
	for _, field := range attachment.Fields {
		if field.Title == "status" {
			status = field.Value
		}
	}
	return attachment.Title + " " + status + " " + attachment.Color
}

func TestEveryReadToolCarriesTheMessageAttachmentsOfAWebhookPost(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	hook := seedWebhook(t, admin, channel, "")
	marker := word(t)
	mention := "@" + user.Username + " " + marker
	postToWebhook(t, hook, alertPayload(mention, "FIRING", "#e40303", "*Description:* /var on db-1 is 97% full"))
	postToWebhook(t, hook, alertPayload(mention, "RESOLVED", "good", "*Description:* /var on db-1 is 61% full"))
	long := strings.Repeat("a long alert line. ", 300)
	postToWebhook(t, hook, alertPayload("", "FIRING", "danger", long))
	session := sessionFor(t, admin, user)

	firing := "[FIRING] DiskFull db-1 (critical) firing #e40303"
	resolved := "[RESOLVED] DiskFull db-1 (critical) resolved good"
	// alerts are what the model reads of the first two alerts, oldest first.
	alerts := func(posts []server.Post) []string {
		var out []string
		for _, post := range posts {
			if strings.Contains(post.Message, marker) {
				out = append(out, alertState(post))
			}
		}
		return out
	}
	want := fmt.Sprint([]string{firing, resolved})

	listed := readChannel(t, session, map[string]any{"channel_id": channel.Id})
	if got := alerts(listed.Posts); fmt.Sprint(got) != want {
		t.Errorf("read_channel: %v", got)
	}
	var cut server.Post
	for _, post := range listed.Posts {
		if post.Type == model.PostTypeMessageAttachment && post.Message == "" {
			cut = post
		}
	}
	if cut.MessageAttachmentsLength <= len(long) || len(cut.MessageAttachments) != 1 || !strings.HasSuffix(cut.MessageAttachments[0].Text, "…") {
		t.Errorf("read_channel shows a long alert as %d attachments of length %d", len(cut.MessageAttachments), cut.MessageAttachmentsLength)
	}

	var whole server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": cut.ID, "include_thread": false}}), &whole)
	if got := whole.Post.MessageAttachments; whole.Post.MessageAttachmentsLength != 0 || len(got) != 1 || got[0].Text != long || len(got[0].Fields) != 2 {
		t.Errorf("read_post reads a long alert as %+v", got)
	}

	var unread server.UnreadPosts
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_unread", Arguments: map[string]any{"channel_id": channel.Id}}), &unread)
	if got := alerts(unread.Posts); fmt.Sprint(got) != want {
		t.Errorf("read_unread: %v", got)
	}

	// Search lists the most recent first.
	newestFirst := fmt.Sprint([]string{resolved, firing})
	eventually(t, 30*time.Second, func() (bool, string) {
		found := searchPosts(t, session, map[string]any{"terms": marker})
		got := alerts(found.Posts)
		return fmt.Sprint(got) == newestFirst, fmt.Sprint(got)
	})
	eventually(t, 30*time.Second, func() (bool, string) {
		found := listMentions(t, session, map[string]any{"in": channel.Id})
		got := alerts(found.Posts)
		return fmt.Sprint(got) == newestFirst, fmt.Sprint(got)
	})
}

// An attachment's bar keeps the color the release draws it in: where 11.11
// draws attachments as blocks, any CSS color, as the integration wrote it; on
// 11.7, a hex color or one of the three Mattermost names. A container in an
// integration's blocks keeps any CSS color too (mm-mcp#79).
func TestAnAttachmentKeepsTheColorTheReleaseDrawsItsBarIn(t *testing.T) {
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
	colors := []string{"red", "rgb(220, 0, 0)", "#00ff00", "good", "ff0000"}
	for _, color := range colors {
		postToWebhook(t, hook, map[string]any{"text": marker, "attachments": []map[string]any{{"color": color, "title": "Disk full"}}})
	}
	postToWebhook(t, hook, map[string]any{"text": marker, "props": map[string]any{"mm_blocks": []any{
		map[string]any{"type": "container", "accent_color": "red", "content": []any{map[string]any{"type": "text", "text": "Disk full"}}},
	}}})

	var got []string
	for _, post := range readChannel(t, sessionFor(t, admin, user), map[string]any{"channel_id": channel.Id}).Posts {
		switch {
		case !strings.Contains(post.Message, marker):
		case len(post.MessageAttachments) == 1:
			got = append(got, post.MessageAttachments[0].Color)
		case len(post.Blocks) > 0:
			got = append(got, "block "+post.Blocks[0].AccentColor)
		default:
			got = append(got, "nothing")
		}
	}
	want := []string{"red", "rgb(220, 0, 0)", "#00ff00", "good", "", "block red"}
	if !drawsBlocks {
		want = []string{"", "", "#00ff00", "good", "", "nothing"}
	}
	if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
		t.Errorf("the colors of %q and a red container read as %q; want %q", colors, got, want)
	}
}
