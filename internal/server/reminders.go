package server

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// When a reminder someone set on a post is due, the system bot sends them a
// direct message with a link to the post, which it writes from the server's
// SiteURL (CheckPostReminders, app.post_reminder_dm). A server without one
// leaves the link a path, /team/pl/id, which a model cannot open. Mattermost's
// web app builds the link whole from the reminder's props, and so does mm-mcp.

// reminderMessage is a reminder's message with its link whole, and any other
// post's message as it is.
func reminderMessage(client *mattermost.Client, post *model.Post) string {
	if post.Type != model.PostTypeReminder {
		return post.Message
	}
	team, _ := post.GetProp("team_name").(string)
	id, _ := post.GetProp("post_id").(string)
	if !model.IsValidTeamName(team) || !model.IsValidId(id) {
		return post.Message
	}
	return strings.Replace(post.Message, " /"+team+"/pl/"+id, " "+client.Link(team, "pl", id), 1)
}
