package server

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post an integration made shows what Mattermost's web app shows of its
// author: the name the integration chose where the server lets it, the BOT
// tag, and the integration's icon (mm-mcp#71).

// recordedBotWebhookPost is a post a webhook a bot owns made under the name
// and icon it chose, as Mattermost 11.11.1 answers with it, overrides allowed.
func recordedBotWebhookPost(t *testing.T) *model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/bot-webhook-post.json")
	if err != nil {
		t.Fatal(err)
	}
	var post model.Post
	if err := json.Unmarshal(data, &post); err != nil {
		t.Fatal(err)
	}
	return &post
}

func TestAnIntegrationsPostShowsTheNameItChoseWhereTheServerLetsIt(t *testing.T) {
	post := recordedBotWebhookPost(t)
	bot := &model.User{Id: post.UserId, Username: "hookbot", FirstName: "Hook", LastName: "Bot", IsBot: true}
	around := surroundings{client: clientAt("https://chat.example.com"), zone: time.UTC, people: map[string]*model.User{post.UserId: bot}}

	around.overrides = overrides{names: true, icons: true}
	got := around.post(post)
	if got.Author != "hookbot" || got.AuthorName != "Hook Bot" || !got.AuthorIsBot || !got.FromIntegration || got.ShownAs != "Build server" || !got.IntegrationIcon {
		t.Errorf("a bot's webhook post is described as %+v", got)
	}
	// The icon's address is the integration's, which the post does not pass on.
	if data, _ := json.Marshal(got); strings.Contains(string(data), "icon.png") {
		t.Errorf("the post carries its icon's address: %s", data)
	}

	// A server that lets no integration choose shows the account it posted
	// under, still tagged BOT.
	around.overrides = overrides{}
	got = around.post(post)
	if got.ShownAs != "" || got.IntegrationIcon || !got.FromIntegration || !got.AuthorIsBot {
		t.Errorf("on a server without overrides, a bot's webhook post is described as %+v", got)
	}
}

func TestOnlyAnIntegrationsPostShowsANameOrIconOfItsOwn(t *testing.T) {
	person := &model.User{Id: "u1", Username: "alice", FirstName: "Alice"}
	around := surroundings{client: clientAt("https://chat.example.com"), zone: time.UTC,
		people: map[string]*model.User{"u1": person}, overrides: overrides{names: true, icons: true}}

	// The names an integration sets mean nothing on a post it did not make.
	post := &model.Post{Id: "p1", UserId: "u1", Message: "hello"}
	post.AddProp(model.PostPropsOverrideUsername, "The CEO")
	post.AddProp(model.PostPropsOverrideIconURL, "https://ceo.example.com/face.png")
	if got := around.post(post); got.ShownAs != "" || got.FromIntegration || got.IntegrationIcon || got.AuthorIsBot {
		t.Errorf("a person's post is described as %+v", got)
	}

	// An integration that keeps its account's picture says use_user_icon, and
	// a name that is not text is none.
	post.AddProp(model.PostPropsFromWebhook, "true")
	post.AddProp("use_user_icon", "true")
	post.AddProp(model.PostPropsOverrideUsername, 42)
	if got := around.post(post); got.ShownAs != "" || !got.FromIntegration || got.IntegrationIcon {
		t.Errorf("a webhook post that keeps its account's picture is described as %+v", got)
	}
}
