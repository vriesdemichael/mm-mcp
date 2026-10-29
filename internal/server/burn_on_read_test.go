package server

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A burn-on-read post says what it is, and a concealed one says nothing of
// what it holds (mm-mcp#78).

// burnOnReadPosts is one burn-on-read post as Mattermost 11.11.1 answers each
// reader with it: a recipient who has not revealed it (setUnrevealedPost and
// PreparePostForClient empty its message, files and metadata, but not its
// props), one who has (revealPostForUser sets metadata.expire_at), and its
// author (revealPostForAuthor names who revealed it). Shaped from the server's
// code, since Team Edition, which the live stacks run, refuses such a post.
func burnOnReadPosts(t *testing.T) map[string]*model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/burn-on-read-posts.json")
	if err != nil {
		t.Fatal(err)
	}
	var posts map[string]*model.Post
	if err := json.Unmarshal(data, &posts); err != nil {
		t.Fatal(err)
	}
	return posts
}

func TestAConcealedPostSaysSoAndNothingOfWhatItHolds(t *testing.T) {
	t.Parallel()
	posts := burnOnReadPosts(t)
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	author := &model.User{Id: "y6d8b4ebgbfjjenyatq36utqgy", Username: "bob"}

	got := shownWith(posts["concealed"], display{priority: true}, reader, author)
	if !got.Concealed || got.Type != model.PostTypeBurnOnRead || got.Author != "bob" || got.ExpireAt != "2026-11-05T11:26:44Z" {
		t.Errorf("a concealed post is described as %+v", got)
	}
	// Mattermost leaves its props in the answer, and the cards in them are
	// what it keeps from the person until they reveal it.
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "" || got.MessageAttachments != nil || got.Files != nil || strings.Contains(string(data), "4512") || strings.Contains(string(data), "vault") {
		t.Errorf("a concealed post carries what it holds: %s", data)
	}

	// Revealed, it reads as any post, with when it burns for the person.
	got = shownWith(posts["revealed"], display{}, reader, author)
	if got.Concealed || got.Message == "" || len(got.MessageAttachments) != 1 || got.ExpireAt != "2026-10-29T11:37:44Z" {
		t.Errorf("a revealed post is described as %+v", got)
	}
	// Its author always reads it, with the latest it lasts until everyone
	// revealed it.
	got = shownWith(posts["author"], display{}, author, reader)
	if got.Concealed || got.Message == "" || got.ExpireAt != "2026-11-05T11:26:44Z" {
		t.Errorf("a burn-on-read post is described to its author as %+v", got)
	}
}

func TestOnlyABurnOnReadPostTheReaderDidNotRevealIsConcealed(t *testing.T) {
	t.Parallel()
	reader := &model.User{Id: "u1"}
	for _, c := range []struct {
		name   string
		post   *model.Post
		reader *model.User
		hidden bool
	}{
		{"a plain post", &model.Post{UserId: "u2"}, reader, false},
		{"another's, unrevealed", &model.Post{UserId: "u2", Type: model.PostTypeBurnOnRead}, reader, true},
		{"another's, revealed", &model.Post{UserId: "u2", Type: model.PostTypeBurnOnRead, Metadata: &model.PostMetadata{ExpireAt: 1}}, reader, false},
		{"one's own", &model.Post{UserId: "u1", Type: model.PostTypeBurnOnRead}, reader, false},
		{"read for no one", &model.Post{UserId: "u1", Type: model.PostTypeBurnOnRead}, nil, true},
	} {
		if got := concealed(c.post, c.reader); got != c.hidden {
			t.Errorf("%s: concealed %t", c.name, got)
		}
	}
	// The latest a post lasts comes as a number, or one written as text.
	for _, at := range []any{float64(1793878004936), "1793878004936", json.Number("1793878004936"), int64(1793878004936)} {
		post := &model.Post{Type: model.PostTypeBurnOnRead}
		post.AddProp(model.PostPropsExpireAt, at)
		if got := burnsAt(post); got != 1793878004936 {
			t.Errorf("expire_at %#v reads as %d", at, got)
		}
	}
	plain := &model.Post{}
	plain.AddProp(model.PostPropsExpireAt, float64(1))
	if got := burnsAt(plain); got != 0 {
		t.Errorf("a plain post burns at %d", got)
	}
}

// A question about a concealed post, such as whether to react to it, quotes
// none of what it holds: the cards Mattermost leaves in its props included.
func TestAQuestionDoesNotQuoteAConcealedPost(t *testing.T) {
	t.Parallel()
	posts := burnOnReadPosts(t)
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	author := &model.User{Id: "y6d8b4ebgbfjjenyatq36utqgy", Username: "bob"}
	if got := (display{}).quote(posts["concealed"], reader); got != "a burn-on-read message, not opened yet" {
		t.Errorf("a concealed post is quoted as %q", got)
	}
	if got := (display{}).quote(posts["revealed"], reader); !strings.HasPrefix(got, "“"+excerpt(posts["revealed"].Message)) {
		t.Errorf("a revealed post is quoted as %q", got)
	}
	if got := (display{}).quote(posts["author"], author); !strings.HasPrefix(got, "“"+excerpt(posts["author"].Message)) {
		t.Errorf("a burn-on-read post is quoted to its author as %q", got)
	}
}
