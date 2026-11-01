//go:build views

package views_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The client check in a browser (ADR-022): a page that tries what a host lets
// a view do, and says what it found. The host is the official app-bridge, each
// time leaving out or refusing what a real client was measured to.

// checkServer is the address the check is told is its Mattermost's. It never
// resolves, so these tests reach nothing.
const (
	checkServer = "https://chat.invalid"
	checkBase   = "mattermost://chat.invalid"
	ownPicture  = checkBase + "/users/u1/image"
)

// checkResult is check_views' result, as it would arrive whole.
func checkResult(stamp string) map[string]any {
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": "Shown to the person"}},
		"structuredContent": map[string]any{"stamp": stamp, "avatar_uri": ownPicture, "carries_pictures": false},
		"_meta":             map[string]any{"mm-mcp/check": map[string]any{"stamp": stamp}},
	}
}

// fixtures answers every read the check makes, as a host that passes them on
// does.
func fixtures() map[string]any {
	found := map[string]any{
		views.CheckURI: map[string]any{"mimeType": views.MIMEType, "text": "<!doctype html><html></html>"},
		ownPicture:     map[string]any{"mimeType": "image/png", "blob": pixel},
	}
	for _, fixture := range views.CheckFixtures() {
		if fixture.Text != "" {
			found[fixture.URI] = map[string]any{"mimeType": fixture.MIMEType, "text": fixture.Text}
			continue
		}
		found[fixture.URI] = map[string]any{"mimeType": fixture.MIMEType, "blob": base64.StdEncoding.EncodeToString(fixture.Blob)}
	}
	return found
}

// mountCheck puts the check page in a frame, with options as host.js takes
// them, each of which a test may add to or take from.
func (h *host) mountCheck(options map[string]any) int {
	h.t.Helper()
	settings := map[string]any{
		"result":    checkResult(views.Stamp()),
		"resources": fixtures(),
		"tools":     map[string]any{"view_check": []any{map[string]any{"content": []any{}, "structuredContent": map[string]any{"fixture_reads": 3, "calls": 1}}}},
	}
	for key, value := range options {
		settings[key] = value
	}
	page, _ := json.Marshal(views.CheckPage(checkBase, checkServer, "v0.0.0-test"))
	encodedSettings, _ := json.Marshal(settings)
	var index int
	h.eval("mmHost.mount("+string(page)+", "+string(encodedSettings)+")", &index)
	return index
}

// checkRow is one row of the report.
type checkRow struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// checkReport is what the report holds, as a test reads it.
type checkReport struct {
	Check            string              `json:"check"`
	Release          string              `json:"mm_mcp"`
	Scheme           string              `json:"server_scheme"`
	HostCapabilities map[string]any      `json:"host_capabilities"`
	ContextKeys      []string            `json:"host_context_keys"`
	Modes            []string            `json:"available_display_modes"`
	Checks           map[string]checkRow `json:"checks"`
	Changes          []map[string]any    `json:"display_changes"`
	Effects          []string            `json:"effects"`
	NotMeasured      []string            `json:"not_measured"`
}

// reported waits for the check to have told the model, which it does last, and
// reads the report from what it told.
func (h *host) reported(i int) (checkReport, string) {
	h.t.Helper()
	// A frame under a client's policy cannot be read into, so this asks the host.
	h.within(time.Minute, fmt.Sprintf(`mmHost.frames[%d].contexts.some((context) => context.content[0].text.includes('"model_context"'))`, i))
	return h.lastReport(i)
}

func (h *host) lastReport(i int) (checkReport, string) {
	h.t.Helper()
	var told string
	h.in(i, "return f.contexts[f.contexts.length - 1].content[0].text;", &told)
	start := strings.Index(told, "{")
	if start < 0 {
		h.t.Fatalf("the model was told %q", told)
	}
	var report checkReport
	if err := json.Unmarshal([]byte(told[start:]), &report); err != nil {
		h.t.Fatalf("reading the report %q: %v", told, err)
	}
	return report, told
}

func (h *host) wantStatus(report checkReport, id, want string) {
	h.t.Helper()
	if got := report.Checks[id].Status; got != want {
		h.t.Errorf("%s is %q (%s), want %q", id, got, report.Checks[id].Detail, want)
	}
}

func mentioning(lines []string, text string) bool {
	for _, line := range lines {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}

func TestTheCheckFindsWhatAFullHostDoes(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"csp": map[string]any{"connectDomains": []string{checkServer}}})
	report, _ := h.reported(i)

	if report.Check != "mm-mcp client check" || report.Release != "v0.0.0-test" || report.Scheme != "https" {
		t.Errorf("the report opens with %+v", report)
	}
	for _, id := range []string{"host", "frame", "page_age", "result", "read_page", "read_image", "read_text", "read_pdf", "read_template", "tool_call", "data_image", "model_context", "size", "policy"} {
		h.wantStatus(report, id, "works")
	}
	if !strings.Contains(report.Checks["tool_call"].Detail, "reads 3, calls 1") {
		t.Errorf("the tool's call says %q", report.Checks["tool_call"].Detail)
	}
	for _, id := range []string{"message"} {
		h.wantStatus(report, id, "declared")
	}
	h.wantStatus(report, "logging", "not_declared")
	if len(report.Effects) != 0 {
		t.Errorf("a host that does everything has these effects: %v", report.Effects)
	}
	if !strings.Contains(strings.Join(report.ContextKeys, " "), "availableDisplayModes") || len(report.Modes) != 2 {
		t.Errorf("the report has context keys %v and modes %v", report.ContextKeys, report.Modes)
	}
	if len(report.NotMeasured) != 2 {
		t.Errorf("the report says it cannot measure %v", report.NotMeasured)
	}

	// The same report goes to the server, where the host passes calls on.
	h.untilIn(i, "f.calls.some((call) => call.name === 'view_check' && call.arguments && call.arguments.report)")
	var sent string
	h.in(i, "return f.calls.filter((call) => call.arguments && call.arguments.report).pop().arguments.report;", &sent)
	if !strings.Contains(sent, `"mm_mcp": "v0.0.0-test"`) {
		t.Errorf("the server was sent %q", sent)
	}

	// And it stands on the page, to copy.
	h.untilIn(i, "d.querySelector('.check-report') && d.querySelector('.check-report').value.includes('client check')")
	var rows int
	h.in(i, "return d.querySelectorAll('[data-check]').length;", &rows)
	if rows < 25 {
		t.Errorf("the page draws %d rows", rows)
	}
}

// As Claude Code in the Claude desktop app was measured to behave: no tool
// calls, no reads but the page itself, and links refused.
func TestTheCheckSaysWhatAHostLeavesOutMeansForMmMcp(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{
		"without":   []string{"serverTools"},
		"resources": map[string]any{views.CheckURI: map[string]any{"mimeType": views.MIMEType, "text": "<!doctype html><html></html>"}},
		"tools":     map[string]any{},
	})
	report, _ := h.reported(i)

	h.wantStatus(report, "read_page", "works")
	for _, id := range []string{"read_image", "read_text", "read_pdf", "read_template", "tool_call"} {
		h.wantStatus(report, id, "fails")
	}
	if !mentioning(report.Effects, "cannot post, edit, discard") {
		t.Errorf("no tool calls means %v", report.Effects)
	}
	if !mentioning(report.Effects, "show as initials") {
		t.Errorf("no reads means %v", report.Effects)
	}
	var said string
	h.in(i, "return d.querySelector('.check-effects').textContent;", &said)
	if !strings.Contains(said, "A draft waits in Mattermost") {
		t.Errorf("the page says %q", said)
	}

	// With tool calls out, nothing tries to send the report that way again.
	var sends int
	h.in(i, "return f.calls.filter((call) => call.arguments && call.arguments.report).length;", &sends)
	if sends != 0 {
		t.Errorf("the check tried to send its report through %d calls that do not work", sends)
	}
}

func TestACheckWherePicturesAreCarriedSaysSo(t *testing.T) {
	h := openHost(t)
	result := checkResult(views.Stamp())
	result["structuredContent"].(map[string]any)["carries_pictures"] = true
	i := h.mountCheck(map[string]any{"result": result, "resources": map[string]any{}})
	report, _ := h.reported(i)
	if !mentioning(report.Effects, "brings its own with it") || mentioning(report.Effects, "here: people show as initials") {
		t.Errorf("where pictures are carried, the effects are %v", report.Effects)
	}
}

// A read that answers with something else, as Goose's binary resources do, is
// no read that works.
func TestACheckHoldsAReadToWhatItShouldBe(t *testing.T) {
	h := openHost(t)
	wrong := fixtures()
	for _, fixture := range views.CheckFixtures() {
		wrong[fixture.URI] = map[string]any{"mimeType": fixture.MIMEType, "blob": "AAAA", "text": "not what was put there"}
	}
	i := h.mountCheck(map[string]any{"resources": wrong})
	report, _ := h.reported(i)
	for _, id := range []string{"read_image", "read_text", "read_pdf"} {
		h.wantStatus(report, id, "fails")
		if !strings.Contains(report.Checks[id].Detail, "something else") {
			t.Errorf("%s says %q", id, report.Checks[id].Detail)
		}
	}
}

// A client that keeps a page for its session goes on drawing the old one after
// the server is rebuilt.
func TestACheckDrawnByAnOlderPageSaysToStartANewSession(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"result": checkResult("a-newer-build")})
	report, _ := h.reported(i)
	h.wantStatus(report, "page_age", "fails")
	if !mentioning(report.Effects, "start a new session") {
		t.Errorf("an older page means %v", report.Effects)
	}
}

// What did not arrive of the result is said by name, and what cannot be told
// without it is skipped.
func TestACheckSaysWhichPartOfTheResultDidNotArrive(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"result": map[string]any{
		"content": []any{map[string]any{"type": "text", "text": "Shown"}},
	}})
	report, _ := h.reported(i)
	h.wantStatus(report, "result", "partly")
	if !strings.Contains(report.Checks["result"].Detail, "structuredContent, _meta") {
		t.Errorf("the result row says %q", report.Checks["result"].Detail)
	}
	h.wantStatus(report, "page_age", "skipped")
	h.wantStatus(report, "read_template", "skipped")
	if !mentioning(report.Effects, "nothing to draw") {
		t.Errorf("no structured output means %v", report.Effects)
	}
}

// The report gets pasted into issues, so it holds neither the server's address
// nor the person's picture, even where the host repeats them.
func TestTheReportHoldsNeitherTheServerNorThePerson(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{
		"resources": map[string]any{},
		"tools":     map[string]any{"view_check": []any{map[string]any{"error": "refused " + checkServer + " and " + ownPicture}}},
		"csp":       map[string]any{"connectDomains": []string{checkServer}},
	})
	_, told := h.reported(i)
	for _, private := range []string{checkServer, checkBase, "chat.invalid", "users/u1"} {
		if strings.Contains(told, private) {
			t.Errorf("the report holds %s:\n%s", private, told)
		}
	}
	if !strings.Contains(told, "<server>") {
		t.Errorf("the report does not name <server> where it stood:\n%s", told)
	}
	// A screenshot of the page is as easily posted as the report.
	var shown string
	h.in(i, "return d.getElementById('app').textContent;", &shown)
	for _, private := range []string{checkServer, checkBase, "chat.invalid", "users/u1"} {
		if strings.Contains(shown, private) {
			t.Errorf("the page shows %s:\n%s", private, shown)
		}
	}
}

// Under the policy a client's frame sets, the check reports what stopped it,
// and whether that is what the client said it approved.
func TestTheCheckReportsWhatAPolicyStopsAndWhatTheHostSaidItApproved(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{
		"isolated": true,
		"csp":      map[string]any{"connectDomains": []string{checkServer}, "resourceDomains": []string{checkServer}},
	})
	report, _ := h.reported(i)
	for id, directive := range map[string]string{"fetch_server": "connect-src", "image_server": "img-src", "fetch_unlisted": "connect-src", "image_unlisted": "img-src"} {
		h.wantStatus(report, id, "blocked")
		if !strings.Contains(report.Checks[id].Detail, directive) {
			t.Errorf("%s says %q, not %s", id, report.Checks[id].Detail, directive)
		}
	}
	h.wantStatus(report, "data_image", "works")
	h.wantStatus(report, "policy", "partly")
	if !strings.Contains(report.Checks["policy"].Detail, "still stops it") {
		t.Errorf("the policy row says %q", report.Checks["policy"].Detail)
	}
}

// Without a policy in the way, an address that does not exist is not reported
// as stopped by one.
func TestAnAddressThatDoesNotAnswerIsNotCalledBlocked(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"csp": map[string]any{"connectDomains": []string{checkServer}}})
	report, _ := h.reported(i)
	for _, id := range []string{"fetch_server", "image_server", "fetch_unlisted", "image_unlisted"} {
		h.wantStatus(report, id, "no_answer")
	}
	h.wantStatus(report, "policy", "works")
}

func TestAPressReportsWhatTheHostAnswersToALink(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"refuseLinks": true})
	h.reported(i)
	h.click(i, "[data-check='open_link'] .check-press")
	h.untilIn(i, "f.links.length === 1")
	h.click(i, "[data-check='open_link_other'] .check-press")
	h.untilIn(i, "f.links.length === 2")
	var links []string
	h.in(i, "return f.links;", &links)
	if len(links) != 2 || links[0] != checkServer || links[1] != "http://chat.invalid" {
		t.Errorf("the presses opened %v", links)
	}
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('open_link_other'))")
	report, _ := h.lastReport(i)
	h.wantStatus(report, "open_link", "fails")
	h.wantStatus(report, "open_link_other", "fails")
	// A pasted report says which scheme each answer was to.
	if !strings.Contains(report.Checks["open_link"].Detail, "https") || !strings.Contains(report.Checks["open_link_other"].Detail, "http address") {
		t.Errorf("the rows say %q and %q", report.Checks["open_link"].Detail, report.Checks["open_link_other"].Detail)
	}
	if !mentioning(report.Effects, "Links do not open") {
		t.Errorf("refused links mean %v", report.Effects)
	}
}

func TestThePersonSaysWhetherAPlainLinkOrAWindowOpened(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(nil)
	h.reported(i)
	// A plain link goes nowhere in this test: the click is on the anchor, and
	// what the row then asks is what is held.
	h.in(i, "const a = d.querySelector(\"[data-check='plain_link'] a\"); a.addEventListener('click', (e) => e.preventDefault()); a.click(); return a.getAttribute('rel');", nil)
	h.untilIn(i, "d.querySelector(\"[data-check='plain_link'] .check-status\").dataset.status === 'asking'")
	h.click(i, "[data-check='plain_link'] .check-detail button:last-of-type")
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('You saw nothing open'))")
	report, _ := h.lastReport(i)
	h.wantStatus(report, "plain_link", "fails")
}

func TestAPressReportsWhatTheHostAnswersToAMode(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(nil)
	h.reported(i)
	// Asking to leave fullscreen waits until the view is in it.
	var disabled bool
	h.in(i, "return d.querySelector(\"[data-check='display_inline'] .check-press\").disabled;", &disabled)
	if !disabled {
		t.Error("the view offers to leave a fullscreen it is not in")
	}
	h.click(i, "[data-check='display_fullscreen'] .check-press")
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('asked for fullscreen'))")
	report, _ := h.lastReport(i)
	h.wantStatus(report, "display_fullscreen", "fails")
	if detail := report.Checks["display_fullscreen"].Detail; !strings.Contains(detail, "From inline, asked for fullscreen, answered inline; now inline") {
		t.Errorf("the row says %q", detail)
	}

	// The host's own button is logged as it goes, and asking to leave then
	// says what the host answered and where the view stands.
	h.eval(fmt.Sprintf("mmHost.setDisplayMode(%d, 'fullscreen')", i), nil)
	h.untilIn(i, "d.documentElement.dataset.display === 'fullscreen'")
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('\"display_mode\": \"fullscreen\"'))")
	h.untilIn(i, "!d.querySelector(\"[data-check='display_inline'] .check-press\").disabled")
	h.click(i, "[data-check='display_inline'] .check-press")
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('asked for inline'))")
	report, _ = h.lastReport(i)
	if len(report.Changes) == 0 {
		t.Error("the host's own button left no mark in the report")
	}
	if detail := report.Checks["display_inline"].Detail; !strings.Contains(detail, "From fullscreen, asked for inline, answered inline; now fullscreen") {
		t.Errorf("the row says %q", detail)
	}
	h.wantStatus(report, "display_inline", "fails")
}

func TestAHostThatGrantsAModeIsReportedAsWorking(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"grantDisplayModes": true})
	h.reported(i)
	h.click(i, "[data-check='display_fullscreen'] .check-press")
	h.untilIn(i, "f.contexts.some((context) => context.content[0].text.includes('asked for fullscreen'))")
	report, _ := h.lastReport(i)
	h.wantStatus(report, "display_fullscreen", "works")
}

func TestTheReportCanBeCopiedOrSelected(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(nil)
	h.reported(i)
	h.untilIn(i, "d.querySelector('.check-report')")
	h.click(i, ".check-report + .check-press")
	h.untilIn(i, "d.activeElement && d.activeElement.classList.contains('check-report')")
	var selected int
	h.in(i, "const area = d.querySelector('.check-report'); return area.selectionEnd - area.selectionStart;", &selected)
	if selected == 0 {
		t.Error("the report is not selected to copy by hand")
	}
}

// A host may hold a view to a height, as VS Code does: that is no frame failing
// to follow, and a scrollbar there is the view's way of showing the rest.
func TestAHostThatHoldsAViewToAHeightIsNotCalledBroken(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"maxHeight": 300})
	report, _ := h.reported(i)
	h.wantStatus(report, "size", "partly")
	if detail := report.Checks["size"].Detail; !strings.Contains(detail, "It stops at 300") {
		t.Errorf("the size row says %q", detail)
	}
}

// A frame that does not follow what a view reports is called out.
func TestAFrameThatDoesNotFollowTheViewIsCalledOut(t *testing.T) {
	h := openHost(t)
	// The host stops listening, so the frame keeps a height the view fits in,
	// and does not grow when the view does.
	i := h.mountCheck(nil)
	h.in(i, "f.iframe.style.height = '4000px'; f.bridge.onsizechange = () => {}; return null;", nil)
	report, _ := h.reported(i)
	h.wantStatus(report, "size", "fails")
	if detail := report.Checks["size"].Detail; !strings.Contains(detail, "then 4000 when it grew") {
		t.Errorf("the size row says %q", detail)
	}
}

// Run again out of sight, as a host may when it redraws a conversation, the
// frame says nothing of how one in sight behaves.
func TestAViewRunOutOfSightDoesNotMeasureItsHeight(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"prelude": "Object.defineProperty(document, 'visibilityState', { get: () => 'hidden' });"})
	report, _ := h.reported(i)
	h.wantStatus(report, "size", "skipped")
}

// A host that listens is told at once, even where the page draws no frames, as
// one out of sight does not: its height is measured from what it was told.
func TestTheHeightCheckReportsWithoutWaitingForAFrame(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"prelude": "window.requestAnimationFrame = () => 0;"})
	report, _ := h.reported(i)
	h.wantStatus(report, "size", "works")
}

// A fetch that gets through is reported as reaching the address, whatever it
// answered: the check asks only whether the frame lets it.
func TestAFetchThatGetsThroughIsReportedAsReaching(t *testing.T) {
	h := openHost(t)
	i := h.mountCheck(map[string]any{"prelude": "window.fetch = async () => new Response(null, { status: 204 });"})
	report, _ := h.reported(i)
	h.wantStatus(report, "fetch_server", "reached")
	h.wantStatus(report, "fetch_unlisted", "reached")
}

// Outside a host, as when the page is opened on its own, nothing answers it,
// and it says what it is.
func TestTheCheckOutsideAHostSaysWhatItIs(t *testing.T) {
	h := openHost(t)
	page, _ := json.Marshal(views.CheckPage(checkBase, checkServer, "v0"))
	h.eval("(() => { const frame = document.createElement('iframe'); frame.id = 'alone'; frame.srcdoc = "+string(page)+"; document.body.append(frame); return true; })()", nil)
	h.within(30*time.Second, "document.getElementById('alone').contentDocument && document.getElementById('alone').contentDocument.querySelector('.notice')")
	var said string
	h.eval("document.getElementById('alone').contentDocument.querySelector('.notice').textContent", &said)
	if !strings.Contains(said, "inside an app that shows views") {
		t.Errorf("on its own, the page says %q", said)
	}
}
