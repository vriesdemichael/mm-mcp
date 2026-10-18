package server

import (
	"strings"
	"testing"
)

// What Mattermost's web app asks about before sending, decided from what it
// reads (use_submit.tsx, use_groups.tsx), and what a call that states it
// wrong is told. Reading those facts from Mattermost is the live suite's.

func TestLoudnessIsWhatTheWebAppAsksAbout(t *testing.T) {
	t.Parallel()
	group := func(word string, members int64, allowed bool) groupMention {
		return groupMention{word: word, name: strings.TrimPrefix(word, "@"), members: members, allowed: allowed}
	}
	for _, c := range []struct {
		name  string
		facts loudFacts
		want  int64
	}{
		{"nothing is asked while the server does not confirm", loudFacts{members: 50, specials: []string{"@channel"}, mayMentionChannel: true}, 0},
		{"@channel alerts every member but the user", loudFacts{confirming: true, members: 6, specials: []string{"@channel"}, mayMentionChannel: true}, 5},
		{"not in a channel of 5", loudFacts{confirming: true, members: 5, specials: []string{"@here"}, mayMentionChannel: true}, 0},
		{"not by a user who may not mention the channel", loudFacts{confirming: true, members: 40, specials: []string{"@all"}}, 0},
		{"a group's members in the channel", loudFacts{confirming: true, members: 40, groups: []groupMention{group("@devs", 7, true)}}, 7},
		{"the largest group", loudFacts{confirming: true, members: 40, groups: []groupMention{group("@devs", 6, true), group("@ops", 9, true)}}, 9},
		{"not a group of 5 in the channel", loudFacts{confirming: true, members: 40, groups: []groupMention{group("@devs", 5, true)}}, 0},
		{"not a group the user may not mention", loudFacts{confirming: true, members: 40, groups: []groupMention{group("@devs", 30, false)}}, 0},
		{"groups count only without @channel", loudFacts{confirming: true, members: 40, specials: []string{"@channel"}, groups: []groupMention{group("@devs", 30, true)}}, 0},
	} {
		if got := loudnessOf(c.facts); got.count != c.want {
			t.Errorf("%s: %d; want %d (%s)", c.name, got.count, c.want, got.why)
		}
	}
}

func TestAWrongReachIsRefusedWithTheRealCountAndAskedToBeWeighed(t *testing.T) {
	t.Parallel()
	twelve, ten := int64(12), int64(10)
	if err := checkReaches("create_post", "~Town Square", &twelve, 12); err != nil {
		t.Fatalf("the right count: %v", err)
	}
	for stated, says := range map[*int64]string{nil: "left the number out", &ten: "said 10"} {
		err := checkReaches("create_post", "~Town Square", stated, 12)
		if err == nil {
			t.Fatalf("%s: not refused", says)
		}
		for _, want := range []string{says, "12 people", "~Town Square", "Consider whether", "reaches: 12"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q lacks %q", says, err, want)
			}
		}
	}
}

func TestAWrongNotifiesIsRefusedAndOneWhereNothingIsAskedToo(t *testing.T) {
	t.Parallel()
	zero, five, nine := int64(0), int64(5), int64(9)
	quiet, loud := loudness{}, loudness{count: 9, why: "@channel alerts 9 people"}
	for _, c := range []struct {
		name   string
		stated *int64
		loud   loudness
		says   string
	}{
		{"nothing asked, nothing stated", nil, quiet, ""},
		{"nothing asked, none stated", &zero, quiet, ""},
		{"the right number", &nine, loud, ""},
		{"a number where nothing is asked", &five, quiet, "Leave notifies out"},
		{"no number where Mattermost asks", nil, loud, "notifies: 9"},
		{"the wrong number", &five, loud, "said 5"},
	} {
		err := checkNotifies("create_post", c.stated, c.loud)
		switch {
		case c.says == "" && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case c.says != "" && (err == nil || !strings.Contains(err.Error(), c.says)):
			t.Errorf("%s: %v; want it refused, saying %q", c.name, err, c.says)
		}
	}
	if err := checkNotifies("create_post", nil, loud); !strings.Contains(err.Error(), "@channel alerts 9 people") || !strings.Contains(err.Error(), "Consider whether") {
		t.Errorf("the refusal does not say why, or ask for it to be weighed: %v", err)
	}
}
