package server

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post the Calls plugin starts a call with carries the call, as the plugin's
// card shows it (mm-mcp#79).

// callPosts are a call's post as Mattermost answers with it while the call
// goes on and once it ended. Shaped from createCallStartedPost and
// updateCallPostEnded in the plugin's server/plugin.go at 1.12.3, with its
// English messages, since a call needs a client in it to start.
func callPosts(t *testing.T) map[string]*model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/call-posts.json")
	if err != nil {
		t.Fatal(err)
	}
	var posts map[string]*model.Post
	if err := json.Unmarshal(data, &posts); err != nil {
		t.Fatal(err)
	}
	return posts
}

func TestACallPostCarriesItsCallWithoutTheAttachmentThatRepeatsIt(t *testing.T) {
	t.Parallel()
	posts := callPosts(t)
	alice := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	bob := &model.User{Id: "y6d8b4ebgbfjjenyatq36utqgy", Username: "bob"}

	// Mattermost draws the plugin's card the same whether it draws blocks.
	for _, shown := range []display{{}, {blocks: true}} {
		got := shownWith(posts["started"], shown, bob, alice)
		if want := (&Call{Title: "Release go/no-go", StartedAt: "2026-11-05T11:10:00Z"}); !reflect.DeepEqual(got.Call, want) {
			t.Errorf("a call going on is carried as %+v; want %+v", got.Call, want)
		}
		if got.Message != "alice started a call" || got.MessageAttachments != nil || got.Blocks != nil {
			t.Errorf("a call going on is described as %+v", got)
		}
	}

	// Once it ended: who was in it, by username, and an id where none was read.
	got := shownWith(posts["ended"], display{}, alice)
	want := &Call{StartedAt: "2026-11-05T11:10:00Z", EndedAt: "2026-11-05T11:40:00Z", Participants: []string{"alice", bob.Id}}
	if !reflect.DeepEqual(got.Call, want) || got.Message != "Call ended" || got.MessageAttachments != nil {
		t.Errorf("an ended call is described as %+v, its call %+v; want %+v", got, got.Call, want)
	}
	if ids := callParticipants(posts["ended"]); !reflect.DeepEqual(ids, []string{alice.Id, bob.Id}) {
		t.Errorf("the participants read are %q", ids)
	}

	// Another post with the same props is no call.
	other := posts["ended"].Clone()
	other.Type = model.PostTypeMessageAttachment
	if got := shownWith(other, display{}, alice); got.Call != nil || len(got.MessageAttachments) != 1 || callParticipants(other) != nil {
		t.Errorf("a post that is not a call is described as %+v", got)
	}
}
