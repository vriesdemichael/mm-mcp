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

// A post that links to another carries the post Mattermost quotes below it,
// and only where the person can read that post (mm-mcp#76).

// quotes is what a model reads of the posts with marker in their message:
// the start of each, and the post it quotes.
func quotes(posts []server.Post, marker string) []string {
	var out []string
	for _, post := range posts {
		if !strings.Contains(post.Message, marker) {
			continue
		}
		quote := "nothing"
		if post.QuotedPost != nil {
			quote = fmt.Sprintf("%q by %s in %s, %s", post.QuotedPost.Message, post.QuotedPost.Author, post.QuotedPost.Channel, post.QuotedPost.Priority)
		}
		out = append(out, strings.Fields(post.Message)[0]+" quotes "+quote)
	}
	return out
}

func TestALinkToAPostCarriesItsQuoteOnlyForSomeoneWhoCanReadIt(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	reader := seedUser(t, admin)
	author := seedUser(t, admin)
	team := seedTeam(t, admin, reader, author)
	channel := seedChannel(t, admin, team, reader, author)
	private, _, err := admin.CreateChannel(t.Context(), &model.Channel{TeamId: team.Id, Name: uniqueName("private"), DisplayName: "Leadership", Type: model.ChannelTypePrivate})
	check(t, err)
	_, _, err = admin.AddChannelMember(t.Context(), private.Id, author.Id)
	check(t, err)

	writer := clientAs(t, author)
	marker := word(t)
	link := func(post *model.Post) string { return liveURL + "/" + team.Name + "/pl/" + post.Id }
	public := postMarked(t, writer, channel.Id, "The retry budget is 3 attempts per request.", "important")
	secret := postAs(t, writer, private.Id, "", "We close the Utrecht office in March.")
	postAs(t, writer, channel.Id, "", "Still? "+marker+" "+link(public))
	postAs(t, writer, channel.Id, "", "Seen? "+marker+" "+link(secret))

	want := map[string]string{
		reader.Username: fmt.Sprint([]string{
			fmt.Sprintf("Still? quotes %q by %s in %s, important", public.Message, author.Username, channel.DisplayName),
			"Seen? quotes nothing",
		}),
		author.Username: fmt.Sprint([]string{
			fmt.Sprintf("Still? quotes %q by %s in %s, important", public.Message, author.Username, channel.DisplayName),
			fmt.Sprintf("Seen? quotes %q by %s in %s, ", secret.Message, author.Username, private.DisplayName),
		}),
	}
	for _, person := range []*model.User{reader, author} {
		session := sessionFor(t, admin, person)
		listed := readChannel(t, session, map[string]any{"channel_id": channel.Id})
		if got := fmt.Sprint(quotes(listed.Posts, marker)); got != want[person.Username] {
			t.Errorf("read_channel, read by %s: %s\nwant %s", person.Username, got, want[person.Username])
		}
		for _, post := range listed.Posts {
			if strings.Contains(post.Message, marker) && post.LinkPreview != nil {
				t.Errorf("a link to a post is previewed as a page: %+v", post.LinkPreview)
			}
		}
		var last server.PostWithThread
		structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": listed.Posts[len(listed.Posts)-1].ID, "include_thread": false}}), &last)
		if got, wanted := fmt.Sprint(quotes([]server.Post{*last.Post}, marker)), want[person.Username]; !strings.Contains(wanted, strings.Trim(got, "[]")) {
			t.Errorf("read_post, read by %s: %s", person.Username, got)
		}
		eventually(t, 30*time.Second, func() (bool, string) {
			found := quotes(searchPosts(t, session, map[string]any{"terms": marker}).Posts, marker)
			// Search lists the newest first.
			for i, j := 0, len(found)-1; i < j; i, j = i+1, j-1 {
				found[i], found[j] = found[j], found[i]
			}
			return fmt.Sprint(found) == want[person.Username], fmt.Sprint(found)
		})
	}
}
