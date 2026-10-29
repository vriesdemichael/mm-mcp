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

// Posts their authors marked Important or Urgent (mm-mcp#77). Asking for
// acknowledgements needs a license the stacks do not have, so the unit tests
// cover those on a recorded post.

// postMarked posts a message its author marked with a priority.
func postMarked(t *testing.T, author *model.Client4, channelID, message, priority string) *model.Post {
	t.Helper()
	post := &model.Post{ChannelId: channelID, Message: message}
	post.Metadata = &model.PostMetadata{Priority: &model.PostPriority{Priority: model.NewPointer(priority)}}
	created, _, err := author.CreatePost(t.Context(), post)
	check(t, err)
	return created
}

func TestEveryReadToolSaysWhichPostsAreImportantOrUrgent(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	allowed, _, err := admin.GetClientConfig(t.Context(), "")
	check(t, err)
	if allowed["PostPriority"] != "true" {
		t.Fatal("this stack shows no message priorities, which Mattermost shows by default")
	}
	user := seedUser(t, admin)
	author := seedUser(t, admin)
	team := seedTeam(t, admin, user, author)
	channel := seedChannel(t, admin, team, user, author)
	writer := clientAs(t, author)
	marker := word(t)
	mention := "@" + user.Username + " " + marker
	urgent := postMarked(t, writer, channel.Id, "The payment API is down "+mention, model.PostPriorityUrgent)
	postMarked(t, writer, channel.Id, "Deploys freeze at five "+mention, "important")
	postAs(t, writer, channel.Id, urgent.Id, "Looking into it "+mention)
	session := sessionFor(t, admin, user)

	// labels are what the model reads of the marked posts and the reply.
	labels := func(posts []server.Post) []string {
		var out []string
		for _, post := range posts {
			if strings.Contains(post.Message, marker) {
				out = append(out, fmt.Sprintf("%s: %q", strings.Fields(post.Message)[0], post.Priority))
			}
			if post.RequestedAck || post.Acknowledgements != nil || post.PersistentNotifications {
				t.Errorf("Team Edition shows no acknowledgements, yet %s carries them", post.ID)
			}
		}
		return out
	}
	oldestFirst := fmt.Sprint([]string{`The: "urgent"`, `Deploys: "important"`, `Looking: ""`})
	newestFirst := fmt.Sprint([]string{`Looking: ""`, `Deploys: "important"`, `The: "urgent"`})

	if got := labels(readChannel(t, session, map[string]any{"channel_id": channel.Id}).Posts); fmt.Sprint(got) != oldestFirst {
		t.Errorf("read_channel: %v", got)
	}
	var thread server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": urgent.Id}}), &thread)
	if got := labels(append([]server.Post{*thread.Post}, thread.Thread...)); fmt.Sprint(got) != fmt.Sprint([]string{`The: "urgent"`, `The: "urgent"`, `Looking: ""`}) {
		t.Errorf("read_post: %v", got)
	}
	var unread server.UnreadPosts
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_unread", Arguments: map[string]any{"channel_id": channel.Id}}), &unread)
	if got := labels(unread.Posts); fmt.Sprint(got) != oldestFirst {
		t.Errorf("read_unread: %v", got)
	}
	eventually(t, 30*time.Second, func() (bool, string) {
		got := labels(searchPosts(t, session, map[string]any{"terms": marker}).Posts)
		return fmt.Sprint(got) == newestFirst, fmt.Sprint(got)
	})
	eventually(t, 30*time.Second, func() (bool, string) {
		got := labels(listMentions(t, session, map[string]any{"in": channel.Id}).Posts)
		return fmt.Sprint(got) == newestFirst, fmt.Sprint(got)
	})
}
