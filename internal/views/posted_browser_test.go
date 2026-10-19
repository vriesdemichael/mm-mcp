//go:build views

package views_test

import "testing"

// A post as posted: what create_post, dm and group_message answer with,
// drawn under where it went (ADR-022). Their call names no kind.

func showPosted(posted map[string]any) map[string]any {
	return map[string]any{
		"input":     map[string]any{"channel_id": base + "/team/channels/town-square", "message": posted["message"], "reaches": 12},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "posted"}}, "structuredContent": posted},
		"resources": pictures(),
	}
}

func TestAPostIsShownAsPosted(t *testing.T) {
	h := openHost(t)
	place := map[string]any{"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team"}
	i := h.mount(showPosted(post("p9", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "The notes are **ready**.", place)))
	var drawn struct {
		Label, Channel, Team, Message string
		Posts, Thread                 int
	}
	h.in(i, `const text = (selector) => { const e = d.querySelector(selector); return e ? e.textContent.trim() : ''; };
		return {
			Label: text('.posted-label'), Channel: text('.thread-title'), Team: text('.team'),
			Message: text('.post .markdown'), Posts: d.querySelectorAll('.post').length,
			Thread: d.querySelectorAll('.more-row').length,
		};`, &drawn)
	if drawn.Label != "Posted" || drawn.Channel != "Town Square" || drawn.Team != "Team" || drawn.Message != "The notes are ready." || drawn.Posts != 1 || drawn.Thread != 0 {
		t.Errorf("a post as posted is drawn as %+v", drawn)
	}
	if got := h.text(i, ".post strong"); got != "ready" {
		t.Errorf("its Markdown is drawn as %q", got)
	}

	reply := post("p10", "alice", "Alice Example", "u1", "2026-10-09T09:31:00+02:00", "Tomorrow.", place)
	reply["root_id"] = "p1"
	r := h.mount(showPosted(reply))
	if got := h.text(r, ".posted-label"); got != "Replied in a thread" {
		t.Errorf("a reply is labelled %q", got)
	}
	if got := h.text(r, ".more-row"); got != "Open the thread in Mattermost" {
		t.Errorf("a reply offers %q", got)
	}
}
