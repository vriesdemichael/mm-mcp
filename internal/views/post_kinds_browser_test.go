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
