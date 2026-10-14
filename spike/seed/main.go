// Command seed (#37 spike, never merged) makes what the spike reads on a local
// live-test instance: a user of its own with a personal access token, a team,
// and a post with a reply, and writes them to .tmp/spike.env. The token is
// written to that file only, never printed.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/teststack"
)

func main() {
	address := os.Getenv("MM_URL")
	if address == "" {
		address = "http://localhost:8065"
	}
	out := ".tmp/spike.env"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	ctx := context.Background()
	admin := model.NewAPIv4Client(address)
	if _, _, err := admin.Login(ctx, teststack.AdminUsername, teststack.AdminPassword); err != nil {
		log.Fatalf("logging in as %s on %s: %v", teststack.AdminUsername, address, err)
	}
	suffix := random()
	password := "Spike-" + random() + "-1!"
	user, _, err := admin.CreateUser(ctx, &model.User{Username: "spike-" + suffix, Email: "spike-" + suffix + "@example.com", Password: password, FirstName: "Spike", LastName: "Person"})
	check("creating the user", err)
	team, _, err := admin.CreateTeam(ctx, &model.Team{Name: "spike-" + suffix, DisplayName: "Spike " + suffix, Type: model.TeamOpen})
	check("creating the team", err)
	_, _, err = admin.AddTeamMember(ctx, team.Id, user.Id)
	check("adding the user to the team", err)
	channel, _, err := admin.GetChannelByName(ctx, model.DefaultChannelName, team.Id, "")
	check("finding town-square", err)
	_, _, err = admin.AddChannelMember(ctx, channel.Id, user.Id)
	check("adding the user to town-square", err)
	token, _, err := admin.CreateUserAccessToken(ctx, user.Id, "spike #37", 0)
	check("creating the token", err)

	client := model.NewAPIv4Client(address)
	client.SetToken(token.Token)
	root, _, err := client.CreatePost(ctx, &model.Post{ChannelId: channel.Id,
		Message: "The release moved to **Thursday**. <img src=x onerror=alert(1)> stays text.\n\n- one\n- two"})
	check("posting", err)
	_, _, err = client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: root.Id, Message: "Thursday works for me :tada:"})
	check("replying", err)

	content := fmt.Sprintf("MM_URL=%s\nMM_TOKEN=%s\nPOST_ID=%s\n", address, token.Token, root.Id)
	check("making the directory", os.MkdirAll(filepath.Dir(out), 0o750))
	check("writing "+out, os.WriteFile(out, []byte(content), 0o600))
	fmt.Printf("seeded @%s in ~%s/town-square; post %s; written to %s\n", user.Username, team.Name, root.Id, out)
}

func random() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func check(what string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", what, err)
	}
}
