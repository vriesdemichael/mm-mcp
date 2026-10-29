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

	"github.com/mattermost/mattermost/server/public/model"
)

// Posts an integration makes under an account, a person's or a bot's, through
// an incoming webhook made for it (mm-mcp#71).

// seedWebhook creates an incoming webhook that posts to a channel under the
// owner's account, the administrator's when it is empty, as an integration is
// set up, and deletes it when the test ends. It answers with the address the
// integration posts to.
func seedWebhook(t *testing.T, admin *model.Client4, channel *model.Channel, owner string) string {
	t.Helper()
	hook, _, err := admin.CreateIncomingWebhook(t.Context(), &model.IncomingWebhook{ChannelId: channel.Id, UserId: owner, DisplayName: "Integration"})
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

// An integration's post shows what Mattermost shows of its author: the BOT
// tag, and the name and icon it chose where the server lets it, as
// docker/mattermost.env does. One that chooses no name shows as Mattermost's
// own "webhook", with Mattermost's icon for webhooks.
func TestAnIntegrationsPostCarriesWhatMattermostShowsOfItsAuthor(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	allowed, _, err := admin.GetClientConfig(t.Context(), "")
	check(t, err)
	if allowed["EnablePostUsernameOverride"] != "true" || allowed["EnablePostIconOverride"] != "true" {
		t.Fatal("this stack lets no integration choose its name and icon, as docker/mattermost.env does; `task stack:reset` starts it again with them")
	}
	user := seedUser(t, admin)
	owner := seedUser(t, admin)
	bot, _ := seedBot(t, admin)
	team := seedTeam(t, admin, user, owner)
	_, _, err = admin.AddTeamMember(t.Context(), team.Id, bot.UserId)
	check(t, err)
	channel := seedChannel(t, admin, team, user, owner)
	_, _, err = admin.AddChannelMember(t.Context(), channel.Id, bot.UserId)
	check(t, err)
	marker := word(t)
	postToWebhook(t, seedWebhook(t, admin, channel, bot.UserId), map[string]any{
		"text": marker, "username": "Build server", "icon_url": "https://ci.example.com/icon.png",
		"attachments": []map[string]any{{"color": "danger", "title": "Build 4821 failed", "fields": []map[string]any{{"title": "Branch", "value": "main", "short": true}}}},
	})
	postToWebhook(t, seedWebhook(t, admin, channel, owner.Id), map[string]any{"text": "Deployed 4822 " + marker})

	listed := readChannel(t, sessionFor(t, admin, user), map[string]any{"channel_id": channel.Id})
	var got []string
	for _, post := range listed.Posts {
		if strings.Contains(post.Message, marker) {
			got = append(got, fmt.Sprintf("%s as %q, bot %t, integration %t, its icon %t, %d cards",
				post.Author, post.ShownAs, post.AuthorIsBot, post.FromIntegration, post.IntegrationIcon, len(post.MessageAttachments)))
		}
	}
	want := []string{
		fmt.Sprintf("%s as %q, bot true, integration true, its icon true, 1 cards", bot.Username, "Build server"),
		fmt.Sprintf("%s as %q, bot false, integration true, its icon true, 0 cards", owner.Username, model.DefaultWebhookUsername),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("read_channel shows the integrations' posts as\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
