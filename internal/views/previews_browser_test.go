//go:build views

package views_test

import (
	"fmt"
	"strings"
	"testing"
)

// The post, page or picture Mattermost shows below a post's message, and an
// attachment's pictures and links (mm-mcp#76).

// previews is a thread as read_post answers with it: a question quoting an
// important post, a link to a page, a link to a picture, an alert with a
// chart, and a reply in which each of them is hostile.
func previews() map[string]any {
	place := map[string]any{"channel": "Platform", "channel_url": "https://chat.example.com/team/channels/platform", "team": "Team"}
	with := func(fields map[string]any) map[string]any {
		for key, value := range place {
			fields[key] = value
		}
		return fields
	}
	evil := "<img src=x onerror=\"window.parent.pwned=1\">"
	root := post("q1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "Is this still true? https://chat.example.com/team/pl/r1", with(map[string]any{
		"quoted_post": map[string]any{
			"id": "r1", "url": "https://chat.example.com/team/pl/r1", "channel": "Architecture", "author": "bob", "author_name": "Bob Example",
			"created_at": "2026-10-01T14:00:00+02:00", "message": "The **retry budget** is 3 attempts", "priority": "important",
			"files":        []any{map[string]any{"id": "f1", "uri": base + "/files/f1", "name": "budget.pdf"}},
			"link_preview": map[string]any{"url": "https://example.com/retries", "title": "Retries, explained", "site_name": "Example"},
		},
	}))
	return map[string]any{"post": root, "root_id": "q1", "thread": []any{
		root,
		post("q2", "bob", "Bob Example", "u2", "2026-10-09T09:31:00+02:00", "Release notes https://github.com/mattermost/mattermost/releases", with(map[string]any{
			"root_id": "q1", "link_preview": map[string]any{"url": "https://github.com/mattermost/mattermost/releases", "site_name": "GitHub",
				"title": "Releases · mattermost/mattermost", "description": "Mattermost is an open source platform"},
		})),
		post("q3", "carol", "Carol Example", "u3", "2026-10-09T09:32:00+02:00", "The logo", with(map[string]any{
			"root_id": "q1", "link_preview": map[string]any{"url": "https://example.com/images/logo.png", "image": true, "width": 465, "height": 91},
		})),
		post("q4", "hookbot", "Hook Bot", "u9", "2026-10-09T09:33:00+02:00", "", with(map[string]any{
			"root_id": "q1", "type": "slack_attachment", "author_is_bot": true,
			"message_attachments": []any{map[string]any{
				"color": "danger", "author_name": "Grafana", "author_link": "https://grafana.example.com/", "title": "[Alerting] API latency",
				"thumb_url": "https://grafana.example.com/thumb.png", "text": "p99 above 800 ms", "image_url": "https://grafana.example.com/render/panel-4.png",
			}},
		})),
		post("q5", "mallory", "Mallory", "u5", "2026-10-09T09:34:00+02:00", "look", with(map[string]any{
			"root_id": "q1",
			"quoted_post": map[string]any{
				"id": "r2", "url": "javascript:window.parent.pwned=2", "channel": evil, "author": evil, "message": evil, "created_at": "2026-10-01T14:00:00+02:00",
				"link_preview": map[string]any{"url": "https://example.com/x", "title": evil, "description": evil, "site_name": evil},
			},
			"link_preview": map[string]any{"url": "javascript:window.parent.pwned=3", "image": true},
		})),
	}}
}

func TestWhatALinkLeadsToIsDrawnAsMattermostDrawsIt(t *testing.T) {
	h := openHost(t)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "q1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": previews()},
		"resources": pictures(),
	})
	var drawn struct {
		Quote, QuoteLabel, QuoteChannel, QuoteFiles, QuotePage string
		Pages, Images                                          []string
		Thumb, Chart, Author                                   string
		Drawn                                                  int
	}
	h.in(i, `const q = d.querySelector('[data-post="q1"] .quote');
	return {
		Quote: q ? q.querySelector('.author').textContent + ': ' + q.querySelector('.markdown').textContent : '',
		QuoteLabel: q && q.querySelector('.priority-tag') ? q.querySelector('.priority-tag').textContent : '',
		QuoteChannel: q && q.querySelector('.quote-channel') ? q.querySelector('.quote-channel').textContent : '',
		QuoteFiles: q && q.querySelector('.quote-files') ? q.querySelector('.quote-files').textContent : '',
		QuotePage: q && q.querySelector('.link-preview .link-title') ? q.querySelector('.link-preview .link-title').textContent : '',
		Pages: [...d.querySelectorAll('[data-post="q2"] .link-preview > *')].map((e) => e.className + '=' + e.textContent),
		Images: [...d.querySelectorAll('[data-post="q3"] .md-image')].map((e) => e.textContent + ' ' + e.title),
		Author: (d.querySelector('[data-post="q4"] .attachment-author button.link') || {}).title || '',
		Thumb: (d.querySelector('[data-post="q4"] .attachment-thumb .md-image') || {}).textContent || '',
		Chart: (d.querySelector('[data-post="q4"] .attachment-image .md-image') || {}).textContent || '',
		Drawn: d.querySelectorAll('.quote img, .link-preview img, .attachment img').length,
	};`, &drawn)
	if drawn.Quote != "Bob Example: The retry budget is 3 attempts" || drawn.QuoteLabel != "Important" || drawn.QuoteChannel != "Posted in Architecture" ||
		drawn.QuoteFiles != "1 file" || drawn.QuotePage != "Retries, explained" {
		t.Errorf("the quote is drawn as %+v", drawn)
	}
	if strings.Join(drawn.Pages, " | ") != "muted link-site=GitHub | link-title=Releases · mattermost/mattermost | link-description=Mattermost is an open source platform" {
		t.Errorf("the page is drawn as %q", drawn.Pages)
	}
	// A picture is named after its file, and opens through the host; the view
	// draws none, having no network.
	if len(drawn.Images) != 1 || drawn.Images[0] != "logo.png Open the image: https://example.com/images/logo.png" || drawn.Drawn != 0 {
		t.Errorf("the picture is drawn as %q, with %d drawn", drawn.Images, drawn.Drawn)
	}
	if drawn.Author != "https://grafana.example.com/" || drawn.Thumb != "thumb.png" || drawn.Chart != "panel-4.png" {
		t.Errorf("the alert's pictures and links are drawn as %+v", drawn)
	}

	// A link inside the quote opens that link, and not the quoted post.
	h.click(i, "[data-post=\"q1\"] .quote .link-title button")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 1 || links[0] != "https://example.com/retries" {
		t.Errorf("the link in the quote opened %v", links)
	}
	if calls := h.calls(i); len(calls) != 0 {
		t.Errorf("the link in the quote also called %v", calls)
	}
}

func TestAQuoteOrAPreviewIsNeverParsedAsHTML(t *testing.T) {
	h := openHost(t)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "q1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": previews()},
		"resources": pictures(),
	})
	var found struct {
		Pwned                    any
		Handlers, Parsed, Opened int
		Author, Channel, Title   string
	}
	h.in(i, `const q = d.querySelector('[data-post="q5"] .quote');
	return {
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Handlers: [...d.querySelectorAll('*')].filter((e) => [...e.attributes].some((a) => a.name.startsWith('on'))).length,
		Parsed: d.querySelectorAll('[data-post="q5"] img').length,
		Opened: d.querySelectorAll('[data-post="q5"] .quote .time').length - d.querySelectorAll('[data-post="q5"] .quote button.time').length,
		Author: q.querySelector('.author').textContent,
		Channel: q.querySelector('.quote-channel').textContent,
		Title: q.querySelector('.link-title').textContent,
	};`, &found)
	if found.Pwned != nil || found.Handlers != 0 || found.Parsed != 0 || found.Opened != 1 ||
		!strings.HasPrefix(found.Author, "<img") || !strings.Contains(found.Channel, "<img") || !strings.HasPrefix(found.Title, "<img") {
		t.Errorf("a hostile quote or preview was parsed, or its script link made a button: %+v", found)
	}
}

// showPreviews mounts the thread above, with extra options for the host.
func showPreviews(extra map[string]any) map[string]any {
	options := map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "q1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": previews()},
		"resources": pictures(),
	}
	for key, value := range extra {
		options[key] = value
	}
	return options
}

// A press anywhere on a quote opens the quoted post: in the view's place where
// the host lets the view call tools, with the way back, in Mattermost where it
// does not, and with a word about it where it refuses.
func TestAQuoteOpensTheQuotedPost(t *testing.T) {
	h := openHost(t)
	quoted := post("r1", "bob", "Bob Example", "u2", "2026-10-01T14:00:00+02:00", "The retry budget is 3 attempts", map[string]any{
		"channel": "Architecture", "channel_url": "https://chat.example.com/team/channels/architecture", "team": "Team", "reply_count": 0,
	})
	i := h.mount(showPreviews(map[string]any{"tools": map[string]any{"view_action": []any{
		answered(map[string]any{"post": quoted, "root_id": "r1", "thread": []any{quoted}}),
	}}}))
	h.click(i, `[data-post="q1"] .quote .quote-text`)
	h.untilIn(i, `d.querySelector('[data-post="r1"]')`)
	calls := h.calls(i)
	arguments, _ := calls[len(calls)-1]["arguments"].(map[string]any)
	if len(calls) != 1 || arguments["action"] != "thread" || arguments["post_id"] != "r1" {
		t.Errorf("the quote called %v", calls)
	}
	var there struct {
		Original, Focused bool
		Links             []string
	}
	h.in(i, `return { Original: Boolean(d.querySelector('[data-post="q1"]')), Focused: d.activeElement === d.querySelector('.back'), Links: f.links };`, &there)
	if there.Original || !there.Focused || len(there.Links) != 0 {
		t.Errorf("the quoted post is drawn as %+v", there)
	}
	h.click(i, ".back")
	h.untilIn(i, `d.querySelector('[data-post="q1"]') && !d.querySelector('.back')`)

	// The time opens it too, for a keyboard. Where the host offers no tools, it
	// opens in Mattermost, as does the box.
	none := h.mount(showPreviews(map[string]any{"without": []string{"serverTools"}}))
	h.click(none, `[data-post="q1"] .quote .time`)
	h.click(none, `[data-post="q1"] .quote`)
	var links []string
	h.in(none, "return f.links;", &links)
	if strings.Join(links, " ") != "https://chat.example.com/team/pl/r1 https://chat.example.com/team/pl/r1" {
		t.Errorf("without tools, the quote opened %v", links)
	}
	if calls := h.calls(none); len(calls) != 0 {
		t.Errorf("without tools, the quote called %v", calls)
	}

	// Selecting a quote's text to copy it opens nothing.
	selecting := h.mount(showPreviews(map[string]any{"without": []string{"serverTools"}}))
	var clicked bool
	h.in(selecting, `w.getSelection().selectAllChildren(d.querySelector('[data-post="q1"] .quote-text')); d.querySelector('[data-post="q1"] .quote-text').click(); w.clickedAt = Date.now(); return true;`, &clicked)
	// What opens reaches the host a moment later, so nothing having opened is
	// read after that moment.
	h.untilIn(selecting, `Date.now() - d.defaultView.clickedAt > 500`)
	h.in(selecting, "return f.links;", &links)
	if len(links) != 0 {
		t.Errorf("selecting the quote's text opened %v", links)
	}

	// A host that refuses says so, and offers the post in Mattermost.
	refused := h.mount(showPreviews(map[string]any{"tools": map[string]any{"view_action": []any{map[string]any{"isError": true, "content": []any{}}}}}))
	h.click(refused, `[data-post="q1"] .quote`)
	h.untilIn(refused, `d.querySelector('.notice')`)
	if got := h.text(refused, ".notice"); !strings.Contains(got, "This app does not open the quoted post here.") {
		t.Errorf("a refusal says %q", got)
	}

	// A quote whose address is no web address opens nothing, and does not look
	// as if it would.
	var hostile struct {
		Opens string
		Links []string
	}
	h.in(none, `const q = d.querySelector('[data-post="q5"] .quote'); q.click(); return { Opens: q.dataset.opens, Links: f.links };`, &hostile)
	if hostile.Opens != "false" || len(hostile.Links) != 2 {
		t.Errorf("a quote with a hostile address is drawn as %+v", hostile)
	}
}

// A long quote is cut to a height, and faded where it is cut, with a button
// that shows the rest, in place and without opening the post; a short one is
// left as it is.
func TestALongQuoteIsCutToAHeight(t *testing.T) {
	h := openHost(t)
	long := previews()
	root, _ := long["post"].(map[string]any)
	quote, _ := root["quoted_post"].(map[string]any)
	quote["message"] = strings.Repeat("The retry budget is 3 attempts, and here is why.\n\n", 40)
	i := h.mount(map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "q1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": long},
		"resources": pictures(),
	})
	h.untilIn(i, `d.querySelector('[data-post="q1"] .quote-text').dataset.clamped === 'true'`)
	var drawn struct{ Height, Whole int }
	h.in(i, `const q = d.querySelector('[data-post="q1"] .quote-text'); return { Height: q.clientHeight, Whole: q.scrollHeight };`, &drawn)
	if drawn.Height >= drawn.Whole || drawn.Height > 300 {
		t.Errorf("a long quote is drawn %d high of %d", drawn.Height, drawn.Whole)
	}

	more := `[data-post="q1"] .quote-more`
	shown := func(frame int, selector string) bool {
		var visible bool
		h.in(frame, fmt.Sprintf(`return w.getComputedStyle(d.querySelector(%q)).display !== 'none';`, selector), &visible)
		return visible
	}
	if !shown(i, more) || h.text(i, more) != "Show more" {
		t.Errorf("a long quote offers %q, shown %v", h.text(i, more), shown(i, more))
	}
	h.click(i, more)
	h.untilIn(i, `d.querySelector('[data-post="q1"] .quote-text').clientHeight >= d.querySelector('[data-post="q1"] .quote-text').scrollHeight && d.querySelector('[data-post="q1"] .quote-text').clientHeight > 300`)
	h.untilIn(i, `d.querySelector('[data-post="q1"] .quote-text').dataset.clamped === 'false'`)
	if !shown(i, more) || h.text(i, more) != "Show less" {
		t.Errorf("a long quote shown whole offers %q, shown %v", h.text(i, more), shown(i, more))
	}
	// Drawn again, as the view is when anything changes, it stays whole.
	h.eval(fmt.Sprintf("mmHost.frames[%d].bridge.setHostContext({ theme: 'dark' })", i), nil)
	h.untilIn(i, `d.documentElement.dataset.theme === 'dark'`)
	h.untilIn(i, `d.querySelector('[data-post="q1"] .quote-text').dataset.whole === 'true' && d.querySelector('[data-post="q1"] .quote-more').textContent === 'Show less'`)
	h.click(i, more)
	h.untilIn(i, `d.querySelector('[data-post="q1"] .quote-text').dataset.clamped === 'true'`)
	// Neither press opened the post, in the view or in Mattermost.
	var opened struct{ Links, Calls int }
	h.in(i, `return { Links: f.links.length, Calls: f.calls.length };`, &opened)
	if opened.Links != 0 || opened.Calls != 0 {
		t.Errorf("showing the rest of a quote opened it: %+v", opened)
	}

	short := h.mount(showPreviews(nil))
	h.untilIn(short, `d.querySelector('[data-post="q1"] .quote-text').dataset.clamped === 'false'`)
	if shown(short, more) {
		t.Error("a short quote offers to show more")
	}
}
