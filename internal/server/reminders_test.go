package server

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A reminder links to its post with a full URL, as Mattermost's web app builds
// it (mm-mcp#79).

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
