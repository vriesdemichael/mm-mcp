package server

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// What a call takes away, stated in the call and checked against mm-mcp's own
// count, before anyone is asked and again just before it acts (ADR-031): the
// characters a draft's deletion discards, and the posts a deletion removes.
// Counted again as the call acts, the number also binds the call to what the
// model read: a draft typed on, or a reply that arrived since, changes it, and
// the call is refused.

// discardField is the argument of delete_draft.
type discardField struct {
	Discards *int64 `json:"discards,omitempty" jsonschema:"how many characters the draft holds, as list_drafts gives its length. Required; a missing or wrong count is refused with the real one"`
}

// deleteField is the argument of a tool that deletes posts.
type deleteField struct {
	Deletes *int64 `json:"deletes,omitempty" jsonschema:"how many posts the call deletes, the post itself included: 1 for a reply, and the post with every reply in its thread, other people's included, for a post that starts one. Required; a missing or wrong count is refused with the real one"`
}

// discardCheck and deleteCheck say what discards and deletes are checked
// against, for the specs that take them.
const (
	discardCheck = "the length in characters of the draft it deletes, read just before it acts"
	deleteCheck  = "the post and the replies in its thread, read just before it acts"
)

// checkDiscards refuses a delete_draft whose discards is not the length of the
// draft. what names the draft, such as "the unsent draft in ~Town Square";
// files is how many files it holds besides, which go with it.
func checkDiscards(what, text string, files int, stated *int64) error {
	count := int64(utf8.RuneCountInString(text))
	if stated != nil && *stated == count {
		return nil
	}
	held := ""
	if files > 0 {
		held = fmt.Sprintf(", and it holds %d %s besides", files, plural(int64(files), "file", "files"))
	}
	return fmt.Errorf("delete_draft did not run: it discards the %d characters of %s, which begins “%s”%s, and the call %s. "+
		"Consider whether the person meant to lose those words. If they did, call again with discards: %d. If not, "+
		"leave them as they are, or ask the person", count, what, excerpt(text), held, said(stated), count)
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

// What an edit states of what it changes (ADR-031): the post's whole text as
// it reads now, which binds the edit to it and puts what it replaces beside
// what replaces it, and, for a post others have answered, how many answer it,
// since their answers will read as answers to the new text.

// editField is what update_post states of the post it edits.
type editField struct {
	OldMessage string `json:"old_message,omitempty" jsonschema:"the post's whole text exactly as it reads now, which message replaces, so whoever approves the edit sees both. Required; a post that reads otherwise is refused with what it reads"`
	Responses  *int64 `json:"responses,omitempty" jsonschema:"how many reactions and, for a post that starts a thread, replies answer the post, as read_post gives it: Mattermost tells nobody about an edit, so they will read as answers to the new text. Required when the post has any; a missing or wrong count is refused with who answered"`
}

// oldMessageCheck and responsesCheck say what old_message and responses are
// checked against, for the spec that takes them.
const (
	oldMessageCheck = "the post's text as it reads now, read just before the edit"
	responsesCheck  = "the post's reactions and, when it starts a thread, its replies, read just before the edit"
)

// quotedWhole is the longest text a refusal quotes whole; a longer one it
// quotes the start of.
const quotedWhole = 2000

// checkOldMessage refuses an edit whose old_message is not the post's text as
// it reads now. what names the post, such as "your post in ~Town Square".
func checkOldMessage(what, current, stated string) error {
	if stated == current {
		return nil
	}
	shown := current
	if utf8.RuneCountInString(current) > quotedWhole {
		shown = excerpt(current) + "”, which read_post reads whole, “"
	}
	if stated == "" {
		return fmt.Errorf("update_post did not run: give old_message, the whole text of %s as it reads now, beside message, so whoever approves the edit sees what it replaces. It reads now: “%s”", what, shown)
	}
	return fmt.Errorf("update_post did not run: %s does not read as old_message says: it changed since it was read, or old_message is not all of it. It reads now: “%s”. Consider whether the edit still fits what it reads before calling again with that as old_message", what, shown)
}

// postAnswers is what answers a post: its reactions, by emoji, with who gave
// each, and its replies when it starts a thread.
type postAnswers struct {
	reactions []*model.Reaction
	replies   int64
	names     map[string]string
}

// answersTo is what answers a post, as Mattermost answered with it.
// GetUsersByIds.
func answersTo(ctx context.Context, client *mattermost.Client, post *model.Post) (postAnswers, error) {
	a := postAnswers{reactions: reactionsOf(post), replies: repliesTo(post)}
	ids := make([]string, 0, len(a.reactions))
	for _, reaction := range a.reactions {
		ids = append(ids, reaction.UserId)
	}
	names, err := usernames(ctx, client, ids)
	a.names = names
	return a, err
}

// count is how many answer the post: what responses states.
func (a postAnswers) count() int64 { return int64(len(a.reactions)) + a.replies }

// String says who answered, for a refusal: each emoji with how many gave it
// and the first of them, then the replies.
func (a postAnswers) String() string {
	var order []string
	by := map[string][]string{}
	for _, reaction := range a.reactions {
		if _, seen := by[reaction.EmojiName]; !seen {
			order = append(order, reaction.EmojiName)
		}
		who := cmp.Or(a.names[reaction.UserId], reaction.UserId)
		by[reaction.EmojiName] = append(by[reaction.EmojiName], "@"+who)
	}
	var parts []string
	for _, emoji := range order {
		people := by[emoji]
		shown := people
		if len(shown) > 3 {
			shown = append(slices.Clone(people[:3]), fmt.Sprintf("%d more", len(people)-3))
		}
		parts = append(parts, fmt.Sprintf(":%s: ×%d (%s)", emoji, len(people), strings.Join(shown, ", ")))
	}
	if a.replies > 0 {
		parts = append(parts, fmt.Sprintf("%d %s in its thread", a.replies, plural(a.replies, "reply", "replies")))
	}
	return strings.Join(parts, ", ")
}

// checkResponses refuses an edit of a post others answered whose responses is
// not how many answer it, or that states answers a post nobody answered.
func checkResponses(what string, a postAnswers, stated *int64) error {
	count := a.count()
	switch {
	case count == 0 && (stated == nil || *stated == 0):
		return nil
	case count == 0:
		return fmt.Errorf("update_post did not run: the call says responses: %d, but nobody has answered %s. Leave responses out", *stated, what)
	case stated != nil && *stated == count:
		return nil
	}
	return fmt.Errorf("update_post did not run: %s %s %s as it reads now: %s, and the call %s. Mattermost tells nobody about an edit, "+
		"so they will read as answers to the new text. Consider whether they still fit it; if the change is news to those who answered, "+
		"reply in the thread instead. If they still fit, call again with responses: %d",
		peopleAnswer(count), plural(count, "answers", "answer"), what, a, said(stated), count)
}

// peopleAnswer counts the answers to a post as a refusal names them.
func peopleAnswer(count int64) string {
	return fmt.Sprintf("%d %s", count, plural(count, "reaction or reply", "reactions and replies"))
}

// reactionsOf is a post's reactions, as Mattermost answered with them.
func reactionsOf(post *model.Post) []*model.Reaction {
	if post.Metadata == nil {
		return nil
	}
	return post.Metadata.Reactions
}

// repliesTo is how many replies answer a post: its thread's, when it starts one.
func repliesTo(post *model.Post) int64 {
	if post.RootId != "" {
		return 0
	}
	return post.ReplyCount
}
