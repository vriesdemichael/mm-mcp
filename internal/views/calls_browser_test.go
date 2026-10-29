//go:build views

package views_test

import (
	"strings"
	"testing"
)

// A call the Calls plugin started, drawn as the plugin's card draws it in
// place of the post's message (mm-mcp#79).

// calls are a channel's two calls as read_channel answers with them: one
// going on, with a title, and one that ended, whose title is hostile.
func calls() map[string]any {
	place := map[string]any{"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team", "type": "custom_calls", "reply_count": 0}
	with := func(fields map[string]any) map[string]any {
		for key, value := range place {
			fields[key] = value
		}
		return fields
	}
	going := post("k1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "alice started a call", with(map[string]any{
		"call": map[string]any{"title": "Release go/no-go", "started_at": "2026-10-09T09:30:00+02:00"},
	}))
	ended := post("k2", "bob", "Bob Example", "u2", "2026-10-09T10:00:00+02:00", "Call ended", with(map[string]any{
		"call": map[string]any{
			"title": "<img src=x onerror=\"window.parent.pwned=1\">", "started_at": "2026-10-09T10:00:00+02:00", "ended_at": "2026-10-09T11:05:00+02:00",
			"participants": []any{"bob", "alice", 3},
		},
	}))
	return map[string]any{"post": going, "root_id": "k1", "thread": []any{going, ended}}
}

func TestACallIsDrawnAsTheCallsPluginDrawsIt(t *testing.T) {
	h := openHost(t)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "k1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": calls()},
		"resources": pictures(),
	})
	var drawn struct {
		Calls    []string
		Messages int
		Pwned    any
	}
	h.in(i, `return {
		Calls: [...d.querySelectorAll('.call')].map((c) =>
			[...c.querySelectorAll('.call-title, strong, .call-detail, .call-people, .call-join')].map((e) => e.textContent).join(' | ')),
		Messages: d.querySelectorAll('.post-body > .markdown').length,
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
	};`, &drawn)
	want := []string{
		"Release go/no-go | Call in progress | Started at 9:30 AM by Alice Example | Join it in Mattermost",
		"<img src=x onerror=\"window.parent.pwned=1\"> | Call over | Finished at 11:05 AM · 1 hour, 5 minutes long | 2 people were in it: @bob, @alice",
	}
	if strings.Join(drawn.Calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("the calls are drawn as\n%s\nwant\n%s", strings.Join(drawn.Calls, "\n"), strings.Join(want, "\n"))
	}
	// The card stands in for the message, which says the same.
	if drawn.Messages != 0 || drawn.Pwned != nil {
		t.Errorf("the calls' messages are drawn %d times, or the title ran: %+v", drawn.Messages, drawn)
	}

	// Joining is the person's to do, in Mattermost.
	h.click(i, ".call-join")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 1 || links[0] != "https://chat.example.com/team/pl/k1" {
		t.Errorf("Join opened %v", links)
	}
}
