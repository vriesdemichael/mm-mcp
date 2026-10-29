//go:build live

package live

import (
	"fmt"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post the Calls plugin starts a call with, which carries the call as the
// plugin's card shows it (mm-mcp#79). Starting a call needs a client in it,
// so the post is made as the plugin makes it once the call ended, through the
// API, which takes a custom_ type from anyone, and read by someone who was
// not in it.
func TestACallPostCarriesWhoWasInTheCall(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	starter, joined, reader := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, starter, joined, reader)
	channel := seedChannel(t, admin, team, starter, joined, reader)
	ended := "Call ended"
	_, _, err := clientAs(t, starter).CreatePost(t.Context(), &model.Post{ChannelId: channel.Id, Message: ended, Type: "custom_calls", Props: model.StringInterface{
		"attachments":  []*model.MessageAttachment{{Fallback: ended, Title: ended, Text: ended}},
		"start_at":     1793877000000,
		"end_at":       1793878800000,
		"participants": []string{starter.Id, joined.Id},
		"title":        "Release go/no-go",
	}})
	check(t, err)

	var got []string
	for _, post := range readChannel(t, sessionFor(t, admin, reader), map[string]any{"channel_id": channel.Id}).Posts {
		if post.Type == "custom_calls" {
			got = append(got, fmt.Sprintf("%q by %s, %d cards, %+v", post.Message, post.Author, len(post.MessageAttachments), *post.Call))
		}
	}
	want := fmt.Sprintf("%q by %s, 0 cards, %+v", ended, starter.Username, struct {
		Title, StartedAt, EndedAt string
		Participants              []string
	}{"Release go/no-go", "2026-11-05T11:10:00Z", "2026-11-05T11:40:00Z", []string{starter.Username, joined.Username}})
	if len(got) != 1 || got[0] != want {
		t.Errorf("read_channel reads the call as %q; want %q", got, want)
	}
}
