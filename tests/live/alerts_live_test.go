//go:build live

package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// Posts an integration made of message attachments, as Alertmanager posts to
// a channel through an incoming webhook (mm-mcp#71).

// seedWebhook creates an incoming webhook that posts to a channel, as an
// integration is set up, and deletes it when the test ends. It answers with
// the address the integration posts to.
func seedWebhook(t *testing.T, admin *model.Client4, channel *model.Channel) string {
	t.Helper()
	hook, _, err := admin.CreateIncomingWebhook(t.Context(), &model.IncomingWebhook{ChannelId: channel.Id, DisplayName: "Alertmanager"})
	check(t, err)
	t.Cleanup(func() { _, _ = admin.DeleteIncomingWebhook(context.Background(), hook.Id) })
	return liveURL + "/hooks/" + hook.Id
}

// postToWebhook posts what an integration sends an incoming webhook.
func postToWebhook(t *testing.T, address string, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	check(t, err)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, address, bytes.NewReader(body))
	check(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	check(t, err)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("posting to the webhook: %s", response.Status)
	}
}

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
	hook := seedWebhook(t, admin, channel)
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
