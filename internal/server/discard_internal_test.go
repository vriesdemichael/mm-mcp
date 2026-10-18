package server

import (
	"strings"
	"testing"
)

// What a call that states wrongly how much it takes away is told.

func TestAWrongDiscardsIsRefusedWithTheLengthAndTheStartOfTheText(t *testing.T) {
	t.Parallel()
	text := "Café at nine, then the review"
	right, wrong := int64(29), int64(28)
	if err := checkDiscards("update_post", "your post in ~Town Square", text, 0, &right); err != nil {
		t.Fatalf("the right length, in characters rather than bytes: %v", err)
	}
	for stated, says := range map[*int64]string{nil: "left the number out", &wrong: "said 28"} {
		err := checkDiscards("delete_draft", "the unsent draft in ~Town Square", text, 2, stated)
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
	if err := checkDeletes("delete_post", "your post in ~Town Square", 0, &one); err != nil {
		t.Fatalf("a reply alone: %v", err)
	}
	if err := checkDeletes("delete_post", "your post in ~Town Square", 2, &three); err != nil {
		t.Fatalf("a post and its two replies: %v", err)
	}
	err := checkDeletes("delete_post", "your post in ~Town Square", 2, &one)
	if err == nil {
		t.Fatal("a thread stated as one post was not refused")
	}
	for _, want := range []string{"3 posts", "the 2 replies in its thread, other people's included", "said 1", "Consider whether", "deletes: 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q lacks %q", err, want)
		}
	}
	if err := checkDeletes("delete_post", "your post in ~Town Square", 0, nil); err == nil || !strings.Contains(err.Error(), "1 post,") {
		t.Errorf("a reply stated as nothing: %v", err)
	}
}
