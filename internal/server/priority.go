package server

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// A person can mark a post Important or Urgent, which Mattermost shows beside
// their name, and on a licensed server ask everyone to acknowledge it, and
// keep notifying the people it mentions until they read it. A post carries
// what Mattermost shows of it, where the server shows it.

// postPriorityImportant is the web app's other label; the model package names
// only urgent.
const postPriorityImportant = "important"

// Acknowledgement is one person's acknowledgement of a post that asked for
// them.
type Acknowledgement struct {
	User           string `json:"user" jsonschema:"their username, or their id when it could not be read"`
	AcknowledgedAt string `json:"acknowledged_at"`
}

// priority sets what Mattermost shows of the post's priority on out: its
// label while the server shows priorities, and who acknowledged it while it
// shows acknowledgements, for a post that asks for them.
func (s surroundings) priority(post *model.Post, out *Post) {
	if !s.display.priority || post.Metadata == nil || post.Metadata.Priority == nil {
		return
	}
	priority := post.Metadata.Priority
	// Mattermost labels these two, and a post of standard priority not at all.
	if label := model.SafeDereference(priority.Priority); label == model.PostPriorityUrgent || label == postPriorityImportant {
		out.Priority = label
	}
	out.PersistentNotifications = model.SafeDereference(priority.PersistentNotifications)
	if !s.display.acknowledgements || !model.SafeDereference(priority.RequestedAck) {
		return
	}
	out.RequestedAck = true
	for _, given := range post.Metadata.Acknowledgements {
		// A withdrawn acknowledgement keeps its row, with no time.
		if given == nil || given.AcknowledgedAt == 0 {
			continue
		}
		who := given.UserId
		if user := s.people[who]; user != nil {
			who = user.Username
		}
		out.Acknowledgements = append(out.Acknowledgements, Acknowledgement{User: who, AcknowledgedAt: timestamp(given.AcknowledgedAt, s.zone)})
		if s.self != nil && given.UserId == s.self.Id {
			out.Acknowledged = true
		}
	}
}
