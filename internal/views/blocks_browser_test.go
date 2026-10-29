//go:build views

package views_test

import (
	"strings"
	"testing"
)

// What integrations post as blocks or cards, drawn as Mattermost draws them
// (mm-mcp#75).

// deploys is a thread as read_post answers with it: a deploy an integration
// posted as blocks, with every kind of block the view draws, and a reply of
// hostile blocks.
func deploys() map[string]any {
	place := map[string]any{"channel": "Deploys", "channel_url": "https://chat.example.com/team/channels/deploys", "team": "Team"}
	with := func(fields map[string]any) map[string]any {
		for key, value := range place {
			fields[key] = value
		}
		return fields
	}
	block := func(kind string, depth any, more map[string]any) map[string]any {
		out := map[string]any{"type": kind, "depth": depth}
		for key, value := range more {
			out[key] = value
		}
		return out
	}
	evil := "<img src=x onerror=\"window.parent.pwned=1\">"
	root := post("d1", "deploybot", "Deploy Bot", "u9", "2026-10-09T09:30:00+02:00", "", with(map[string]any{
		"author_is_bot": true, "from_integration": true, "blocks_length": 20000,
		"blocks": []any{
			block("container", 0, map[string]any{"accent_color": "danger", "border": true}),
			block("text", 1, map[string]any{"text": "**Deploy 4821 failed** on `api`"}),
			block("column_set", 1, nil),
			block("column", 2, nil),
			block("text", 3, map[string]any{"text": "Stage", "subtle": true}),
			block("text", 3, map[string]any{"text": "migrate"}),
			block("column", 2, nil),
			block("text", 3, map[string]any{"text": "Duration", "subtle": true}),
			block("text", 3, map[string]any{"text": "4m 12s"}),
			block("divider", 1, nil),
			block("collapsible", 1, map[string]any{"text": "Log tail", "collapsed": true}),
			block("text", 2, map[string]any{"text": "relation users already exists"}),
			block("image", 1, map[string]any{"url": "https://ci.example.com/graphs/4821.png", "alt_text": "Build time graph"}),
			block("container", 0, map[string]any{"accent_color": "primary"}),
			block("text", 1, map[string]any{"text": "Rolled back"}),
		},
	}))
	hostile := post("d2", "mallory", "Mallory", "u5", "2026-10-09T09:31:00+02:00", "", with(map[string]any{
		"root_id": "d1",
		"blocks": []any{
			block("container", -3, map[string]any{"accent_color": "#e40303; background: url(https://evil.example.com/x)"}),
			block("text", 99, map[string]any{"text": evil}),
			block("image", "1", map[string]any{"url": "javascript:window.parent.pwned=2", "alt_text": evil}),
			block("collapsible", 0, map[string]any{"text": evil}),
			block("<script>", 1, map[string]any{"text": "not a block"}),
			"not a block",
		},
	}))
	return map[string]any{"post": root, "root_id": "d1", "thread": []any{root, hostile}}
}

func showDeploys() map[string]any {
	return map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "d1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": deploys()},
		"resources": pictures(),
	}
}

func TestBlocksAreDrawnAsMattermostDrawsThem(t *testing.T) {
	h := openHost(t)
	i := h.mount(showDeploys())
	var drawn struct {
		Bars, Borders, Columns []string
		Subtle, Folded, Image  string
		Open, Dividers         int
		Message, Cut, Rolled   string
	}
	h.in(i, `const p = d.querySelector('[data-post="d1"]');
	return {
		Bars: [...p.querySelectorAll('.block-container')].map((c) => c.dataset.bar),
		Borders: [...p.querySelectorAll('.block-container')].map((c) => c.dataset.border),
		Columns: [...p.querySelectorAll('.block-columns > .block-column')].map((c) => [...c.querySelectorAll('.block-text')].map((x) => x.textContent + (x.classList.contains('muted') ? '~' : '')).join('+')),
		Subtle: getComputedStyle(p.querySelector('.block-text.muted')).color === getComputedStyle(p.querySelector('.block-text:not(.muted)')).color ? 'same' : 'muted',
		Folded: [...p.querySelectorAll('details.block-collapsible')].map((c) => c.open + ':' + c.querySelector('summary').textContent + ':' + c.querySelector('.block-text').textContent).join(),
		Image: (p.querySelector('.block-image .md-image') || {}).textContent || '',
		Open: p.querySelectorAll('details[open]').length,
		Dividers: p.querySelectorAll('hr.block-divider').length,
		Message: p.querySelectorAll('.post-body > .markdown').length + '',
		Cut: [...p.querySelectorAll('.cut')].map((c) => c.textContent).join(),
		Rolled: p.querySelectorAll('.blocks > .block-container')[1].textContent,
	};`, &drawn)
	if strings.Join(drawn.Bars, " ") != "red blue" || strings.Join(drawn.Borders, " ") != "true false" {
		t.Errorf("the containers are drawn with bars %q, borders %q", drawn.Bars, drawn.Borders)
	}
	if strings.Join(drawn.Columns, " | ") != "Stage~+migrate | Duration~+4m 12s" || drawn.Subtle != "muted" {
		t.Errorf("the columns are drawn as %q, subtle %s", drawn.Columns, drawn.Subtle)
	}
	// Folded where Mattermost folds it, its header showing, and opened by the
	// person; a rule; and a picture named rather than drawn.
	if drawn.Folded != "false:Log tail:relation users already exists" || drawn.Open != 0 || drawn.Dividers != 1 || drawn.Image != "Build time graph" {
		t.Errorf("the deploy is drawn as %+v", drawn)
	}
	// A post of blocks alone draws no empty message, and says when it is cut.
	if drawn.Message != "0" || !strings.Contains(drawn.Cut, "Its blocks are longer: 20,000 characters.") || drawn.Rolled != "Rolled back" {
		t.Errorf("the post around the blocks is drawn as %+v", drawn)
	}
	h.click(i, `[data-post="d1"] summary`)
	var opened bool
	h.in(i, `return d.querySelector('[data-post="d1"] details').open;`, &opened)
	if !opened {
		t.Error("the person cannot open what is folded")
	}
}

func TestABlockIsNeverParsedAsHTMLNorStyledFromItsData(t *testing.T) {
	h := openHost(t)
	i := h.mount(showDeploys())
	var found struct {
		Pwned                      any
		Handlers, Styled, Parsed   int
		Bar, Text, Header, Image   string
		Drawn, Containers, Buttons int
	}
	h.in(i, `const p = d.querySelector('[data-post="d2"]');
	return {
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Handlers: [...d.querySelectorAll('*')].filter((e) => [...e.attributes].some((a) => a.name.startsWith('on'))).length,
		Styled: [...p.querySelectorAll('.blocks *')].filter((e) => e.hasAttribute('style')).length,
		Parsed: p.querySelectorAll('.blocks img, .blocks script').length,
		Bar: p.querySelector('.block-container').dataset.bar,
		Text: p.querySelector('.block-container .block-text').textContent,
		Header: p.querySelector('summary').textContent,
		Image: (p.querySelector('.block-image .md-image') || {}).tagName || '',
		Drawn: p.querySelectorAll('.blocks > *').length,
		Containers: p.querySelectorAll('.block-container .block-image').length,
		Buttons: p.querySelectorAll('.blocks button').length,
	};`, &found)
	// A depth out of place is read as the nearest that fits, one that is not a
	// whole number as none, so the image stands on its own; the script link
	// is named but opens nothing, and what is not a block is not drawn.
	if found.Pwned != nil || found.Handlers != 0 || found.Styled != 0 || found.Parsed != 0 || found.Bar != "none" ||
		!strings.HasPrefix(found.Text, "<img") || !strings.HasPrefix(found.Header, "<img") || found.Image != "SPAN" ||
		found.Drawn != 3 || found.Containers != 0 || found.Buttons != 0 {
		t.Errorf("hostile blocks are drawn as %+v", found)
	}
}
