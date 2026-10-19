//go:build views

package views_test

import (
	"fmt"
	"strings"
	"testing"
)

// A proposal: what propose_post answers with, drawn as it will be posted,
// and what the person presses on it, which goes through view_change
// (ADR-035). The host's answers to the presses are the test's.

// proposal is a draft as propose_post answers with it.
func proposal(message string, more map[string]any) map[string]any {
	out := map[string]any{
		"channel_id": "c1", "channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team",
		"address": base + "/team/channels/town-square", "url": "https://chat.example.com/team/channels/town-square",
		"message": message, "length": len([]rune(message)), "saved": true,
	}
	for key, value := range more {
		out[key] = value
	}
	return out
}

func showProposal(proposed map[string]any, extra map[string]any) map[string]any {
	options := map[string]any{
		"input":     map[string]any{"channel_id": "c1", "message": proposed["message"]},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Proposed"}}, "structuredContent": proposed},
		"resources": pictures(),
	}
	for key, value := range extra {
		options[key] = value
	}
	return options
}

// answered is a result of view_change as the host passes it back.
func answered(change map[string]any) map[string]any {
	return map[string]any{"structuredContent": change, "content": []any{}}
}

// buttons is the text of every button the view shows, in order.
func (h *host) buttons(index int) []string {
	h.t.Helper()
	var found []string
	h.in(index, "return [...d.querySelectorAll('button')].map((b) => b.textContent.trim());", &found)
	return found
}

// press clicks the button whose text is name.
func (h *host) press(index int, name string) {
	h.t.Helper()
	var pressed bool
	h.in(index, fmt.Sprintf("const b = [...d.querySelectorAll('button')].find((b) => b.textContent.trim() === %q); if (b) b.click(); return Boolean(b);", name), &pressed)
	if !pressed {
		h.t.Fatalf("no button %q among %v", name, h.buttons(index))
	}
}

func (h *host) calls(index int) []map[string]any {
	h.t.Helper()
	var calls []map[string]any
	h.in(index, "return f.calls;", &calls)
	return calls
}

func has(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestAProposalIsShownAsItWillBePostedHidingNothing(t *testing.T) {
	h := openHost(t)
	message := "The plan is [here](https://files.example.net/plan) and at https://chat.example.com/team/pl/p1 ‮evil‬"
	i := h.mount(showProposal(proposal(message, map[string]any{"notes": []string{"@here notifies everyone in the channel who is online: 12 people in ~Town Square."}}), nil))
	var drawn struct {
		Label, Title, Team, Status, Warning string
		Targets                             []string
		Notes                               int
	}
	h.in(i, `const text = (selector) => { const e = d.querySelector(selector); return e ? e.textContent.trim() : ''; };
		return {
			Label: text('.posted-label'), Title: text('.thread-title'), Team: text('.team'), Status: text('.proposal-status'),
			Warning: text('.proposal-warning'), Notes: d.querySelectorAll('.proposal-notes li').length,
			Targets: [...d.querySelectorAll('.md-link-target')].map((e) => e.textContent),
		};`, &drawn)
	if drawn.Label != "Proposed post" || drawn.Title != "Town Square" || drawn.Team != "Team" || drawn.Notes != 1 ||
		!strings.Contains(drawn.Status, "Nothing is sent until you post it") {
		t.Errorf("the proposal is drawn as %+v", drawn)
	}
	// A link shows where it leads where its text says otherwise, and a bare
	// address is its own target.
	if len(drawn.Targets) != 1 || drawn.Targets[0] != "https://files.example.net/plan" {
		t.Errorf("the link targets shown are %v", drawn.Targets)
	}
	if !strings.Contains(drawn.Warning, "2 characters you cannot see") {
		t.Errorf("the hidden characters are flagged as %q", drawn.Warning)
	}
	for _, want := range []string{"Post", "Edit", "Discard", "Open the draft in Mattermost"} {
		if got := h.buttons(i); !has(got, want) {
			t.Errorf("the view offers %v; want %s", got, want)
		}
	}
	h.press(i, "Show the text as written")
	var hidden []string
	h.in(i, "return [...d.querySelectorAll('.hidden-char')].map((e) => e.textContent);", &hidden)
	if strings.Join(hidden, " ") != "U+202E U+202C" {
		t.Errorf("as written, the hidden characters are %v", hidden)
	}
	if got := h.text(i, ".as-written"); !strings.Contains(got, "[here](https://files.example.net/plan)") {
		t.Errorf("as written the text reads %q", got)
	}
	// Drawn, and drawn again, the view presses nothing on its own.
	h.eval(fmt.Sprintf("mmHost.frames[%d].bridge.sendToolResult(%s)", i, mustJSON(t, answered(proposal(message, nil)))), nil)
	if calls := h.calls(i); len(calls) != 0 {
		t.Errorf("the view called %v on its own", calls)
	}

	reply := h.mount(showProposal(proposal("I can take it", map[string]any{"root_id": "p1", "url": "https://chat.example.com/team/pl/p1"}), nil))
	if got := h.text(reply, ".posted-label"); got != "Proposed reply in a thread" {
		t.Errorf("a reply is labelled %q", got)
	}
}

func TestPostingAProposalGoesThroughTheToolOnlyViewsCall(t *testing.T) {
	h := openHost(t)
	posted := post("p9", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "The notes are ready.",
		map[string]any{"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/town-square", "team": "Team", "ai_generated": true})
	i := h.mount(showProposal(proposal("The notes are ready.", map[string]any{"root_id": "p1"}),
		map[string]any{"tools": map[string]any{"view_change": []any{answered(map[string]any{"status": "posted", "post": posted})}}}))
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.posted-label') && d.querySelector('.posted-label').textContent === 'Posted'")
	calls := h.calls(i)
	arguments, _ := calls[0]["arguments"].(map[string]any)
	if len(calls) != 1 || calls[0]["name"] != "view_change" || arguments["action"] != "post" || arguments["channel_id"] != "c1" ||
		arguments["root_id"] != "p1" || arguments["shown"] != "The notes are ready." || arguments["confirmed"] != nil {
		t.Errorf("Post called %v", calls)
	}
	var told struct{ Contexts, Messages int }
	var context string
	h.in(i, "return { Contexts: f.contexts.length, Messages: f.messages.length };", &told)
	h.in(i, "return f.contexts.length ? f.contexts[0].content[0].text : '';", &context)
	if told.Contexts != 1 || told.Messages != 0 || !strings.Contains(context, "posted the proposal") || !strings.Contains(context, `"The notes are ready."`) {
		t.Errorf("after posting the view told %+v: %q", told, context)
	}
}

func TestAMessageThatAlertsAWholeChannelIsConfirmedAsMattermostAsks(t *testing.T) {
	h := openHost(t)
	confirm := answered(map[string]any{"status": "confirm", "notifies": map[string]any{"people": 12, "mentions": []string{"@channel"}, "timezones": 3}})
	posted := post("p9", "alice", "", "u1", "2026-10-09T09:30:00+02:00", "@channel lunch", nil)
	i := h.mount(showProposal(proposal("@channel lunch", nil),
		map[string]any{"tools": map[string]any{"view_change": []any{confirm, confirm, answered(map[string]any{"status": "posted", "post": posted})}}}))
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.confirm')")
	if got := h.text(i, ".confirm-title"); got != "Confirm sending notifications to entire channel" {
		t.Errorf("the confirmation is titled %q", got)
	}
	if got := h.text(i, ".confirm-question"); got != "By using @all or @channel you are about to send notifications to 12 people in 3 timezones. Are you sure you want to do this?" {
		t.Errorf("the confirmation asks %q", got)
	}
	h.press(i, "Cancel")
	h.untilIn(i, "!d.querySelector('.confirm')")
	if calls := h.calls(i); len(calls) != 1 {
		t.Errorf("cancelling called %d times", len(calls))
	}
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.confirm')")
	h.press(i, "Confirm")
	h.untilIn(i, "d.querySelector('.posted-label')")
	calls := h.calls(i)
	last, _ := calls[len(calls)-1]["arguments"].(map[string]any)
	if len(calls) != 3 || last["confirmed"] != true {
		t.Errorf("the confirmed press called %v", calls)
	}

	// @here, alone and without timezones, says so.
	here := h.mount(showProposal(proposal("@here lunch", nil), map[string]any{"tools": map[string]any{"view_change": []any{
		answered(map[string]any{"status": "confirm", "notifies": map[string]any{"people": 7, "mentions": []string{"@here"}}}),
	}}}))
	h.press(here, "Post")
	h.untilIn(here, "d.querySelector('.confirm')")
	if got := h.text(here, ".confirm-question"); got != "By using @here you are about to send notifications to at least 7 other people. Are you sure you want to do this?" {
		t.Errorf("the confirmation for @here asks %q", got)
	}
}

func TestAProposalIsEditedAndDiscardedThroughTheToolOnlyViewsCall(t *testing.T) {
	h := openHost(t)
	i := h.mount(showProposal(proposal("Lunch at noon", nil), map[string]any{"tools": map[string]any{"view_change": []any{
		answered(map[string]any{"status": "saved", "draft": proposal("Lunch at one", nil)}),
		answered(map[string]any{"status": "discarded"}),
	}}}))
	h.press(i, "Edit")
	h.in(i, "const box = d.querySelector('.proposal-editor'); box.value = 'Lunch at one'; box.dispatchEvent(new w.Event('input')); return null;", nil)
	h.press(i, "Save")
	h.untilIn(i, "d.querySelector('.proposal-text') && d.querySelector('.proposal-text').textContent.includes('Lunch at one')")
	if got := h.text(i, ".notice"); !strings.Contains(got, "Saved in your draft.") {
		t.Errorf("the edit says %q", got)
	}
	h.press(i, "Discard")
	h.untilIn(i, "d.querySelector('.proposal-status') && d.querySelector('.proposal-status').textContent.startsWith('Discarded')")
	calls := h.calls(i)
	edit, _ := calls[0]["arguments"].(map[string]any)
	discard, _ := calls[1]["arguments"].(map[string]any)
	if edit["action"] != "edit" || edit["shown"] != "Lunch at noon" || edit["message"] != "Lunch at one" || discard["action"] != "discard" || discard["shown"] != "Lunch at one" {
		t.Errorf("the presses called %v", calls)
	}
	var contexts []string
	h.in(i, "return f.contexts.map((c) => c.content[0].text);", &contexts)
	if len(contexts) != 2 || !strings.Contains(contexts[0], "is theirs now") || !strings.Contains(contexts[1], "discarded the proposal") {
		t.Errorf("the view told the model %q", contexts)
	}
}

func TestADraftChangedInMattermostIsDrawnAsItIsNow(t *testing.T) {
	h := openHost(t)
	i := h.mount(showProposal(proposal("Lunch at noon", nil), map[string]any{"tools": map[string]any{"view_change": []any{
		answered(map[string]any{"status": "changed", "draft": proposal("Lunch at noon, and dessert", nil)}),
		answered(map[string]any{"status": "gone"}),
	}}}))
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.proposal-text').textContent.includes('and dessert')")
	if got := h.text(i, ".notice"); !strings.Contains(got, "The draft changed in Mattermost") {
		t.Errorf("a changed draft says %q", got)
	}
	// What the person sees now is what the next press names.
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.proposal-status').textContent.includes('no longer waits')")
	calls := h.calls(i)
	second, _ := calls[1]["arguments"].(map[string]any)
	if second["shown"] != "Lunch at noon, and dessert" {
		t.Errorf("the second press named %v", second["shown"])
	}

	elsewhere := h.mount(showProposal(proposal("see the plan", nil), map[string]any{"tools": map[string]any{"view_change": []any{
		answered(map[string]any{"status": "elsewhere", "draft": proposal("see the plan", nil)}),
	}}}))
	h.press(elsewhere, "Discard")
	h.untilIn(elsewhere, "d.querySelector('.proposal-status').textContent.includes('which this view cannot show')")
}

func TestThePersonsOwnDraftIsKeptOrReplacedOnTheirPress(t *testing.T) {
	h := openHost(t)
	theirs := proposal("Friday works", map[string]any{"length": 12})
	choosing := proposal("Friday works for me, see you there", map[string]any{"saved": false, "theirs": theirs})
	i := h.mount(showProposal(choosing, nil))
	var headings []string
	h.in(i, "return [...d.querySelectorAll('.proposal-heading')].map((e) => e.textContent);", &headings)
	if strings.Join(headings, " | ") != "Your draft | Proposal" {
		t.Errorf("the choice shows %v", headings)
	}
	if got := h.buttons(i); !has(got, "Replace mine") || !has(got, "Keep mine") || has(got, "Post") {
		t.Errorf("the choice offers %v", got)
	}
	h.press(i, "Keep mine")
	h.untilIn(i, "d.querySelector('.proposal-status').textContent.startsWith('Kept your draft')")
	if calls := h.calls(i); len(calls) != 0 {
		t.Errorf("keeping called %v", calls)
	}

	replace := h.mount(showProposal(choosing, map[string]any{"tools": map[string]any{"view_change": []any{
		answered(map[string]any{"status": "saved", "draft": proposal("Friday works for me, see you there", nil)}),
	}}}))
	h.press(replace, "Replace mine")
	h.untilIn(replace, "[...d.querySelectorAll('button')].some((b) => b.textContent === 'Post')")
	calls := h.calls(replace)
	arguments, _ := calls[0]["arguments"].(map[string]any)
	if arguments["action"] != "replace" || arguments["shown"] != "Friday works" || arguments["message"] != "Friday works for me, see you there" {
		t.Errorf("Replace mine called %v", calls)
	}

	// A revision the person asked for shows what changes, word by word.
	revised := showProposal(choosing, nil)
	revised["input"] = map[string]any{"channel_id": "c1", "message": choosing["message"], "revises_draft": true}
	r := h.mount(revised)
	var diff struct{ Added, Removed string }
	h.in(r, "return { Added: [...d.querySelectorAll('.diff ins')].map((e) => e.textContent).join('|'), Removed: [...d.querySelectorAll('.diff del')].map((e) => e.textContent).join('|') };", &diff)
	if diff.Added != " for me, see you there" || diff.Removed != "" {
		t.Errorf("the revision shows %+v", diff)
	}
	if got := h.buttons(r); !has(got, "Apply") || has(got, "Replace mine") {
		t.Errorf("the revision offers %v", got)
	}
}

func TestARefusedPressSaysSoAndOffersTheDraftInMattermost(t *testing.T) {
	h := openHost(t)
	i := h.mount(showProposal(proposal("Lunch", nil), map[string]any{"tools": map[string]any{"view_change": []any{map[string]any{"error": "Invalid params"}}}}))
	h.press(i, "Post")
	h.untilIn(i, "d.querySelector('.notice')")
	if got := h.text(i, ".notice"); !strings.Contains(got, "This app does not let the view change the draft here.") || !strings.Contains(got, "Open in Mattermost") {
		t.Errorf("a refused press says %q", got)
	}
	// A press mm-mcp refuses says why.
	why := "the message mentions someone nobody is called: @bobb"
	j := h.mount(showProposal(proposal("hi @bobb", nil), map[string]any{"tools": map[string]any{"view_change": []any{
		map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": why}}},
	}}}))
	h.press(j, "Post")
	h.untilIn(j, "d.querySelector('.notice')")
	if got := h.text(j, ".notice"); !strings.Contains(got, why) {
		t.Errorf("a press mm-mcp refused says %q", got)
	}
}

func TestAServerThatDoesNotPostFromViewsOffersNoPost(t *testing.T) {
	h := openHost(t)
	i := h.mount(showProposal(proposal("Lunch", nil), map[string]any{"postsOff": true}))
	if got := h.buttons(i); has(got, "Post") || !has(got, "Edit") || !has(got, "Discard") {
		t.Errorf("without posting from views the view offers %v", got)
	}
	if got := h.text(i, ".proposal-status"); !strings.Contains(got, "for you to send from there") {
		t.Errorf("without posting from views the view says %q", got)
	}
	// A host that calls no tools offers the draft in Mattermost alone.
	none := h.mount(showProposal(proposal("Lunch", nil), map[string]any{"without": []string{"serverTools"}}))
	if got := h.buttons(none); has(got, "Post") || has(got, "Edit") || !has(got, "Open the draft in Mattermost") {
		t.Errorf("a host that calls no tools is offered %v", got)
	}
}
