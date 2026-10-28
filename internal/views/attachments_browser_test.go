//go:build views

package views_test

import (
	"strings"
	"testing"
)

// Posts an integration made of message attachments, such as Alertmanager's
// alerts, drawn as Mattermost draws them (mm-mcp#71).

// alerts is a channel's alerts as read_post answers with them: one firing in
// a hex color, one resolved in Mattermost's own good, and one built to get
// out of its card.
func alerts() map[string]any {
	place := map[string]any{"channel": "Alerts", "channel_url": "https://chat.example.com/team/channels/alerts", "team": "Team"}
	alert := func(id, at string, card map[string]any, more map[string]any) map[string]any {
		fields := map[string]any{"type": "slack_attachment", "message_attachments": []any{card}}
		for key, value := range place {
			fields[key] = value
		}
		for key, value := range more {
			fields[key] = value
		}
		return post(id, "alertmanager", "Alert Manager", "u9", at, "", fields)
	}
	firing := alert("a1", "2026-10-09T09:30:00+02:00", map[string]any{
		"color": "#e40303", "pretext": "Production **alerts**", "title": "[FIRING:1] DiskFull db-1",
		"title_link": "https://alertmanager.example.com/#/alerts", "text": "/var on db-1 is *97%* full",
		"fields": []any{
			map[string]any{"title": "severity", "value": "critical", "short": true},
			map[string]any{"title": "status", "value": "firing", "short": true},
			map[string]any{"title": "runbook", "value": "[disk full](https://runbooks.example.com/disk)"},
		},
		"footer": "Alertmanager", "fallback": "[FIRING:1] DiskFull db-1",
	}, map[string]any{"reply_count": 0})
	resolved := alert("a2", "2026-10-09T09:31:00+02:00", map[string]any{
		"color": "good", "title": "[RESOLVED] DiskFull db-1", "fields": []any{map[string]any{"title": "status", "value": "resolved", "short": true}},
	}, map[string]any{"root_id": "a1", "message_attachments_length": 9000})
	hostile := alert("a3", "2026-10-09T09:32:00+02:00", map[string]any{
		"color":       "#e40303; background: url(https://evil.example.com/x)",
		"author_name": "<img src=x onerror=\"window.parent.pwned=1\">",
		"title":       "run me", "title_link": "javascript:window.parent.pwned=2",
		"text":   "<script>window.parent.pwned=3</script>",
		"fields": []any{map[string]any{"title": "<b>bold</b>", "value": 3}, "not a field"},
	}, map[string]any{"root_id": "a1"})
	return map[string]any{"post": firing, "root_id": "a1", "thread": []any{firing, resolved, hostile}}
}

func showAlerts() map[string]any {
	return map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "a1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": alerts()},
		"resources": pictures(),
	}
}

func TestMessageAttachmentsAreDrawnAsMattermostDrawsThem(t *testing.T) {
	h := openHost(t)
	i := h.mount(showAlerts())
	var drawn struct {
		Posts, System, Continued, Messages int
		Bars                               []string
		Pretext, Titles, Footer, Cut       string
		Rows                               []string
		TitleLinks                         int
	}
	h.in(i, `return {
		Posts: d.querySelectorAll('.post').length,
		System: d.querySelectorAll('.post.system').length,
		Continued: d.querySelectorAll('.post.continued').length,
		Messages: d.querySelectorAll('.post-body > .markdown').length,
		Bars: [...d.querySelectorAll('.attachment-card')].map((c) => c.dataset.bar),
		Pretext: (d.querySelector('.attachment > .markdown strong') || {}).textContent || '',
		Titles: [...d.querySelectorAll('.attachment-title')].map((h) => h.textContent).join(' | '),
		TitleLinks: d.querySelectorAll('.attachment-title button.link').length,
		Rows: [...d.querySelectorAll('.attachment-row')].map((r) => [...r.querySelectorAll('th')].map((c) => c.textContent).join('+') + '=' +
			[...r.querySelectorAll('td')].map((c) => c.textContent).join('+')),
		Footer: (d.querySelector('.attachment-footer') || {}).textContent || '',
		Cut: [...d.querySelectorAll('.cut')].map((c) => c.textContent).join(' | '),
	};`, &drawn)
	// An integration's post is its author's, not a message Mattermost wrote:
	// one after another within minutes run on, with no message of their own.
	if drawn.Posts != 3 || drawn.System != 0 || drawn.Continued != 2 || drawn.Messages != 0 {
		t.Errorf("the posts are drawn as %+v", drawn)
	}
	if strings.Join(drawn.Bars, " ") != "red green none" {
		t.Errorf("the bars are %v; want firing red, resolved green, and no color the view does not know", drawn.Bars)
	}
	if drawn.Pretext != "alerts" || drawn.Titles != "[FIRING:1] DiskFull db-1 | [RESOLVED] DiskFull db-1 | run me" || drawn.TitleLinks != 1 || drawn.Footer != "Alertmanager" {
		t.Errorf("the cards are drawn as %+v", drawn)
	}
	// Two short fields share a row, and a long one has its own.
	want := []string{"severity+status=critical+firing", "runbook=disk full", "status=resolved", "<b>bold</b>="}
	if strings.Join(drawn.Rows, " | ") != strings.Join(want, " | ") {
		t.Errorf("the fields are laid out as %q; want %q", drawn.Rows, want)
	}
	if !strings.Contains(drawn.Cut, "Its attachments are longer: 9,000 characters.") {
		t.Errorf("a post whose attachments are cut says %q", drawn.Cut)
	}

	h.click(i, ".attachment-title button.link")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 1 || links[0] != "https://alertmanager.example.com/#/alerts" {
		t.Errorf("the title opened %v", links)
	}
}

func TestAMessageAttachmentIsNeverParsedAsHTMLNorStyledFromItsData(t *testing.T) {
	h := openHost(t)
	i := h.mount(showAlerts())
	var found struct {
		Pwned    any
		Handlers int
		Styled   int
		Scripts  int
		Author   string
	}
	h.in(i, `return {
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Handlers: [...d.querySelectorAll('*')].filter((e) => [...e.attributes].some((a) => a.name.startsWith('on'))).length,
		Styled: [...d.querySelectorAll('.attachments *')].filter((e) => e.hasAttribute('style')).length,
		Scripts: d.querySelectorAll('.attachments script, .attachments img, .attachments b').length,
		Author: (d.querySelector('.attachment-author') || {}).textContent || '',
	};`, &found)
	if found.Pwned != nil || found.Handlers != 0 || found.Styled != 0 || found.Scripts != 0 || !strings.HasPrefix(found.Author, "<img src=x") {
		t.Errorf("a hostile attachment was parsed or styled: %+v", found)
	}
}
