package server

import "testing"

// A place's address, made from its link (ADR-030).

func TestAnAddressIsThePlacesLinkUnderTheMattermostScheme(t *testing.T) {
	t.Parallel()
	for link, want := range map[string]string{
		"https://Chat.Example.com/eng/channels/town-square":      "mattermost://chat.example.com/eng/channels/town-square",
		"http://localhost:8065/eng/messages/@alice":              "mattermost://localhost:8065/eng/messages/@alice",
		"https://example.com/mattermost/eng/channels/release-qa": "mattermost://example.com/mattermost/eng/channels/release-qa",
		"https://chat.example.com/eng":                           "mattermost://chat.example.com/eng",
		"":                                                       "",
		"not a link":                                             "",
	} {
		if got := asAddress(link); got != want {
			t.Errorf("asAddress(%q) = %q; want %q", link, got, want)
		}
	}
	if got := postAddress("mattermost://chat.example.com/eng/channels/town-square", "p1"); got != "mattermost://chat.example.com/eng/channels/town-square/p1" {
		t.Errorf("a post's address: %q", got)
	}
	if postAddress("", "p1") != "" || postAddress("mattermost://chat.example.com/eng", "") != "" {
		t.Error("an address made of nothing")
	}
}

func TestAnAddressThatNamesItsChannelByIdIsRefused(t *testing.T) {
	t.Parallel()
	for _, l := range []link{
		{given: "a", kind: linkChannel, name: "abcdefghijklmnopqrstuvwxyz"},
		{given: "b", kind: linkChannel, name: "abcdefghijklmnopqrstuvwxyz__zyxwvutsrqponmlkjihgfedcba"},
		{given: "c", kind: linkMessages, name: "abcdefghijklmnopqrstuvwxyz"},
	} {
		if byName(l) == nil {
			t.Errorf("%+v was taken", l)
		}
	}
	for _, l := range []link{
		{given: "d", kind: linkChannel, name: "town-square"},
		{given: "e", kind: linkMessages, name: "@alice"},
		{given: "f", kind: linkMessages, name: "0123456789abcdef0123456789abcdef01234567"},
	} {
		if err := byName(l); err != nil {
			t.Errorf("%+v: %v", l, err)
		}
	}
}
