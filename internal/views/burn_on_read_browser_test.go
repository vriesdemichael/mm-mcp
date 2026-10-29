//go:build views

package views_test

import (
	"strings"
	"testing"
)

// Burn-on-read posts, drawn as Mattermost draws them: concealed until the
// person reveals one in Mattermost (mm-mcp#78).

// burning is a direct message's burn-on-read posts as read_channel answers
// with them: one concealed from the person, which a hostile answer gives
// cards, files and reactions all the same, and one they revealed.
func burning() map[string]any {
	place := map[string]any{"channel": "bob", "channel_url": "https://chat.example.com/team/messages/@bob", "type": "burn_on_read"}
	with := func(fields map[string]any) map[string]any {
		for key, value := range place {
			fields[key] = value
		}
		return fields
	}
	concealed := post("c1", "bob", "Bob Example", "u2", "2026-10-09T09:30:00+02:00", "", with(map[string]any{
		"concealed": true, "expire_at": "2026-10-16T09:30:00+02:00", "reply_count": 0,
		"message_attachments": []any{map[string]any{"title": "The vault code", "text": "4512 8890"}},
		"files":               []any{map[string]any{"id": "f1", "uri": base + "/files/f1", "name": "code.txt", "size": 9, "mime_type": "text/plain"}},
		"reactions":           map[string]any{"fire": []any{"bob"}},
	}))
	revealed := post("c2", "bob", "Bob Example", "u2", "2026-10-09T09:40:00+02:00", "The door code is *4711*", with(map[string]any{
		"expire_at": "2026-10-09T09:50:00+02:00", "reply_count": 0,
	}))
	return map[string]any{"post": concealed, "thread": []any{}, "root_id": "", "concealed": concealed, "revealed": revealed}
}

func TestAConcealedPostIsDrawnAsAPlaceholderThatOpensMattermost(t *testing.T) {
	h := openHost(t)
	data := burning()
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "c1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": map[string]any{"post": data["concealed"]}},
		"resources": pictures(),
	})
	var drawn struct {
		Placeholder, Author, Mark, Text string
		Held                            int
	}
	h.in(i, `return {
		Placeholder: (d.querySelector('.concealed') || {}).textContent || '',
		Author: (d.querySelector('.post .author') || {}).textContent || '',
		Mark: (d.querySelector('.mark.burn') || {}).title || '',
		Text: d.body.textContent,
		Held: d.querySelectorAll('.attachments, .files, .reactions, .post-body > .markdown').length,
	};`, &drawn)
	if !strings.Contains(drawn.Placeholder, "Viewing it in Mattermost reveals it") || !strings.Contains(drawn.Placeholder, "View message in Mattermost") ||
		drawn.Author != "Bob Example" || drawn.Mark == "" {
		t.Errorf("a concealed post is drawn as %+v", drawn)
	}
	// Nothing it holds is drawn, whatever the answer carried.
	if drawn.Held != 0 || strings.Contains(drawn.Text, "4512") || strings.Contains(drawn.Text, "code.txt") {
		t.Errorf("a concealed post shows what it holds: %d parts, %q", drawn.Held, drawn.Text)
	}
	// The way to it is Mattermost, where the person reveals it themselves.
	h.click(i, ".concealed-open")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 1 || links[0] != "https://chat.example.com/team/pl/c1" {
		t.Errorf("View message opened %v", links)
	}
	var calls []string
	h.in(i, "return (f.calls || []).map((c) => c.name);", &calls)
	if len(calls) != 0 {
		t.Errorf("the view called %v", calls)
	}
}

func TestARevealedBurnOnReadPostIsMarkedWithWhenItBurns(t *testing.T) {
	h := openHost(t)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "c2"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": map[string]any{"post": burning()["revealed"]}},
		"resources": pictures(),
	})
	var drawn struct{ Mark, Title, Message string }
	h.in(i, `return {
		Mark: (d.querySelector('.mark.burn') || {}).textContent || '',
		Title: (d.querySelector('.mark.burn') || {}).title || '',
		Message: (d.querySelector('.post-body > .markdown') || {}).textContent || '',
	};`, &drawn)
	if drawn.Mark != "Burn-on-read" || !strings.HasPrefix(drawn.Title, "Gone ") || !strings.Contains(drawn.Message, "4711") {
		t.Errorf("a revealed burn-on-read post is drawn as %+v", drawn)
	}
}
