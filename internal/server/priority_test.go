package server

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post its author marked Important or Urgent carries what Mattermost shows
// of it, where the server shows it (mm-mcp#77).

// recordedPost is a post as Mattermost answered with it, from testdata.
func recordedPost(t *testing.T, name string) *model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var post model.Post
	if err := json.Unmarshal(data, &post); err != nil {
		t.Fatal(err)
	}
	return &post
}

// shownWith is a post as a tool describes it, read by person on a server that
// shows what shown says.
func shownWith(post *model.Post, shown display, person *model.User, people ...*model.User) Post {
	known := map[string]*model.User{}
	for _, user := range append(people, person) {
		known[user.Id] = user
	}
	return surroundings{client: clientAt("https://chat.example.com"), zone: time.UTC, self: person, people: known, display: shown}.post(post)
}

func TestAnUrgentPostSaysSoWhereTheServerShowsPriorities(t *testing.T) {
	t.Parallel()
	// Recorded from Mattermost 11.11.1, Team Edition, which lets a post be
	// marked urgent but not ask for acknowledgements.
	post := recordedPost(t, "urgent-post.json")
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}

	got := shownWith(post, display{priority: true}, reader)
	if got.Priority != "urgent" || got.PersistentNotifications || got.RequestedAck || got.Acknowledgements != nil || got.Acknowledged {
		t.Errorf("an urgent post is described as %+v", got)
	}
	// A server with priorities off shows none, whatever the post holds.
	if got := shownWith(post, display{acknowledgements: true}, reader); got.Priority != "" {
		t.Errorf("with priorities off, an urgent post is described as %+v", got)
	}
	// Mattermost labels important and urgent alone.
	for label, shown := range map[string]string{"important": "important", "urgent": "urgent", "": "", "standard": "", "URGENT": ""} {
		marked := &model.Post{Id: "p1", Metadata: &model.PostMetadata{Priority: &model.PostPriority{Priority: model.NewPointer(label)}}}
		if got := shownWith(marked, display{priority: true}, reader).Priority; got != shown {
			t.Errorf("a post marked %q is labelled %q; want %q", label, got, shown)
		}
	}
}

func TestAPostThatAsksForAcknowledgementsSaysWhoGaveThem(t *testing.T) {
	t.Parallel()
	// The recorded urgent post as a licensed server answers with it once it
	// asks for acknowledgements: shaped from model.PostAcknowledgement, since
	// Team Edition refuses such a post. One acknowledgement was withdrawn.
	post := recordedPost(t, "acknowledged-post.json")
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	bob := &model.User{Id: "8sgr1qo5n7fkdf5dw4b6dqcxoc", Username: "bob"}

	got := shownWith(post, display{priority: true, acknowledgements: true}, reader, bob)
	want := []Acknowledgement{
		{User: "bob", AcknowledgedAt: "2026-10-29T11:27:44Z"},
		{User: "alice", AcknowledgedAt: "2026-10-29T11:28:19Z"},
	}
	if got.Priority != "urgent" || !got.PersistentNotifications || !got.RequestedAck || !got.Acknowledged || !reflect.DeepEqual(got.Acknowledgements, want) {
		t.Errorf("a post that asks for acknowledgements is described as %+v", got)
	}

	// Someone the answer could not name is given by id, and a person who has
	// not acknowledged it reads that they have not.
	carol := &model.User{Id: "c7w5f1dxq3bkzm8yh2ntr4gesa", Username: "carol"}
	got = shownWith(post, display{priority: true, acknowledgements: true}, carol, bob)
	if got.Acknowledged || len(got.Acknowledgements) != 2 || got.Acknowledgements[1].User != reader.Id {
		t.Errorf("read by carol, the post is described as %+v", got)
	}

	// A server that shows no acknowledgements, Team Edition's, still labels it.
	got = shownWith(post, display{priority: true}, reader, bob)
	if got.Priority != "urgent" || got.RequestedAck || got.Acknowledgements != nil || got.Acknowledged {
		t.Errorf("without acknowledgements shown, the post is described as %+v", got)
	}
	// Nor does one that shows no priorities at all.
	got = shownWith(post, display{acknowledgements: true}, reader, bob)
	if got.Priority != "" || got.PersistentNotifications || got.RequestedAck || got.Acknowledgements != nil {
		t.Errorf("without priorities shown, the post is described as %+v", got)
	}
}
