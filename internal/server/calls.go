package server

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// The Calls plugin, which comes with Mattermost and is on by default, posts a
// call in its channel when someone starts one, and changes the post when it
// ends (createCallStartedPost and updateCallPostEnded in its server/plugin.go,
// the same at 1.11.6 and 1.12.3). Its message says who started it, then that
// it ended, and a message attachment repeats that for a client without the
// plugin. The plugin draws its own card from the post's props instead: the
// call's title, when it started and by whom, or when it ended and how long it
// lasted. A post carries that card as its call, and leaves the attachment out.
// Joining a call is a write, and stays out.

// callPostType is the type of the post the Calls plugin starts a call with.
const callPostType = "custom_calls"

// Call is a call the Calls plugin started, as its post shows it.
type Call struct {
	Title        string   `json:"title,omitempty" jsonschema:"the title its starter gave it"`
	StartedAt    string   `json:"started_at,omitempty" jsonschema:"when it started; the post's author started it"`
	EndedAt      string   `json:"ended_at,omitempty" jsonschema:"when it ended; left out while it goes on, which Mattermost shows with a button to join it"`
	Participants []string `json:"participants,omitempty" jsonschema:"once it ended, the usernames of the people who were in it, or their ids where they could not be read"`
}

// call is the call the post started, or nil for any other post.
func (s surroundings) call(post *model.Post) *Call {
	if post.Type != callPostType {
		return nil
	}
	title, _ := post.GetProp("title").(string)
	out := &Call{Title: title, StartedAt: timestamp(propMillis(post, "start_at"), s.zone), EndedAt: timestamp(propMillis(post, "end_at"), s.zone)}
	for _, id := range callParticipants(post) {
		if user := s.people[id]; user != nil {
			id = user.Username
		}
		out.Participants = append(out.Participants, id)
	}
	return out
}

// callParticipants are the ids of the people who were in the call the post
// started, once it ended.
func callParticipants(post *model.Post) []string {
	if post.Type != callPostType {
		return nil
	}
	listed, _ := post.GetProp("participants").([]any)
	var ids []string
	for _, id := range listed {
		if id, ok := id.(string); ok && model.IsValidId(id) {
			ids = append(ids, id)
		}
	}
	return ids
}
