//go:build views

package views_test

import (
	"strings"
	"testing"
)

// Posts an integration made of message attachments, such as Alertmanager's
// alerts, drawn as Mattermost draws them (mm-mcp#71).

// alerts is a channel's alerts as read_post answers with them, posted by a
// webhook a bot owns under a name of its own: one firing in a hex color, one
// resolved in Mattermost's own good, and one built to get out of its card.
func alerts() map[string]any {
	place := map[string]any{"channel": "Alerts", "channel_url": "https://chat.example.com/team/channels/alerts", "team": "Team"}
	alert := func(id, at string, card map[string]any, more map[string]any) map[string]any {
		fields := map[string]any{
			"type": "slack_attachment", "message_attachments": []any{card},
			"author_is_bot": true, "from_integration": true, "shown_as": "Monitoring", "integration_icon": true,
		}
		for key, value := range place {
			fields[key] = value
		}
		for key, value := range more {
			fields[key] = value
		}
		return post(id, "hookbot", "Hook Bot", "u9", at, "", fields)
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
	}, map[string]any{"root_id": "a1", "shown_as": "<img src=x onerror=\"window.parent.pwned=4\">"})
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
	// An integration's post is its author's, not a message Mattermost wrote,
	// and has no message of its own; Mattermost never runs one on from the
	// post before it.
	if drawn.Posts != 3 || drawn.System != 0 || drawn.Continued != 0 || drawn.Messages != 0 {
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
		Shown    string
	}
	h.in(i, `return {
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Handlers: [...d.querySelectorAll('*')].filter((e) => [...e.attributes].some((a) => a.name.startsWith('on'))).length,
		Styled: [...d.querySelectorAll('.attachments *')].filter((e) => e.hasAttribute('style')).length,
		Scripts: d.querySelectorAll('.attachments script, .attachments img, .attachments b, .post-header img').length,
		Author: (d.querySelector('.attachment-author') || {}).textContent || '',
		Shown: d.querySelectorAll('.post .author')[2].textContent,
	};`, &found)
	if found.Pwned != nil || found.Handlers != 0 || found.Styled != 0 || found.Scripts != 0 ||
		!strings.HasPrefix(found.Author, "<img src=x") || !strings.HasPrefix(found.Shown, "<img src=x") {
		t.Errorf("a hostile attachment was parsed or styled: %+v", found)
	}
}

// Where Mattermost draws blocks, 11.11 on, an integration's bar may be any
// color CSS takes, which the view draws in its own color nearest it, as it
// does a hex one; a color no one can see draws no bar (mm-mcp#79).
func TestABarInAnyCSSColorIsDrawnInTheViewsColorNearestIt(t *testing.T) {
	h := openHost(t)
	colors := []string{"red", "rgb(0, 160, 0)", "hsl(220 70% 50%)", "rebeccapurple", "silver", "rgba(220, 0, 0, 0)", "not a color"}
	var thread []any
	for i, color := range colors {
		id := "c" + string(rune('1'+i))
		thread = append(thread, post(id, "hookbot", "Hook Bot", "u9", "2026-10-09T09:3"+string(rune('0'+i))+":00+02:00", "", map[string]any{
			"type": "slack_attachment", "message_attachments": []any{map[string]any{"color": color, "title": color}},
			"channel": "Alerts", "channel_url": "https://chat.example.com/team/channels/alerts", "team": "Team",
		}))
	}
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "c1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": map[string]any{"post": thread[0], "root_id": "c1", "thread": thread}},
		"resources": pictures(),
	})
	var bars []string
	h.in(i, `return [...d.querySelectorAll('.attachment-card')].map((c) => c.dataset.bar);`, &bars)
	if want := "red green blue purple gray none none"; strings.Join(bars, " ") != want {
		t.Errorf("the bars of %q are %v; want %s", colors, bars, want)
	}
}

// integrations is a thread as read_post answers with it: two posts a webhook a
// bot owns made under a name and icon of its own; three posts of a bot's own,
// the last written with AI; and a person's webhook on a server that lets no
// integration choose a name.
func integrations() map[string]any {
	hook := map[string]any{"author_is_bot": true, "from_integration": true, "shown_as": "Build server", "integration_icon": true, "root_id": "b1"}
	bot := map[string]any{"author_is_bot": true, "root_id": "b1"}
	root := post("b1", "hookbot", "Hook Bot", "u9", "2026-10-09T09:30:00+02:00", "Build 4821 failed", map[string]any{
		"author_is_bot": true, "from_integration": true, "shown_as": "Build server", "integration_icon": true,
		"channel": "Builds", "channel_url": "https://chat.example.com/team/channels/builds", "team": "Team",
	})
	return map[string]any{"post": root, "root_id": "b1", "thread": []any{
		root,
		post("b2", "hookbot", "Hook Bot", "u9", "2026-10-09T09:31:00+02:00", "Build 4822 passed", hook),
		post("b3", "remindbot", "Reminder Bot", "u8", "2026-10-09T09:32:00+02:00", "Standup in 10 minutes", bot),
		post("b4", "remindbot", "Reminder Bot", "u8", "2026-10-09T09:33:00+02:00", "Standup now", bot),
		post("b5", "remindbot", "Reminder Bot", "u8", "2026-10-09T09:34:00+02:00", "Yesterday in short", map[string]any{"author_is_bot": true, "ai_generated": true, "root_id": "b1"}),
		post("b6", "alice", "Alice Example", "u1", "2026-10-09T09:35:00+02:00", "Deployed", map[string]any{"from_integration": true, "root_id": "b1"}),
	}}
}

func TestAnIntegrationsPostIsHeadedAsMattermostHeadsIt(t *testing.T) {
	h := openHost(t)
	image := map[string]any{"mimeType": "image/png", "blob": pixel}
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "b1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": integrations()},
		"resources": map[string]any{base + "/users/u1/image": image, base + "/users/u8/image": image, base + "/users/u9/image": image},
	})
	h.untilIn(i, "d.querySelectorAll('.avatar img').length >= 3")
	var drawn struct {
		Names, Accounts, Usernames, Tags []string
		Upper                            string
		Continued                        []string
		Pictures                         int
		Reads                            []string
	}
	h.in(i, `return {
		Names: [...d.querySelectorAll('.post .author')].map((a) => a.textContent),
		Accounts: [...d.querySelectorAll('.post .author')].map((a) => a.title),
		Usernames: [...d.querySelectorAll('.post .username')].map((u) => u.textContent),
		Tags: [...d.querySelectorAll('.post')].map((p) => p.querySelector('.bot-tag') ? p.querySelector('.bot-tag').textContent : '-'),
		Upper: d.querySelector('.bot-tag') ? getComputedStyle(d.querySelector('.bot-tag')).textTransform : '',
		Continued: [...d.querySelectorAll('.post.continued')].map((p) => p.dataset.post),
		Pictures: d.querySelectorAll('.avatar img').length,
		Reads: f.reads,
	};`, &drawn)
	// The name the integration chose stands in for its account's, which its
	// title still names; a bot's own posts and a person's webhook show theirs.
	if strings.Join(drawn.Names, " | ") != "Build server | Build server | Reminder Bot | Reminder Bot | Alice Example" ||
		strings.Join(drawn.Accounts, " | ") != "@hookbot | @hookbot |  |  | " ||
		strings.Join(drawn.Usernames, " ") != "@remindbot @remindbot @alice" {
		t.Errorf("the posts are headed %q, titled %q, with %q", drawn.Names, drawn.Accounts, drawn.Usernames)
	}
	// Every post of a bot or an integration is tagged BOT, a run of them once.
	if strings.Join(drawn.Tags, " ") != "Bot Bot Bot - Bot Bot" || drawn.Upper != "uppercase" {
		t.Errorf("the posts are tagged %q, in %s", drawn.Tags, drawn.Upper)
	}
	// A bot's posts run on as anyone's do; an integration's and one written
	// with AI never do.
	if strings.Join(drawn.Continued, " ") != "b4" {
		t.Errorf("%v run on from the post before them; want b4 alone", drawn.Continued)
	}
	// Where Mattermost shows the integration's icon, the view shows no picture
	// of the account it posted under.
	reads := strings.Join(drawn.Reads, " ")
	if drawn.Pictures != 3 || strings.Contains(reads, "/users/u9/") || !strings.Contains(reads, "/users/u8/") || !strings.Contains(reads, "/users/u1/") {
		t.Errorf("the view drew %d pictures and read %v", drawn.Pictures, drawn.Reads)
	}
}
