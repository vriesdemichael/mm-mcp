//go:build views

package views_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vriesdemichael/mm-mcp/internal/login"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The view page in a browser, inside the official MCP Apps app-bridge
// (ADR-022): `task test:views` bundles the bridge and installs Playwright's
// Chromium first. The data is what read_post answers, made up here; nothing
// here is Mattermost (ADR-005), and the page reaches nothing but the host.

const base = "mattermost://chat.example.com"

// pixel is a one-pixel PNG, as base64.
const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// valuer is the Chromium window's evaluation that answers with a value.
type valuer interface {
	Value(ctx context.Context, expression string) (json.RawMessage, error)
}

type host struct {
	t      *testing.T
	ctx    context.Context
	window valuer
}

func chromium(t *testing.T) login.Browser {
	t.Helper()
	found, err := login.FindBrowsers(login.ThisSystem(), "")
	if err != nil {
		t.Fatalf("no browser on this machine: %v", err)
	}
	var any *login.Browser
	for i, browser := range found {
		if browser.Name == "Playwright's Chromium" {
			return browser
		}
		if browser.Engine == login.Chromium && any == nil {
			any = &found[i]
		}
	}
	if any == nil {
		t.Fatalf("the views tests need a Chromium, Playwright's by preference (npx playwright install chromium); found %v", found)
	}
	return *any
}

func openHost(t *testing.T) *host {
	t.Helper()
	bundle, err := os.ReadFile(filepath.Join("..", "..", ".tmp", "views", "host.js"))
	if err != nil {
		t.Fatalf("the bridge's bundle is missing; task test:views makes it: %v", err)
	}
	page := "<!doctype html><html><head><meta charset=\"utf-8\"><title>host</title></head><body><script>" +
		strings.ReplaceAll(string(bundle), "</script", "<\\/script") + "</script></body></html>"
	path := filepath.Join(t.TempDir(), "host.html")
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	window, err := login.Start(ctx, chromium(t), "file:///"+filepath.ToSlash(path), login.Options{Headless: true, StartTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(window.Close)
	values, ok := window.(valuer)
	if !ok {
		t.Fatal("this browser's window reads no values back")
	}
	h := &host{t: t, ctx: ctx, window: values}
	h.until("typeof window.mmHost === 'object'")
	return h
}

func (h *host) eval(expression string, out any) {
	h.t.Helper()
	value, err := h.window.Value(h.ctx, expression)
	if err != nil {
		h.t.Fatalf("%v\nin: %.300s", err, expression)
	}
	if out != nil {
		if err := json.Unmarshal(value, out); err != nil {
			h.t.Fatalf("reading %s: %v", value, err)
		}
	}
}

// until waits for expression to hold in the host page.
func (h *host) until(expression string) {
	h.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var held bool
		h.eval("Boolean("+expression+")", &held)
		if held {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("waited for %s", expression)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// mount puts the page in a frame of the host, with options as host.js takes
// them, and waits until the view has drawn. The page is a writing server's,
// which lets a view mark a channel read and post a proposal, unless options
// say "readOnly", or "postsOff" for a server that does not post from a view.
func (h *host) mount(options map[string]any) int {
	h.t.Helper()
	readOnly, _ := options["readOnly"].(bool)
	postsOff, _ := options["postsOff"].(bool)
	page, _ := json.Marshal(views.Page(base, views.Settings{Changes: !readOnly, Posts: !postsOff}))
	settings, _ := json.Marshal(options)
	var index int
	h.eval("mmHost.mount("+string(page)+", "+string(settings)+")", &index)
	h.untilIn(index, "f.ready && d.getElementById('app') && !d.querySelector('[aria-busy]')")
	return index
}

// in runs body in a frame, with d its document, w its window and f what the
// host recorded of it, and reads back what body returns.
func (h *host) in(index int, body string, out any) {
	h.t.Helper()
	h.eval(fmt.Sprintf("(() => { const f = mmHost.frames[%d]; const d = f.iframe.contentDocument; const w = f.iframe.contentWindow; %s })()", index, body), out)
}

func (h *host) untilIn(index int, expression string) {
	h.t.Helper()
	h.until(fmt.Sprintf("(() => { const f = mmHost.frames[%d]; const d = f.iframe.contentDocument; return d && (%s); })()", index, expression))
}

func (h *host) text(index int, selector string) string {
	h.t.Helper()
	var text string
	h.in(index, fmt.Sprintf("const e = d.querySelector(%q); return e ? e.textContent : '';", selector), &text)
	return text
}

func (h *host) click(index int, selector string) {
	h.t.Helper()
	var clicked bool
	h.in(index, fmt.Sprintf("const e = d.querySelector(%q); if (e) e.click(); return Boolean(e);", selector), &clicked)
	if !clicked {
		h.t.Fatalf("nothing to click at %s", selector)
	}
}

// post is a post as read_post describes it.
func post(id, author, name, authorID, at, message string, more map[string]any) map[string]any {
	out := map[string]any{
		"id": id, "url": "https://chat.example.com/team/pl/" + id, "author": author, "author_name": name, "author_id": authorID,
		"created_at": at, "message": message, "reply_count": 40,
	}
	for key, value := range more {
		out[key] = value
	}
	return out
}

// thread is a thread's first page as read_post answers it, with hostile text
// in it: HTML, a script link, and a script in an image's handler.
func thread() map[string]any {
	root := post("p1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00",
		"The release moved to **Thursday** <img src=x onerror=\"window.parent.pwned=1\"> [run](javascript:window.parent.pwned=2)\n"+
			"cc @bob and @channel, see ~town-square :tada: :party_parrot:",
		map[string]any{
			"pinned": true, "reactions": map[string]any{"tada": []string{"bob"}, "party_parrot": []string{"bob", "carol"}},
			"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team",
		})
	return map[string]any{
		"post":    root,
		"root_id": "p1",
		"thread": []any{
			root,
			post("p2", "bob", "", "u2", "2026-10-09T09:31:00+02:00", "the chart", map[string]any{"root_id": "p1", "files": []any{
				map[string]any{"id": "f1", "uri": base + "/files/f1", "name": "chart.png", "size": 2048, "mime_type": "image/png", "width": 400, "height": 300, "preview_uri": base + "/files/f1/preview"},
				map[string]any{"id": "f2", "uri": base + "/files/f2", "name": "brief.pdf", "size": 120000, "mime_type": "application/pdf"},
			}}),
			post("p3", "bob", "", "u2", "2026-10-09T09:32:00+02:00", "and the numbers", map[string]any{"root_id": "p1", "edited_at": "2026-10-09T09:40:00+02:00"}),
			post("p4", "system", "", "u3", "2026-10-10T08:00:00+02:00", "carol joined the channel.", map[string]any{"root_id": "p1", "type": "system_join_channel"}),
		},
		"next_cursor": "c1",
	}
}

func pictures() map[string]any {
	image := map[string]any{"mimeType": "image/png", "blob": pixel}
	return map[string]any{
		base + "/users/u1/image":       image,
		base + "/emoji/party_parrot":   image,
		base + "/files/f1/preview":     image,
		base + "/files/f1":             image,
		base + "/users/u2/image":       map[string]any{"mimeType": "image/svg+xml", "blob": "PHN2Zy8+"},
		base + "/emoji/not_a_pictures": image,
	}
}

func showThread(extra map[string]any) map[string]any {
	options := map[string]any{
		"input":     map[string]any{"kind": "thread", "post_id": "p1"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": thread()},
		"resources": pictures(),
	}
	for key, value := range extra {
		options[key] = value
	}
	return options
}

func TestAThreadIsLaidOutAsMattermostLaysItOut(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	if got := h.text(i, ".thread-title"); got != "Town Square" {
		t.Errorf("the thread is titled %q", got)
	}
	if got := h.text(i, ".count"); got != "40 replies" {
		t.Errorf("the thread counts %q; Mattermost says 40", got)
	}
	var drawn struct {
		Posts, Days, Continued, Pinned, Edited, System, Mentions, Everyone int
		Channel                                                            string
		Emoji                                                              string
		Reactions                                                          []string
		Files                                                              int
		More                                                               string
	}
	h.untilIn(i, "d.querySelectorAll('.avatar img').length >= 1 && d.querySelectorAll('.emoji-custom img').length >= 2 && d.querySelector('.image-card img')")
	h.in(i, `return {
		Posts: d.querySelectorAll('.post').length,
		Days: d.querySelectorAll('.day').length,
		Continued: d.querySelectorAll('.post.continued').length,
		Pinned: d.querySelectorAll('.mark.pinned').length,
		Edited: [...d.querySelectorAll('.edited')].filter((m) => m.textContent === 'Edited').length,
		System: d.querySelectorAll('.post.system .system-message').length,
		Mentions: d.querySelectorAll('.md-mention').length,
		Everyone: d.querySelectorAll('.md-mention-all').length,
		Channel: (d.querySelector('.md-channel') || {}).title || '',
		Emoji: [...d.querySelectorAll('.markdown .emoji')].map((e) => e.textContent).join(' '),
		Reactions: [...d.querySelectorAll('.reaction')].map((r) => r.textContent + ' ' + r.title),
		Files: d.querySelectorAll('.file-card').length,
		More: (d.querySelector('.more') || {}).textContent || '',
	};`, &drawn)
	if drawn.Posts != 4 || drawn.Days != 2 || drawn.Continued != 1 || drawn.Pinned != 1 || drawn.Edited != 1 || drawn.System != 1 {
		t.Errorf("the posts are drawn as %+v", drawn)
	}
	if drawn.Mentions != 2 || drawn.Everyone != 1 || drawn.Channel != "https://chat.example.com/team/channels/town-square" {
		t.Errorf("mentions and channel references are drawn as %+v", drawn)
	}
	if !strings.Contains(drawn.Emoji, "🎉") {
		t.Errorf("the emoji are drawn as %q", drawn.Emoji)
	}
	reactions := strings.Join(drawn.Reactions, " | ")
	if len(drawn.Reactions) != 2 || !strings.Contains(reactions, "🎉1 @bob reacted with :tada:") || !strings.Contains(reactions, "2 @bob, @carol reacted with :party_parrot:") {
		t.Errorf("the reactions are drawn as %q", drawn.Reactions)
	}
	if drawn.Files != 1 || drawn.More != "37 more replies" {
		t.Errorf("the files and the rest of the thread are drawn as %+v", drawn)
	}
	// A picture that is not a raster image is not drawn: Bob stays initials.
	var initials string
	h.in(i, "return [...d.querySelectorAll('.avatar')].map((a) => a.querySelector('img') ? 'img' : a.textContent).join(' ');", &initials)
	if initials != "img B S" {
		t.Errorf("the avatars are %q", initials)
	}
	// The view reported its size, and the host made room for it.
	var height float64
	h.eval(fmt.Sprintf("mmHost.frames[%d].iframe.getBoundingClientRect().height", i), &height)
	if height <= 200 {
		t.Errorf("the view is %.0f pixels tall", height)
	}
}

func TestTextFromMattermostIsNeverParsedAsHTML(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	var found struct {
		Pwned      any
		Handlers   int
		ImagesText bool
		ScriptLink int
		Anchors    int
	}
	h.in(i, `return {
		Pwned: w.parent.pwned === undefined ? null : w.parent.pwned,
		Handlers: [...d.querySelectorAll('*')].filter((e) => [...e.attributes].some((a) => a.name.startsWith('on'))).length,
		ImagesText: d.querySelector('.markdown').textContent.includes('<img src=x onerror='),
		ScriptLink: [...d.querySelectorAll('button')].filter((b) => (b.title || '').startsWith('javascript')).length,
		Anchors: d.querySelectorAll('a[href], iframe, script:not([src=""])').length - d.querySelectorAll('script').length,
	};`, &found)
	if found.Pwned != nil || found.Handlers != 0 || !found.ImagesText || found.ScriptLink != 0 || found.Anchors != 0 {
		t.Errorf("hostile text was parsed: %+v", found)
	}
}

// A post shaped to make a parser backtrack, or look again and again for a
// close, as long as Mattermost lets a post be, is drawn about as fast as
// plain text as long: a thread of them must not freeze the view.
func TestAMessageBuiltToBeSlowIsDrawnAsFastAsPlainText(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	const longest = 16383
	separator := string(rune(0x2028))
	// Each ends in a word on a line of its own, which shows it was drawn.
	fill := func(head, unit, tail string) string {
		tail += "\nend"
		size := longest - len(head) - len(tail)
		return head + strings.Repeat(unit, size/len(unit)+1)[:size] + tail
	}
	// draw hands the view a thread of message as its host does, and reads
	// how long the view took to draw it, the fastest of three, and whether it
	// drew it to its end.
	draw := func(message string) (float64, bool) {
		root := post("p1", "alice", "", "u1", "2026-10-09T09:30:00+02:00", message, nil)
		result, _ := json.Marshal(map[string]any{"content": []any{}, "structuredContent": map[string]any{"post": root, "root_id": "p1", "thread": []any{root}}})
		var drawn struct {
			Took float64
			End  bool
		}
		// The message is dispatched as it arrives, and drawn before
		// dispatchEvent returns.
		h.in(i, `const message = { jsonrpc: "2.0", method: "ui/notifications/tool-result", params: `+string(result)+` };
			let took = Infinity;
			for (let n = 0; n < 3; n++) {
				const t0 = w.performance.now();
				w.dispatchEvent(new w.MessageEvent("message", { data: message, source: w.parent }));
				took = Math.min(took, w.performance.now() - t0);
			}
			const markdown = d.querySelector(".markdown");
			return { Took: took, End: Boolean(markdown) && markdown.textContent.trim().endsWith("end") };`, &drawn)
		return drawn.Took, drawn.End
	}
	plain, _ := draw(fill("", "plain words of a long message ", ""))
	hostile := map[string]string{
		"a heading padded to a line separator":   fill("# ", " ", separator),
		"a list item padded to a line separator": fill("* ", "\t", separator),
		"a fence padded to a line separator":     fill("~~~", "~", separator),
		"brackets around backticks":              fill("", "[`", ""),
		"a run of underscores":                   fill("#", "__", ""),
		"emphasis that never closes":             fill("", " *a", ""),
		"links that never finish":                fill("", "[a](", ""),
		"addresses that never finish":            fill("", "(http://", ""),
		"brackets that never close":              fill("", "[", ""),
	}
	for name, message := range hostile {
		took, end := draw(message)
		if !end {
			t.Errorf("%s was not drawn to its end", name)
		}
		if took > 20*plain+10 {
			t.Errorf("%s took %.1f ms to draw; plain text as long took %.1f", name, took, plain)
		}
	}
}

// A view takes messages from its host's window alone: a page beside it, or
// one it holds, cannot hand it a result to draw.
func TestOnlyTheHostSpeaksToTheView(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	forged, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "ui/notifications/tool-result", "params": map[string]any{
		"content": []any{}, "structuredContent": map[string]any{"post": post("p9", "mallory", "", "u9", "2026-10-09T09:30:00+02:00", "forged", nil), "root_id": "p9"},
	}})
	// Everything the view draws from here is kept.
	h.in(i, `w.seen = [];
		new w.MutationObserver(() => w.seen.push(d.getElementById("app").textContent)).observe(d.getElementById("app"), { childList: true, subtree: true });
		return true;`, nil)
	// A frame beside the view, in the host's page, and a frame inside the
	// view each post the forged result to it.
	h.eval(`(() => {
		const view = mmHost.frames[`+fmt.Sprint(i)+`].iframe;
		const send = (into) => {
			const frame = into.createElement("iframe");
			frame.srcdoc = "<script>(" + ((target, message) => { window.sent = true; target().postMessage(message, "*"); }).toString() + ")(() => " + (into === document ? "parent.mmHost.frames[`+fmt.Sprint(i)+`].iframe.contentWindow" : "parent") + ", " + JSON.stringify(`+string(forged)+`) + ")<\/script>";
			into.body.append(frame);
			return frame;
		};
		window.forgers = [send(document), send(view.contentDocument)];
		return true;
	})()`, nil)
	h.until("window.forgers.every((frame) => frame.contentWindow && frame.contentWindow.sent)")
	// Then the host itself sends a result, which arrives after theirs.
	later := post("p8", "alice", "", "u1", "2026-10-09T09:40:00+02:00", "from the host", nil)
	result, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "ui/notifications/tool-result", "params": map[string]any{
		"content": []any{}, "structuredContent": map[string]any{"post": later, "root_id": "p8", "thread": []any{later}},
	}})
	h.eval(fmt.Sprintf("(() => { mmHost.frames[%d].iframe.contentWindow.postMessage(%s, '*'); return true; })()", i, result), nil)
	h.untilIn(i, "d.querySelector('.markdown') && d.querySelector('.markdown').textContent === 'from the host'")
	var seen []string
	h.in(i, "return w.seen;", &seen)
	for _, text := range seen {
		if strings.Contains(text, "forged") {
			t.Fatalf("the view drew a result a page other than its host sent: %q", text)
		}
	}
}

func TestMoreOfAThreadComesThroughTheToolOnlyViewsCall(t *testing.T) {
	h := openHost(t)
	next := map[string]any{"root_id": "p1", "thread": []any{
		post("p5", "carol", "", "u3", "2026-10-10T09:00:00+02:00", "a later reply", map[string]any{"root_id": "p1"}),
	}}
	i := h.mount(showThread(map[string]any{"tools": map[string]any{"view_action": []any{map[string]any{"structuredContent": next, "content": []any{}}}}}))
	h.click(i, ".more")
	h.untilIn(i, "d.querySelectorAll('.post').length === 5")
	var call map[string]any
	h.in(i, "return f.calls[0];", &call)
	arguments, _ := call["arguments"].(map[string]any)
	if call["name"] != "view_action" || arguments["action"] != "more" || arguments["post_id"] != "p1" || arguments["cursor"] != "c1" {
		t.Errorf("the view asked for more with %v", call)
	}

	// Where the host refuses the call, the view says so and offers
	// Mattermost.
	refused := h.mount(showThread(map[string]any{"tools": map[string]any{"view_action": []any{map[string]any{"error": "Invalid params"}}}}))
	h.click(refused, ".more")
	h.untilIn(refused, "d.querySelector('.notice')")
	if got := h.text(refused, ".notice"); !strings.Contains(got, "does not load more") || !strings.Contains(got, "Open in Mattermost") {
		t.Errorf("a refused call says %q", got)
	}
}

func TestLinksOpenThroughTheHostAndARefusedOneIsShownToCopy(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	h.click(i, ".md-channel")
	h.click(i, ".time")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 2 || links[0] != "https://chat.example.com/team/channels/town-square" || links[1] != "https://chat.example.com/team/pl/p1" {
		t.Errorf("the links opened are %v", links)
	}
	refusing := h.mount(showThread(map[string]any{"refuseLinks": true}))
	h.click(refusing, ".time")
	h.untilIn(refusing, "d.querySelector('.notice')")
	if got := h.text(refusing, ".notice"); !strings.Contains(got, "https://chat.example.com/team/pl/p1") {
		t.Errorf("a refused link says %q", got)
	}
}

func TestAnImageOpensWholeInTheView(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	h.untilIn(i, "d.querySelector('.image-card img')")
	h.click(i, ".image-card")
	h.untilIn(i, "d.querySelector('.lightbox-image img')")
	var reads []string
	h.in(i, "return f.reads;", &reads)
	if !strings.Contains(strings.Join(reads, " "), base+"/files/f1 ") && reads[len(reads)-1] != base+"/files/f1" {
		t.Errorf("the whole image was not read: %v", reads)
	}
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
}

func TestAHostThatPassesNoPicturesOnShowsInitialsAndNames(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"without": []string{"serverResources"}}))
	var drawn struct {
		Reads    int
		Images   int
		Initials string
		Custom   string
		Card     string
	}
	h.in(i, `return {
		Reads: f.reads.length,
		Images: d.querySelectorAll('img').length,
		Initials: [...d.querySelectorAll('.avatar')].map((a) => a.textContent).join(' '),
		Custom: [...d.querySelectorAll('.emoji-custom')].map((e) => e.textContent).join(' '),
		Card: (d.querySelector('.image-card') || {}).textContent || '',
	};`, &drawn)
	if drawn.Reads != 0 || drawn.Images != 0 || drawn.Initials != "AE B S" || !strings.Contains(drawn.Custom, ":party_parrot:") || !strings.Contains(drawn.Card, "chart.png") {
		t.Errorf("without pictures the view draws %+v", drawn)
	}
}

func TestTheViewDoesNothingUntilThePersonDoesAndNeverSpeaksForThem(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(nil))
	h.untilIn(i, "d.querySelector('.image-card img')")
	var done struct{ Calls, Links, Contexts, Messages int }
	h.in(i, "return { Calls: f.calls.length, Links: f.links.length, Contexts: f.contexts.length, Messages: f.messages.length };", &done)
	if done != (struct{ Calls, Links, Contexts, Messages int }{}) {
		t.Errorf("the view acted on its own when it was drawn: %+v", done)
	}
	// Drawn again, as a host does when a conversation is reopened.
	h.eval(fmt.Sprintf("mmHost.frames[%d].bridge.sendToolResult(%s)", i, mustJSON(t, map[string]any{"structuredContent": thread(), "content": []any{}})), nil)
	h.click(i, ".md-channel")
	h.in(i, "return { Calls: f.calls.length, Links: f.links.length, Contexts: f.contexts.length, Messages: f.messages.length };", &done)
	if done.Calls != 0 || done.Messages != 0 || done.Links != 1 {
		t.Errorf("after a redraw and a click: %+v", done)
	}
	// Every control is a button, which a keyboard reaches.
	var controls struct{ Buttons, Others int }
	h.in(i, "return { Buttons: d.querySelectorAll('button').length, Others: d.querySelectorAll('a, [onclick], [tabindex]:not(.lightbox)').length };", &controls)
	if controls.Buttons < 5 || controls.Others != 0 {
		t.Errorf("the controls are %+v", controls)
	}
	h.eval(fmt.Sprintf("mmHost.teardown(%d)", i), nil)
}

func TestTheViewTakesTheHostsThemeAndLanguage(t *testing.T) {
	h := openHost(t)
	dark := h.mount(showThread(map[string]any{"theme": "dark", "locale": "fr-FR"}))
	var looks struct {
		Theme, Primary, Lang, Dir, Count string
	}
	h.in(dark, `return {
		Theme: d.documentElement.dataset.theme,
		Primary: d.documentElement.style.getPropertyValue('--color-background-primary'),
		Lang: d.documentElement.lang,
		Dir: d.documentElement.dir,
		Count: (d.querySelector('.count') || {}).textContent || '',
	};`, &looks)
	// French has no catalog yet, so the view speaks English, left to right.
	if looks.Theme != "dark" || looks.Primary != "#1f1f1e" || looks.Lang != "en" || looks.Dir != "ltr" || looks.Count != "40 replies" {
		t.Errorf("the dark French host's view looks %+v", looks)
	}
	light := h.mount(showThread(nil))
	var theme string
	h.in(light, "return d.documentElement.dataset.theme;", &theme)
	if theme != "light" {
		t.Errorf("the light host's view is %q", theme)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Isolated as a client isolates it, scripts alone and under the policy MCP
// Apps sets for a view that declares no domains, the page draws, reads its
// pictures as data, and reports its size; the documentation runs it so.
func TestThePageRunsIsolatedAsAClientRunsIt(t *testing.T) {
	h := openHost(t)
	page, _ := json.Marshal(views.Page(base, views.Settings{Changes: true, Posts: true}))
	settings, _ := json.Marshal(showThread(map[string]any{"isolated": true}))
	var i int
	h.eval("mmHost.mount("+string(page)+", "+string(settings)+")", &i)
	frame := fmt.Sprintf("mmHost.frames[%d]", i)
	h.until(frame + ".ready && " + frame + ".reads.length >= 3 && " + frame + ".sizes.some((s) => s.height > 200)")
	var sandbox string
	h.eval(frame+".iframe.getAttribute('sandbox')", &sandbox)
	if sandbox != "allow-scripts" {
		t.Errorf("the frame is sandboxed %q", sandbox)
	}
}

func TestAPostAloneIsShownWithoutItsThread(t *testing.T) {
	h := openHost(t)
	root := thread()["post"]
	i := h.mount(showThread(map[string]any{
		"input":  map[string]any{"kind": "thread", "post_id": "p1", "include_thread": false},
		"result": map[string]any{"content": []any{}, "structuredContent": map[string]any{"post": root, "root_id": "p1"}},
	}))
	var drawn struct {
		Posts int
		More  bool
		Title string
	}
	h.in(i, "return { Posts: d.querySelectorAll('.post').length, More: Boolean(d.querySelector('.more')), Title: d.querySelector('.thread-title').textContent };", &drawn)
	if drawn.Posts != 1 || drawn.More || drawn.Title != "Town Square" {
		t.Errorf("a post alone is drawn as %+v", drawn)
	}
}

// A thread of many people reads at most 60 of their pictures, however many
// there are, so a crowded thread cannot have the view make a request for each
// (#39).
func TestAViewReadsAtMostSixtyPictures(t *testing.T) {
	h := openHost(t)
	posts := []any{}
	for n := range 80 {
		posts = append(posts, post(fmt.Sprintf("p%d", n), fmt.Sprintf("user%d", n), "", fmt.Sprintf("u%02d", n),
			fmt.Sprintf("2026-10-09T09:%02d:00+02:00", n%60), "a word", map[string]any{"root_id": "p0"}))
	}
	data := map[string]any{"post": posts[0], "root_id": "p0", "thread": posts}
	i := h.mount(showThread(map[string]any{"result": map[string]any{"content": []any{}, "structuredContent": data}}))
	h.untilIn(i, "d.querySelectorAll('.post').length === 80")
	var reads int
	h.in(i, "return f.reads.length;", &reads)
	if reads != 60 {
		t.Errorf("a thread of 80 people read %d pictures; want the cap, 60", reads)
	}
}
