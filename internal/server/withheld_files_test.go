package server

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post whose files an access policy withholds from the person says how many
// (mm-mcp#79).

func TestAPostSaysHowManyFilesAreWithheld(t *testing.T) {
	t.Parallel()
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	post := &model.Post{Id: "p1", UserId: reader.Id, Message: "the plan", Metadata: &model.PostMetadata{RedactedFileCount: 2}}
	if got := shownWith(post, display{}, reader); got.WithheldFiles != 2 || len(got.Files) != 0 {
		t.Errorf("a post with withheld files is described as %+v", got)
	}
}
