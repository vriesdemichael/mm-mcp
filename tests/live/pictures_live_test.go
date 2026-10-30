//go:build live

package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// The pictures a post shows are resources too: an attached image's preview, a
// person's profile picture and a custom emoji (ADR-029).

// liveResource is a resource address on the live instance.
func liveResource(path string) string {
	return "mattermost://" + strings.TrimPrefix(strings.TrimPrefix(liveURL, "http://"), "https://") + path
}

func TestPicturesAreReadAsResourcesAndAPostNamesItsImagesPreview(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	author := clientAs(t, other)
	_, err := author.SetProfileImage(t.Context(), other.Id, picture(t))
	check(t, err)
	me, _, err := admin.GetMe(t.Context(), "")
	check(t, err)
	emoji, _, err := admin.CreateEmoji(t.Context(), &model.Emoji{Name: uniqueName("emoji"), CreatorId: me.Id}, picture(t), "emoji.png")
	check(t, err)
	t.Cleanup(func() { _, _ = admin.DeleteEmoji(context.Background(), emoji.Id) })
	post, pictureID := attachAs(t, author, channel.Id, "chart.png", picture(t))
	_, textID := attachAs(t, author, channel.Id, "notes.txt", []byte("no picture here\n"))
	session := sessionFor(t, admin, user)

	// A post's image says its size and names its preview, which reads as a
	// picture; the whole image stays at the file's own address.
	var read server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": post.Id, "include_thread": false}}), &read)
	if read.Post == nil || len(read.Post.Files) != 1 {
		t.Fatalf("read_post answered %+v", read)
	}
	file := read.Post.Files[0]
	if file.Width != 40 || file.Height != 30 || file.PreviewURI != liveFile(pictureID)+"/preview" || file.URI != liveFile(pictureID) {
		t.Errorf("the image is described as %+v", file)
	}

	readPicture := func(template, uri string) *mcp.ResourceContents {
		t.Helper()
		result, err := readResource(t, session, template, uri)
		check(t, err)
		if len(result.Contents) != 1 || result.Contents[0].URI != uri {
			t.Fatalf("%s read as %+v", uri, result.Contents)
		}
		got := result.Contents[0]
		if !strings.HasPrefix(got.MIMEType, "image/") || got.MIMEType == "image/svg+xml" || len(got.Blob) == 0 {
			t.Errorf("%s reads %d bytes as %s", uri, len(got.Blob), got.MIMEType)
		}
		return got
	}
	readPicture("preview", file.PreviewURI)
	if got := readPicture("picture", liveResource("/users/"+other.Id+"/image")); !bytes.HasPrefix(got.Blob, []byte("\x89PNG")) {
		t.Errorf("the profile picture someone set reads as %s", got.MIMEType)
	}
	// Someone who set none has the one Mattermost draws.
	readPicture("picture", liveResource("/users/"+user.Id+"/image"))
	readPicture("emoji", liveResource("/emoji/"+emoji.Name))

	// A picture read again comes from what was kept, without asking Mattermost.
	rec := recorderOf(t, session)
	rec.take()
	if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: liveResource("/emoji/" + emoji.Name)}); err != nil {
		t.Fatal(err)
	}
	if asked := rec.take(); len(asked) != 0 {
		t.Errorf("reading the emoji again asked Mattermost %d times", len(asked))
	}

	// What Mattermost has no picture of is not found: a file that is no
	// image, someone who does not exist, an emoji that comes with Mattermost
	// or none at all.
	for template, uri := range map[string]string{
		"preview": liveFile(textID) + "/preview",
		"picture": liveResource("/users/" + model.NewId() + "/image"),
		"emoji":   liveResource("/emoji/" + uniqueName("none")),
	} {
		_, err := readResource(t, session, template, uri)
		var refused *jsonrpc.Error
		if !errors.As(err, &refused) || refused.Message != "Resource not found" {
			t.Errorf("%s: got %v; want resource not found", uri, err)
		}
	}
	if _, err := readResource(t, session, "emoji", liveResource("/emoji/tada")); err == nil {
		t.Error("an emoji that comes with Mattermost read as a picture")
	}
}

// Claude Code draws views but passes on no read of a resource, so a thread it
// shows carries its pictures, in _meta, which it keeps from its model. No other
// client gets them.
func TestAThreadCarriesItsPicturesToClaudeCodeAndNoOtherClient(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	author := clientAs(t, other)
	_, err := author.SetProfileImage(t.Context(), other.Id, picture(t))
	check(t, err)
	me, _, err := admin.GetMe(t.Context(), "")
	check(t, err)
	emoji, _, err := admin.CreateEmoji(t.Context(), &model.Emoji{Name: uniqueName("emoji"), CreatorId: me.Id}, picture(t), "emoji.png")
	check(t, err)
	t.Cleanup(func() { _, _ = admin.DeleteEmoji(context.Background(), emoji.Id) })
	post, pictureID := attachAs(t, author, channel.Id, "chart.png", picture(t))
	postAs(t, author, channel.Id, post.Id, "look :"+emoji.Name+": at 12:30:45")
	token := personalAccessToken(t, admin, user.Id).Token
	// A call's params are its own: the client writes its name in them.
	show := func() *mcp.CallToolParams {
		return &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": post.Id}}
	}

	// Mattermost makes a preview some time after the upload.
	session := viewsSessionNamed(t, "claude-code", token)
	wanted := []string{liveResource("/users/" + other.Id + "/image"), liveResource("/emoji/" + emoji.Name), liveFile(pictureID) + "/preview"}
	var result *mcp.CallToolResult
	eventually(t, 30*time.Second, func() (bool, string) {
		result = callTool(t, session, show())
		carried, _ := result.Meta["mm-mcp/pictures"].(map[string]any)
		for _, uri := range wanted {
			if picture, _ := carried[uri].(string); !strings.HasPrefix(picture, "data:image/png;base64,iVBOR") {
				return false, uri + " is not carried, of " + fmt.Sprint(len(carried))
			}
		}
		if len(carried) != len(wanted) {
			return false, fmt.Sprint(len(carried), " are carried where ", len(wanted), " were wanted")
		}
		return true, ""
	})
	// The rest of the answer, which a model may read, is what it always is.
	text := result.Content[0].(*mcp.TextContent).Text
	answer, err := json.Marshal(result.StructuredContent)
	check(t, err)
	if strings.Contains(text, "base64") || strings.Contains(string(answer), "base64") {
		t.Errorf("a picture's bytes reached what a model may read: %q", text)
	}

	if meta := callTool(t, viewsSession(t, token), show()).Meta; meta["mm-mcp/pictures"] != nil {
		t.Errorf("a client that reads resources was carried %d bytes of pictures", len(fmt.Sprint(meta["mm-mcp/pictures"])))
	}
}
