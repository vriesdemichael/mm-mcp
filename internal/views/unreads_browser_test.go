//go:build views

package views_test

import (
	"strings"
	"testing"
)

// The unread view, in the official bridge: what show answers for kind
// unreads, made up here as get_user_channels with unread_only, list_mentions
// and list_threads with unread_only answer (ADR-022).

func channel(id, display, kind, team string, unread, mentions int) map[string]any {
	out := map[string]any{
		"id": id, "name": id, "display_name": display, "type": kind,
		"url":             "https://chat.example.com/team/channels/" + id,
		"unread_messages": unread, "mentions": mentions,
	}
	if team != "" {
		out["team"] = team
	}
	return out
}

// unreads is what is unread for alice: channels in two teams, a direct and a
// group message, a mention, and a followed thread, with more in Mattermost
// than the view lists, and a channel named to break the page.
func unreads() map[string]any {
	place := map[string]any{"channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/c1", "team": "Team One"}
	return map[string]any{
		"channels": map[string]any{"channels": []any{
			channel("c1", "Town Square", "public", "Team One", 5, 1),
			channel("c2", "bob", "direct", "", 2, 2),
			channel("c3", "Release <img src=x onerror=\"window.parent.pwned=1\">", "private", "Team Two", 1, 0),
			channel("c4", "bob, carol", "group", "", 3, 0),
		}, "next_cursor": "c-more"},
		"mentions": map[string]any{
			"posts":        []any{post("m1", "bob", "Bob Example", "u2", "2026-10-09T10:00:00+02:00", "@alice can you look at the build?", place)},
			"mention_keys": []any{"@alice"},
		},
		"threads": map[string]any{"threads": []any{map[string]any{
			"root_id":         "p1",
			"started":         post("p1", "alice", "Alice Example", "u1", "2026-10-09T09:30:00+02:00", "Who takes the release notes?", place),
			"reply_count":     4,
			"participants":    []any{"bob", "carol"},
			"unread_replies":  2,
			"unread_mentions": 1,
		}}},
	}
}

func showUnreads(extra map[string]any) map[string]any {
	options := map[string]any{
		"input":     map[string]any{"kind": "unreads"},
		"result":    map[string]any{"content": []any{map[string]any{"type": "text", "text": "Shown to the person"}}, "structuredContent": unreads()},
		"resources": pictures(),
	}
	for key, value := range extra {
		options[key] = value
	}
	return options
}

// opened is a channel's unread messages, as read_unread answers them.
func opened() map[string]any {
	return map[string]any{
		"channel_id": "c1", "channel": "Town Square", "channel_url": "https://chat.example.com/team/channels/c1", "team": "Team One",
		"posts": []any{
			post("r1", "bob", "", "u2", "2026-10-09T09:00:00+02:00", "read already", nil),
			post("r2", "carol", "", "u3", "2026-10-09T11:00:00+02:00", "this one is new", nil),
		},
		"first_unread_id": "r2", "unread": 1,
	}
}

func TestWhatIsUnreadIsListedByTeamWithMattermostsCounts(t *testing.T) {
	h := openHost(t)
	i := h.mount(showUnreads(nil))
	var drawn struct {
		Headings, Teams, Names, Counts, Notes []string
		Badges                                []string
		Excerpt                               string
		Hostile                               bool
	}
	h.in(i, `const all = (selector) => [...d.querySelectorAll(selector)].map((e) => e.textContent.trim());
		return {
			Headings: all('.unread-heading'),
			Teams: all('.unread-team'),
			Names: all('.unread-section[aria-label="Channels"] .unread-name'),
			Counts: all('.unread-section[aria-label="Channels"] .unread-count'),
			Badges: all('.badge'),
			Notes: all('.more-note'),
			Excerpt: d.querySelector('.unread-excerpt') ? d.querySelector('.unread-excerpt').textContent : '',
			Hostile: Boolean(d.querySelector('img[src="x"]')) || Boolean(window.pwned),
		};`, &drawn)
	if strings.Join(drawn.Headings, "|") != "Channels|Mentions|Followed threads" {
		t.Errorf("the sections are %q", drawn.Headings)
	}
	// Mattermost's order, grouped by team where each first appears, direct
	// and group messages together.
	if strings.Join(drawn.Teams, "|") != "Team One|Direct messages|Team Two" {
		t.Errorf("the groups are %q", drawn.Teams)
	}
	if strings.Join(drawn.Names, "|") != `Town Square|@bob|bob, carol|Release <img src=x onerror="window.parent.pwned=1">` {
		t.Errorf("the channels are %q", drawn.Names)
	}
	if strings.Join(drawn.Counts, "|") != "5 unread|2 unread|3 unread|1 unread" {
		t.Errorf("the counts are %q", drawn.Counts)
	}
	if strings.Join(drawn.Badges, "|") != "@1|@2|@1" {
		t.Errorf("the mention badges are %q", drawn.Badges)
	}
	if len(drawn.Notes) != 1 || !strings.Contains(drawn.Notes[0], "More channels") {
		t.Errorf("the view says of what it leaves out %q", drawn.Notes)
	}
	if drawn.Excerpt != "Who takes the release notes?" {
		t.Errorf("the thread reads %q", drawn.Excerpt)
	}
	if drawn.Hostile {
		t.Error("a channel's name was parsed as HTML")
	}
	if got := h.text(i, ".unread-section[aria-label=\"Mentions\"] .markdown"); got != "@alice can you look at the build?" {
		t.Errorf("the mention reads %q", got)
	}

	empty := h.mount(showUnreads(map[string]any{"result": map[string]any{"structuredContent": map[string]any{
		"channels": map[string]any{"channels": []any{}}, "mentions": map[string]any{"posts": []any{}, "mention_keys": []any{}}, "threads": map[string]any{"threads": []any{}},
	}}}))
	if got := h.text(empty, "[role=status]"); got != "Nothing is unread." {
		t.Errorf("nothing unread reads %q", got)
	}
}

func TestAChannelOrAThreadOpensInPlaceThroughTheToolOnlyViewsCall(t *testing.T) {
	h := openHost(t)
	i := h.mount(showUnreads(map[string]any{"tools": map[string]any{"view_action": []any{
		map[string]any{"structuredContent": opened(), "content": []any{}},
		map[string]any{"structuredContent": thread(), "content": []any{}},
	}}}))
	h.click(i, ".unread-name")
	h.untilIn(i, "d.querySelectorAll('.opened .post').length === 2")
	if got := h.text(i, ".opened .new-messages"); got != "New messages" {
		t.Errorf("the first unread message is marked %q", got)
	}
	var call map[string]any
	h.in(i, "return f.calls[0];", &call)
	arguments, _ := call["arguments"].(map[string]any)
	if call["name"] != "view_action" || arguments["action"] != "unread" || arguments["channel_id"] != "c1" {
		t.Errorf("the view opened the channel with %v", call)
	}
	// Pressed again, it closes.
	h.click(i, ".unread-name")
	h.untilIn(i, "!d.querySelector('.opened')")

	// A followed thread opens as the thread view draws one.
	h.click(i, ".unread-section[aria-label=\"Followed threads\"] .unread-name")
	h.untilIn(i, "d.querySelector('.opened .thread')")
	h.in(i, "return f.calls[1];", &call)
	arguments, _ = call["arguments"].(map[string]any)
	if arguments["action"] != "thread" || arguments["post_id"] != "p1" {
		t.Errorf("the view opened the thread with %v", call)
	}

	// Where the host refuses the call, the view says so and offers
	// Mattermost.
	refused := h.mount(showUnreads(map[string]any{"tools": map[string]any{"view_action": []any{map[string]any{"error": "Invalid params"}}}}))
	h.click(refused, ".unread-name")
	h.untilIn(refused, "d.querySelector('.notice')")
	if got := h.text(refused, ".notice"); !strings.Contains(got, "does not open it here") || !strings.Contains(got, "Open in Mattermost") {
		t.Errorf("a refused call says %q", got)
	}
}

func TestMarkReadIsOfferedOnlyWhereTheServerLetsAViewMarkRead(t *testing.T) {
	h := openHost(t)
	i := h.mount(showUnreads(map[string]any{"tools": map[string]any{"view_change": []any{
		map[string]any{"structuredContent": map[string]any{"channel_id": "c1", "read": true}, "content": []any{}},
	}}}))
	h.click(i, ".mark-read")
	h.untilIn(i, "d.querySelector('.unread-row.marked')")
	if got := h.text(i, ".unread-row.marked .unread-count"); got != "Marked read" {
		t.Errorf("a channel marked read says %q", got)
	}
	var call map[string]any
	h.in(i, "return f.calls[0];", &call)
	arguments, _ := call["arguments"].(map[string]any)
	if call["name"] != "view_change" || arguments["action"] != "mark_read" || arguments["channel_id"] != "c1" {
		t.Errorf("the view marked read with %v", call)
	}

	refused := h.mount(showUnreads(map[string]any{"tools": map[string]any{"view_change": []any{map[string]any{"error": "Invalid params"}}}}))
	h.click(refused, ".mark-read")
	h.untilIn(refused, "d.querySelector('.notice')")
	if got := h.text(refused, ".notice"); !strings.Contains(got, "does not mark it read") {
		t.Errorf("a refused Mark read says %q", got)
	}

	// A server that allows no writes offers no tool to mark read through.
	readOnly := h.mount(showUnreads(map[string]any{"readOnly": true}))
	var offered int
	h.in(readOnly, "return d.querySelectorAll('.mark-read').length;", &offered)
	if offered != 0 {
		t.Errorf("a read-only server's view offers Mark read %d times", offered)
	}
}
