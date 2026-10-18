package server

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// What a call that states wrongly how much it takes away is told.

func TestAWrongDiscardsIsRefusedWithTheLengthAndTheStartOfTheText(t *testing.T) {
	t.Parallel()
	text := "Café at nine, then the review"
	right, wrong := int64(29), int64(28)
	if err := checkDiscards("the unsent draft in ~Town Square", text, 0, &right); err != nil {
		t.Fatalf("the right length, in characters rather than bytes: %v", err)
	}
	for stated, says := range map[*int64]string{nil: "left the number out", &wrong: "said 28"} {
		err := checkDiscards("the unsent draft in ~Town Square", text, 2, stated)
		if err == nil {
			t.Fatalf("%s: not refused", says)
		}
		for _, want := range []string{says, "29 characters", "the unsent draft in ~Town Square", "“Café at nine", "2 files", "Consider whether", "discards: 29"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q lacks %q", says, err, want)
			}
		}
	}
}

func TestAWrongDeletesIsRefusedWithThePostsThatGo(t *testing.T) {
	t.Parallel()
	one, three := int64(1), int64(3)
	if err := checkDeletes("your post in ~Town Square", 0, &one); err != nil {
		t.Fatalf("a reply alone: %v", err)
	}
	if err := checkDeletes("your post in ~Town Square", 2, &three); err != nil {
		t.Fatalf("a post and its two replies: %v", err)
	}
	err := checkDeletes("your post in ~Town Square", 2, &one)
	if err == nil {
		t.Fatal("a thread stated as one post was not refused")
	}
	for _, want := range []string{"3 posts", "the 2 replies in its thread, other people's included", "said 1", "Consider whether", "deletes: 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q lacks %q", err, want)
		}
	}
	if err := checkDeletes("your post in ~Town Square", 0, nil); err == nil || !strings.Contains(err.Error(), "1 post,") {
		t.Errorf("a reply stated as nothing: %v", err)
	}
}

func TestAnEditStatesTheWholeTextItReplaces(t *testing.T) {
	t.Parallel()
	current := "Give me a thumbs up if you like cat memes"
	if err := checkOldMessage("your post in ~Town Square", current, current); err != nil {
		t.Fatalf("the text as it reads: %v", err)
	}
	for stated, says := range map[string]string{"": "give old_message", "Give me a thumbs up": "differs from the current text"} {
		err := checkOldMessage("your post in ~Town Square", current, stated)
		if err == nil || !strings.Contains(err.Error(), says) || !strings.Contains(err.Error(), "“"+current+"”") {
			t.Errorf("old_message %q: %v; want it refused, saying %q and quoting the post", stated, err, says)
		}
	}
	long := strings.Repeat("word ", quotedWhole)
	if err := checkOldMessage("your post in ~Town Square", long, ""); err == nil || strings.Contains(err.Error(), long) || !strings.Contains(err.Error(), "read_post reads whole") {
		t.Errorf("a long post is quoted whole: %.200v", err)
	}
}

func TestAnEditOfAnAnsweredPostStatesHowManyAnswerIt(t *testing.T) {
	t.Parallel()
	answers := postAnswers{
		reactions: []*model.Reaction{
			{UserId: "u1", EmojiName: "thumbsup"}, {UserId: "u2", EmojiName: "thumbsup"}, {UserId: "u3", EmojiName: "thumbsup"},
			{UserId: "u4", EmojiName: "thumbsup"}, {UserId: "u5", EmojiName: "thumbsup"}, {UserId: "u2", EmojiName: "smile"},
		},
		replies: 2,
		names:   map[string]string{"u1": "brent", "u2": "ann", "u3": "joe"},
	}
	if got, want := answers.String(), ":thumbsup: ×5 (@brent, @ann, @joe, 2 more), :smile: ×1 (@ann), 2 replies in its thread"; got != want {
		t.Errorf("who answered: %q; want %q", got, want)
	}
	eight, two, zero := int64(8), int64(2), int64(0)
	if err := checkResponses("your post in ~Town Square", answers, &eight); err != nil {
		t.Fatalf("the right count: %v", err)
	}
	for stated, says := range map[*int64]string{nil: "left the number out", &two: "said 2"} {
		err := checkResponses("your post in ~Town Square", answers, stated)
		if err == nil {
			t.Fatalf("%s: not refused", says)
		}
		for _, want := range []string{says, "8 reactions and replies respond to your post in ~Town Square", "@brent", "leaves them under the replacing text", "reply in the thread instead", "responses: 8"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %q lacks %q", says, err, want)
			}
		}
	}
	nobody := postAnswers{}
	if checkResponses("your post", nobody, nil) != nil || checkResponses("your post", nobody, &zero) != nil {
		t.Error("an edit of a post nobody answered was refused")
	}
	if err := checkResponses("your post", nobody, &two); err == nil || !strings.Contains(err.Error(), "Leave responses out") {
		t.Errorf("responses on a post nobody answered: %v", err)
	}
	one := postAnswers{replies: 1}
	if err := checkResponses("your post", one, nil); err == nil || !strings.Contains(err.Error(), "1 reaction or reply responds to") {
		t.Errorf("one answer: %v", err)
	}
}

func TestWhatAnswersAPostIsItsReactionsAndTheRepliesItStarted(t *testing.T) {
	t.Parallel()
	reactions := []*model.Reaction{{UserId: "u1", EmojiName: "eyes"}}
	root := &model.Post{ReplyCount: 3, Metadata: &model.PostMetadata{Reactions: reactions}}
	reply := &model.Post{RootId: "r1", ReplyCount: 3}
	if len(reactionsOf(root)) != 1 || repliesTo(root) != 3 || reactionsOf(reply) != nil || repliesTo(reply) != 0 {
		t.Error("a post's answers were read wrong")
	}
}
