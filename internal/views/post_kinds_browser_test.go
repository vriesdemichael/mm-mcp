//go:build views

package views_test

import (
	"strings"
	"testing"
)

// A /me post, and one an access policy withholds files from, drawn as
// Mattermost draws them (mm-mcp#79).
func TestAMePostAndWithheldFilesAreDrawnAsMattermostDrawsThem(t *testing.T) {
	h := openHost(t)
	place := map[string]any{"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team", "reply_count": 0}
	me := post("m1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "*waves at **everyone***", map[string]any{"type": "me"})
	withheld := post("m2", "bob", "Bob Example", "u2", "2026-10-09T09:40:00+02:00", "The plan is attached", map[string]any{"withheld_files": 2})
	for key, value := range place {
		me[key], withheld[key] = value, value
	}
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "m1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": map[string]any{"post": me, "root_id": "m1", "thread": []any{me, withheld}}},
		"resources": pictures(),
	})
	var drawn struct {
		Me, Bold, Withheld string
		Emphasis, Cards    int
	}
	h.in(i, `const posts = d.querySelectorAll('.post');
	return {
		Me: posts[0].querySelector('.post-body > .markdown').textContent,
		Bold: (posts[0].querySelector('.post-body > .markdown strong') || {}).textContent || '',
		Emphasis: posts[0].querySelectorAll('.post-body > .markdown em').length,
		Withheld: [...posts[1].querySelectorAll('.withheld .file-name, .withheld .file-meta')].map((e) => e.textContent).join(' | '),
		Cards: posts[1].querySelectorAll('.file-card').length,
	};`, &drawn)
	if strings.TrimSpace(drawn.Me) != "waves at everyone" || drawn.Bold != "everyone" || drawn.Emphasis != 0 {
		t.Errorf("a /me post is drawn as %+v", drawn)
	}
	if drawn.Withheld != "Files withheld | An access policy on this server keeps them from you" || drawn.Cards != 1 {
		t.Errorf("withheld files are drawn as %+v", drawn)
	}
}

// A result can carry its pictures, for a host that passes on no read of a
// resource: the view draws those, and no other kind of address.
func TestPicturesTheResultCarriesAreDrawnWhereTheHostReadsNone(t *testing.T) {
	h := openHost(t)
	picture := "data:image/png;base64," + pixel
	message := post("c1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "Look :party_parrot:", map[string]any{
		"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team", "reply_count": 0,
		"files": []any{map[string]any{"id": "f1", "uri": base + "/files/f1", "preview_uri": base + "/files/f1/preview", "name": "chart.png", "size": 9, "mime_type": "image/png", "width": 4, "height": 3}},
	})
	hostile := post("c2", "bob", "Bob Example", "u2", "2026-10-09T09:31:00+02:00", "And :not_a_picture:", map[string]any{
		"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team", "reply_count": 0,
	})
	i := h.mount(map[string]any{
		"input": map[string]any{"kind": "thread", "post_id": "c1"},
		"result": map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": map[string]any{"post": message, "root_id": "c1", "thread": []any{message, hostile}},
			"_meta": map[string]any{"mm-mcp/pictures": map[string]any{
				base + "/users/u1/image":      picture,
				base + "/emoji/party_parrot":  picture,
				base + "/files/f1/preview":    picture,
				base + "/files/f1":            "data:text/html;base64,PHNjcmlwdD4=",
				base + "/users/u2/image":      "https://evil.example.com/x.png",
				base + "/emoji/not_a_picture": "data:text/html;base64,PHNjcmlwdD4=",
			}},
		},
		"without": []string{"serverResources"},
	})
	h.untilIn(i, `d.querySelector('[data-post="c1"] .avatar img') && d.querySelector('[data-post="c1"] .emoji-custom img') && d.querySelector('[data-post="c1"] .image-card img')`)
	var drawn struct {
		Hostile  int
		Initials string
	}
	h.in(i, `const p = d.querySelector('[data-post="c2"]'); return { Hostile: p.querySelectorAll('img').length, Initials: p.querySelector('.avatar').textContent };`, &drawn)
	if drawn.Hostile != 0 || drawn.Initials != "BE" {
		t.Errorf("what is no picture was drawn as %+v", drawn)
	}
	// What the whole image holds is no picture, so opening it shows its preview.
	h.click(i, `[data-post="c1"] .image-card`)
	h.untilIn(i, `d.querySelector('.lightbox-image img')`)
}
