package server

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A reminder's link is whole, as Mattermost's web app builds it, and a post an
// access policy withholds files from says how many (mm-mcp#79).

func TestAReminderLinksToItsPostWhole(t *testing.T) {
	t.Parallel()
	client := clientAt("https://chat.example.com")
	reminder := func(message string, props model.StringInterface) *model.Post {
		post := &model.Post{Type: model.PostTypeReminder, Message: message}
		post.SetProps(props)
		return post
	}
	props := model.StringInterface{"team_name": "team", "post_id": "k1a7m3cz5tgn8qxbdwpse9hjzo", "username": "alice"}
	for message, want := range map[string]string{
		// Written on a server without a SiteURL.
		"Hi there, here's your reminder about this message from @alice: /team/pl/k1a7m3cz5tgn8qxbdwpse9hjzo": "Hi there, here's your reminder about this message from @alice: https://chat.example.com/team/pl/k1a7m3cz5tgn8qxbdwpse9hjzo",
		// Written on one with a SiteURL, whatever it is.
		"Hi there, here's your reminder about this message from @alice: https://mm.example.com/team/pl/k1a7m3cz5tgn8qxbdwpse9hjzo": "Hi there, here's your reminder about this message from @alice: https://mm.example.com/team/pl/k1a7m3cz5tgn8qxbdwpse9hjzo",
	} {
		if got := reminderMessage(client, reminder(message, props)); got != want {
			t.Errorf("%q reads %q; want %q", message, got, want)
		}
	}
	relative := "about this message from @alice: /team/pl/k1a7m3cz5tgn8qxbdwpse9hjzo"
	for name, post := range map[string]*model.Post{
		"another kind of post":       {Message: relative, Props: props},
		"a reminder without a team":  reminder(relative, model.StringInterface{"post_id": "k1a7m3cz5tgn8qxbdwpse9hjzo"}),
		"a reminder with a bad id":   reminder(relative, model.StringInterface{"team_name": "team", "post_id": "../x"}),
		"a reminder of another post": reminder(relative, model.StringInterface{"team_name": "team", "post_id": "y6d8b4ebgbfjjenyatq36utqgy"}),
	} {
		if got := reminderMessage(client, post); got != relative {
			t.Errorf("%s reads %q", name, got)
		}
	}
}

func TestAPostSaysHowManyFilesAreWithheld(t *testing.T) {
	t.Parallel()
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	post := &model.Post{Id: "p1", UserId: reader.Id, Message: "the plan", Metadata: &model.PostMetadata{RedactedFileCount: 2}}
	if got := shownWith(post, display{}, reader); got.WithheldFiles != 2 || len(got.Files) != 0 {
		t.Errorf("a post with withheld files is described as %+v", got)
	}
}
