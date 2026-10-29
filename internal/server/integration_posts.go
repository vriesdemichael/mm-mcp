package server

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// An integration posts under an account, a person's or a bot's: through an
// incoming webhook made for it, or as the answer to a slash command. Mattermost
// marks such a post from_webhook, and it may carry a name and an icon the
// integration chose, which Mattermost shows in place of the account's where
// the server allows it. A post carries what Mattermost's web app shows of its
// author: that name, its BOT tag, and whether the picture beside it is the
// account's.

// fromIntegration says whether an integration posted the post, as Mattermost's
// web app tells it.
func fromIntegration(post *model.Post) bool {
	return post.GetProp(model.PostPropsFromWebhook) == "true"
}

// shownAs is the name Mattermost shows for an integration's post in place of
// its author's, or "" where it shows the author's.
func (d display) shownAs(post *model.Post) string {
	if !d.names || !fromIntegration(post) {
		return ""
	}
	name, _ := post.GetProp(model.PostPropsOverrideUsername).(string)
	return name
}

// integrationIcon says whether Mattermost shows an integration's icon beside
// its post rather than its author's picture: the one it chose, or Mattermost's
// own for webhooks. An integration keeps the account's picture with
// use_user_icon, which Mattermost reads as any text at all.
func (d display) integrationIcon(post *model.Post) bool {
	userIcon, _ := post.GetProp("use_user_icon").(string)
	return d.icons && fromIntegration(post) && userIcon == ""
}
