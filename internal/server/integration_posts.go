package server

import (
	"context"
	"slices"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// An integration posts under an account, a person's or a bot's: through an
// incoming webhook made for it, or as the answer to a slash command. Mattermost
// marks such a post from_webhook, and it may carry a name and an icon the
// integration chose, which Mattermost shows in place of the account's where
// the server allows it. A post carries what Mattermost's web app shows of its
// author: that name, its BOT tag, and whether the picture beside it is the
// account's.

// overrides is what the server lets an integration's post show in place of
// its author's, as it tells its clients.
type overrides struct {
	names, icons bool
}

// fromIntegration says whether an integration posted the post, as Mattermost's
// web app tells it.
func fromIntegration(post *model.Post) bool {
	return post.GetProp(model.PostPropsFromWebhook) == "true"
}

// readOverrides reads what the server lets the integrations among posts show,
// only when an integration posted one of them. GetClientConfig.
func readOverrides(ctx context.Context, client *mattermost.Client, posts []*model.Post) (overrides, error) {
	if !slices.ContainsFunc(posts, fromIntegration) {
		return overrides{}, nil
	}
	config, err := client.ClientConfig(ctx)
	if err != nil {
		return overrides{}, err
	}
	return overrides{
		names: config["EnablePostUsernameOverride"] == "true",
		icons: config["EnablePostIconOverride"] == "true",
	}, nil
}

// shownAs is the name Mattermost shows for an integration's post in place of
// its author's, or "" where it shows the author's.
func (o overrides) shownAs(post *model.Post) string {
	if !o.names || !fromIntegration(post) {
		return ""
	}
	name, _ := post.GetProp(model.PostPropsOverrideUsername).(string)
	return name
}

// integrationIcon says whether Mattermost shows an integration's icon beside
// its post rather than its author's picture: the one it chose, or Mattermost's
// own for webhooks. An integration keeps the account's picture with
// use_user_icon, which Mattermost reads as any text at all.
func (o overrides) integrationIcon(post *model.Post) bool {
	userIcon, _ := post.GetProp("use_user_icon").(string)
	return o.icons && fromIntegration(post) && userIcon == ""
}
