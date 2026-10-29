//go:build views

package views_test

import (
	"strings"
	"testing"
)

// Posts their authors marked Important or Urgent, and who acknowledged them,
// drawn as Mattermost draws them (mm-mcp#77).

// incident is a thread as read_post answers with it: an urgent post that asks
// for acknowledgements, which the person gave, an important reply that one
// hostile acknowledgement claims, and a post that asks for them and has none.
func incident() map[string]any {
	place := map[string]any{"channel": "Incidents", "channel_url": "https://chat.example.com/team/channels/incidents", "team": "Team"}
	with := func(fields map[string]any) map[string]any {
		for key, value := range place {
			fields[key] = value
		}
		return fields
	}
	root := post("i1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "The payment API is down", with(map[string]any{
		"priority": "urgent", "persistent_notifications": true, "requested_ack": true, "acknowledged": true,
		"acknowledgements": []any{
			map[string]any{"user": "bob", "acknowledged_at": "2026-10-09T09:31:00+02:00"},
			map[string]any{"user": "me", "acknowledged_at": "2026-10-09T09:32:00+02:00"},
		},
		"reactions": map[string]any{"eyes": []any{"bob"}},
	}))
	return map[string]any{"post": root, "root_id": "i1", "thread": []any{
		root,
		post("i2", "bob", "Bob Example", "u2", "2026-10-09T09:33:00+02:00", "Deploys are frozen", with(map[string]any{
			"root_id": "i1", "priority": "important", "requested_ack": true,
			"acknowledgements": []any{map[string]any{"user": "<img src=x onerror=\"window.parent.pwned=1\">", "acknowledged_at": "2026-10-09T09:34:00+02:00"}, "not one"},
		})),
		post("i3", "carol", "Carol Example", "u3", "2026-10-09T09:40:00+02:00", "Status page updated", with(map[string]any{
			"root_id": "i1", "requested_ack": true, "priority": "<b>urgent</b>",
		})),
	}}
}

func TestAPostsPriorityAndAcknowledgementsAreDrawnAsMattermostDrawsThem(t *testing.T) {
	h := openHost(t)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "i1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": incident()},
		"resources": pictures(),
	})
	var drawn struct {
		Labels, Acks, Titles, Given []string
		AfterTime, Upper            string
		Pwned                       any
		Images                      int
	}
	h.in(i, `return {
		Labels: [...d.querySelectorAll('.post')].map((p) => p.querySelector('.priority-tag') ? p.querySelector('.priority-tag').dataset.priority + ':' + p.querySelector('.priority-tag').textContent : '-'),
		AfterTime: (() => { const tag = d.querySelector('.priority-tag'); return tag && tag.previousElementSibling ? tag.previousElementSibling.className : ''; })(),
		Upper: d.querySelector('.priority-tag') ? getComputedStyle(d.querySelector('.priority-tag')).textTransform : '',
		Acks: [...d.querySelectorAll('.post')].map((p) => p.querySelector('.acknowledgements') ? p.querySelector('.acknowledgements').textContent : '-'),
		Titles: [...d.querySelectorAll('.acknowledgements')].map((a) => a.title),
		Given: [...d.querySelectorAll('.acknowledgements')].map((a) => a.dataset.acknowledged),
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Images: d.querySelectorAll('.responses img, .post-header img').length,
	};`, &drawn)
	// The label follows the time, in capitals; a priority Mattermost does not
	// know is not drawn.
	if strings.Join(drawn.Labels, " | ") != "urgent:Urgent | important:Important | -" || !strings.Contains(drawn.AfterTime, "time") || drawn.Upper != "uppercase" {
		t.Errorf("the posts are labelled %q, after %q, in %s", drawn.Labels, drawn.AfterTime, drawn.Upper)
	}
	// Acknowledgements are counted beside the reactions, with who in the
	// title, and marked where the person gave theirs.
	if strings.Join(drawn.Acks, " | ") != "2 | 1 | 0" || strings.Join(drawn.Given, " ") != "true false false" {
		t.Errorf("the acknowledgements are drawn as %q, given %q", drawn.Acks, drawn.Given)
	}
	if len(drawn.Titles) != 3 || drawn.Titles[0] != "Acknowledged by @bob, @me" || !strings.HasPrefix(drawn.Titles[1], "Acknowledged by @<img") ||
		drawn.Titles[2] != "Its author asks everyone to acknowledge it. No one has yet." {
		t.Errorf("the acknowledgements are titled %q", drawn.Titles)
	}
	if drawn.Pwned != nil || drawn.Images != 0 {
		t.Errorf("a hostile acknowledgement was parsed: %+v", drawn)
	}
}
