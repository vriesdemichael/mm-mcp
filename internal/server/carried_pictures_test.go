package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A thread shown in Claude Code carries its pictures, since that client passes
// on no read of a resource, and no other client gets them.

func TestOnlyClaudeCodeWithViewsCarriesPictures(t *testing.T) {
	t.Parallel()
	named := func(name string, capabilities *mcp.ClientCapabilities) *mcp.CallToolRequest {
		request := declaring(capabilities)
		request.Params.Meta[mcp.MetaKeyClientInfo] = &mcp.Implementation{Name: name, Version: "1"}
		return request
	}
	views := withViews(map[string]any{})
	for _, c := range []struct {
		name    string
		request *mcp.CallToolRequest
		carries bool
	}{
		{"Claude Code with views", named("claude-code", views), true},
		{"Claude Code without views", named("claude-code", &mcp.ClientCapabilities{}), false},
		{"VS Code with views", named("Visual Studio Code", views), false},
		{"a client not measured", named("codex", views), false},
		{"a client that names itself nothing", declaring(views), false},
	} {
		if got := carriesPictures(c.request); got != c.carries {
			t.Errorf("%s: carries pictures %t, want %t", c.name, got, c.carries)
		}
	}
}

func TestAThreadWantsItsPicturesSmallOnesFirstAndOnce(t *testing.T) {
	t.Parallel()
	alice, bob := "k1a7m3cz5tgn8qxbdwpse9hjzo", "y6d8b4ebgbfjjenyatq36utqgy"
	thread := PostWithThread{Thread: []Post{
		{AuthorID: alice, Message: "Nice :tada: and :party_parrot:", Reactions: map[string][]string{"shipit": {"bob"}, "+1": {"bob"}},
			Files: []Attachment{{ID: "f1", PreviewURI: "mattermost://chat.example.com/files/f1/preview"}, {ID: "f2", Name: "notes.txt"}}},
		{AuthorID: bob, Message: "Same :party_parrot: at 12:30:45"},
		{AuthorID: alice, Message: "again"},
		{AuthorID: "the-hook", IntegrationIcon: true, Message: "an integration shows its own icon"},
	}}
	var got []string
	for _, picture := range threadPictures(clientAt("https://chat.example.com"), thread) {
		got = append(got, strings.TrimPrefix(picture.uri, "mattermost://chat.example.com"))
	}
	for at, want := range map[int]string{0: "/users/" + alice + "/image", 1: "/users/" + bob + "/image", len(got) - 1: "/files/f1/preview"} {
		if at >= len(got) || got[at] != want {
			t.Errorf("wanted %q; at %d want %s", got, at, want)
		}
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{"/emoji/party_parrot", "/emoji/shipit"} {
		if strings.Count(joined, want) != 1 {
			t.Errorf("wanted %q; want %s once", got, want)
		}
	}
	// What comes with Mattermost is a character, and a time is no emoji.
	for _, not := range []string{"/emoji/tada", "/emoji/+1", "/emoji/30", "the-hook", "f2"} {
		if strings.Contains(joined, not) {
			t.Errorf("wanted %q, which holds %s", got, not)
		}
	}
}

func TestCarriedPicturesStayWithinTheirBounds(t *testing.T) {
	t.Parallel()
	png := func(size int) []byte {
		return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, size-8)...)
	}
	returning := func(data []byte, err error) func(context.Context) ([]byte, error) {
		return func(context.Context) ([]byte, error) { return data, err }
	}
	got := carriedPictures(t.Context(), []wantedPicture{
		{"small", returning(png(100), nil)},
		{"failed", returning(nil, errors.New("gone"))},
		{"not a picture", returning([]byte("<svg onload='alert(1)'></svg>"), nil)},
		{"over the bound on one", returning(png(pictureMost+1), nil)},
		{"at the bound on one", returning(png(pictureMost), nil)},
		{"at the bound again", returning(png(pictureMost), nil)},
		{"and again", returning(png(pictureMost), nil)},
		{"over the bound on all", returning(png(pictureMost), nil)},
		{"small after that", returning(png(100), nil)},
	})
	for _, kept := range []string{"small", "at the bound on one", "at the bound again", "and again", "small after that"} {
		if !strings.HasPrefix(got[kept], "data:image/png;base64,") {
			t.Errorf("%s was not carried: %.40q", kept, got[kept])
		}
	}
	for _, left := range []string{"failed", "not a picture", "over the bound on one", "over the bound on all"} {
		if _, ok := got[left]; ok {
			t.Errorf("%s was carried", left)
		}
	}
	// Nothing is asked of Mattermost once the call has ended.
	ended, cancel := context.WithCancel(t.Context())
	cancel()
	if got := carriedPictures(ended, []wantedPicture{{"small", returning(png(100), nil)}}); len(got) != 0 {
		t.Errorf("an ended call carried %d pictures", len(got))
	}
}
