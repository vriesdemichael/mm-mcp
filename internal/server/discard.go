package server

import (
	"fmt"
	"unicode/utf8"
)

// What a call takes away, stated in the call and checked against mm-mcp's own
// count, before anyone is asked and again just before it acts (ADR-031): the
// characters of text an edit or a draft's deletion discards, and the posts a
// deletion removes. Counted again as the call acts, the number also binds it
// to what the model read: a draft typed on, a post edited in Mattermost or a
// reply that arrived since changes it, and the call is refused.

// discardField is the argument of a tool that replaces or deletes text.
type discardField struct {
	Discards *int64 `json:"discards,omitempty" jsonschema:"how many characters of text the call replaces or deletes, counted as Mattermost counts a message's length: the post's text as it reads now for an edit, the draft's for a draft. Required; a missing or wrong count is refused with the real one"`
}

// deleteField is the argument of a tool that deletes posts.
type deleteField struct {
	Deletes *int64 `json:"deletes,omitempty" jsonschema:"how many posts the call deletes, the post itself included: 1 for a reply, and the post with every reply in its thread, other people's included, for a post that starts one. Required; a missing or wrong count is refused with the real one"`
}

// discardCheck and deleteCheck say what discards and deletes are checked
// against, for the specs that take them.
const (
	discardCheck = "the length in characters of the text the call replaces or deletes, read just before it acts"
	deleteCheck  = "the post and the replies in its thread, read just before it acts"
)

// checkDiscards refuses a call whose discards is not the length of the text it
// replaces or deletes. what names the text, such as "your post in ~Town
// Square"; files is how many files a draft holds besides, which go with it.
func checkDiscards(tool, what, text string, files int, stated *int64) error {
	count := int64(utf8.RuneCountInString(text))
	if stated != nil && *stated == count {
		return nil
	}
	held := ""
	if files > 0 {
		held = fmt.Sprintf(", and it holds %d %s besides", files, plural(int64(files), "file", "files"))
	}
	return fmt.Errorf("%s did not run: it discards the %d characters of %s, which begins “%s”%s, and the call %s. "+
		"Consider whether the person meant to lose those words. If they did, call again with discards: %d. If not, "+
		"leave them as they are, or ask the person", tool, count, what, excerpt(text), held, said(stated), count)
}

// checkDeletes refuses a delete_post whose deletes is not the post and the
// replies that go with it.
func checkDeletes(what string, replies int64, stated *int64) error {
	count := 1 + replies
	if stated != nil && *stated == count {
		return nil
	}
	thread := ""
	if replies > 0 {
		thread = fmt.Sprintf(" and the %d %s in its thread, other people's included", replies, plural(replies, "reply", "replies"))
	}
	return fmt.Errorf("delete_post did not run: it deletes %d %s, %s%s, and the call %s. "+
		"Consider whether the person meant to delete all of them. If they did, call again with deletes: %d. If not, "+
		"leave it, or ask the person", count, plural(count, "post", "posts"), what, thread, said(stated), count)
}
