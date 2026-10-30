package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// Claude Code draws a view but passes on none of the view's reads of a
// resource, so a view there would show no pictures. It does hand a tool's
// result's _meta to the view and not to its model, which was measured with a
// megabyte of it, so a thread it shows carries the thread's pictures there. It
// keeps that _meta in its transcript on the person's machine, so what is
// carried is bounded. No other client gets them: the others read resources, and
// one that has not been measured might hand _meta to its model (ADR-022).

const (
	// claudeCode is the name Claude Code gives itself as an MCP client.
	claudeCode = "claude-code"
	// picturesMetaKey holds the pictures in a result's _meta: each one's
	// resource address, with its data address.
	picturesMetaKey = "mm-mcp/pictures"
	// picturesMost and pictureMost bound what a result carries, in bytes before
	// encoding: a picture over either is left to the view's fallback.
	picturesMost = 1 << 20
	pictureMost  = 256 << 10
	// picturesWanted bounds how many a result asks Mattermost for.
	picturesWanted = 60
)

// emojiInMessage is a name between colons, which may be a custom emoji; one
// of digits alone is more likely part of a time.
var emojiInMessage = regexp.MustCompile(`:([a-zA-Z0-9_+-]*[a-zA-Z_+-][a-zA-Z0-9_+-]*):`)

// carriesPictures says whether the client gets a thread's pictures with the
// result that shows it.
func carriesPictures(request *mcp.CallToolRequest) bool {
	info := request.ClientInfo()
	return info != nil && info.Name == claudeCode && rendersViews(request)
}

// wantedPicture is a picture a view would read, and how to fetch it.
type wantedPicture struct {
	uri   string
	fetch func(context.Context) ([]byte, error)
}

// threadPictures is the wanted pictures of a thread, the small ones first, so
// that a thread with large images still shows its people: each author's
// picture, unless Mattermost shows an integration's icon there, each custom
// emoji, and each attached image's preview.
func threadPictures(client *mattermost.Client, thread PostWithThread) []wantedPicture {
	server, err := url.Parse(client.Address())
	if err != nil {
		return nil
	}
	posts := thread.Thread
	if len(posts) == 0 && thread.Post != nil {
		posts = []Post{*thread.Post}
	}
	seen := map[string]bool{}
	var people, emoji, previews []wantedPicture
	add := func(list *[]wantedPicture, uri string, fetch func(context.Context) ([]byte, error)) {
		if !seen[uri] {
			seen[uri] = true
			*list = append(*list, wantedPicture{uri: uri, fetch: fetch})
		}
	}
	addEmoji := func(name string) {
		if _, comes := model.SystemEmojis[strings.ToLower(name)]; comes || !customEmojiName.MatchString(name) {
			return
		}
		add(&emoji, emojiResource(server, name), func(ctx context.Context) ([]byte, error) {
			found, err := client.Emoji(ctx, name)
			if err != nil {
				return nil, err
			}
			return client.EmojiImage(ctx, found.Id)
		})
	}
	for _, post := range posts {
		if id := post.AuthorID; !post.IntegrationIcon && mattermostID.MatchString(id) {
			add(&people, pictureResource(server, id), func(ctx context.Context) ([]byte, error) {
				return client.ProfileImage(ctx, id)
			})
		}
		for name := range post.Reactions {
			addEmoji(name)
		}
		for _, match := range emojiInMessage.FindAllStringSubmatch(post.Message, -1) {
			addEmoji(match[1])
		}
		for _, file := range post.Files {
			if id := file.ID; file.PreviewURI != "" {
				add(&previews, file.PreviewURI, func(ctx context.Context) ([]byte, error) {
					return client.FilePreview(ctx, id)
				})
			}
		}
	}
	wanted := append(append(people, emoji...), previews...)
	if len(wanted) > picturesWanted {
		wanted = wanted[:picturesWanted]
	}
	return wanted
}

// carriedPictures fetches what is wanted, in order, within the bounds, each as
// a data address by its resource address. What cannot be fetched, is too large
// or is no picture is left out: the view then draws what it draws without it.
func carriedPictures(ctx context.Context, wanted []wantedPicture) map[string]string {
	out := map[string]string{}
	left := picturesMost
	for _, picture := range wanted {
		if ctx.Err() != nil {
			break
		}
		data, err := picture.fetch(ctx)
		if err != nil || len(data) > pictureMost || len(data) > left {
			continue
		}
		mimeType := http.DetectContentType(data)
		if !rasterTypes[mimeType] {
			continue
		}
		left -= len(data)
		out[picture.uri] = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return out
}

// carriedPicturesUses are the operations threadPictures' fetches call, each
// with what fetching one of them sets.
func carriedPicturesUses() []Use {
	why := Fixed("each person, emoji or attached image in the thread", "a thread shown in Claude Code carries its pictures, since that client passes on no read of a resource")
	return []Use{
		{Operation: "GetProfileImage", Params: map[string]Coverage{
			"user_id": why,
			"_":       Omitted("it only stops a browser's cache from answering, and mm-mcp keeps no browser cache"),
		}},
		{Operation: "GetEmojiByName", Params: map[string]Coverage{"emoji_name": why}},
		{Operation: "GetEmojiImage", Params: map[string]Coverage{"emoji_id": why}},
		{Operation: "GetFilePreview", Params: map[string]Coverage{"file_id": why}},
	}
}
