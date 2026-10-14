package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// The pictures Mattermost shows beside a post are resources too (ADR-029): an
// attached image's preview, the smaller image its post shows; a person's
// profile picture; and a custom emoji. Each is content with an address of its
// own, fetched with mm-mcp's credential, which the client does not have, and
// each is a picture: a raster image, never SVG, which a client could run, and
// never anything else Mattermost answers with. A picture read once is kept a
// while for the next read of it with the same credential, since a thread
// shows its people again and again.

// pictureArgument and emojiArgument are the templates' arguments, named as
// tools name them.
const (
	pictureArgument = "user_id"
	emojiArgument   = "emoji_name"
)

// customEmojiName is a custom emoji's name as Mattermost allows one.
var customEmojiName = regexp.MustCompile(`^[a-zA-Z0-9_+-]{1,64}$`)

// rasterTypes are the picture types a picture resource passes on.
var rasterTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// resourceBase is the start of every resource address of the server at server.
func resourceBase(server *url.URL) string {
	return fileResourceScheme + "://" + strings.ToLower(server.Host) + strings.TrimRight(server.Path, "/")
}

// resourcePart is what stands between prefix and suffix in a resource address
// of the server at server, when the address is one of its and of that shape.
func resourcePart(server, address *url.URL, prefix, suffix string) (string, bool) {
	if !strings.EqualFold(address.Scheme, fileResourceScheme) || !strings.EqualFold(address.Host, server.Host) {
		return "", false
	}
	rest, ok := strings.CutPrefix(address.Path, strings.TrimRight(server.Path, "/")+prefix)
	if !ok {
		return "", false
	}
	return strings.CutSuffix(rest, suffix)
}

// previewResource is the address of a file's preview.
func previewResource(server *url.URL, id string) string {
	return fileResource(server, id) + "/preview"
}

// previewURI is a file's preview's address on the client's server.
func previewURI(client *mattermost.Client, id string) string {
	if file := fileURI(client, id); file != "" {
		return file + "/preview"
	}
	return ""
}

// pictureResource is the address of a user's profile picture.
func pictureResource(server *url.URL, userID string) string {
	return resourceBase(server) + "/users/" + userID + "/image"
}

// emojiResource is the address of a custom emoji's picture.
func emojiResource(server *url.URL, name string) string {
	return resourceBase(server) + "/emoji/" + name
}

func previewResourceSpec() ResourceSpec {
	return ResourceSpec{
		Name: "preview",
		Template: func(cfg config.Config) *mcp.ResourceTemplate {
			return &mcp.ResourceTemplate{
				Name:        "preview",
				Title:       "Mattermost image preview",
				URITemplate: previewResource(serverAddress(cfg), "{"+fileArgument+"}"),
				Description: "The smaller image Mattermost makes of an attached image and shows in its post, as a picture. A file's preview_uri " +
					"names it, for an image that has one; the file's own address reads the whole image.",
			}
		},
		Read: pictureReader(func(server, address *url.URL) (string, bool) {
			id, ok := resourcePart(server, address, "/files/", "/preview")
			return id, ok && mattermostID.MatchString(id)
		}, func(ctx context.Context, client *mattermost.Client, id string) ([]byte, error) {
			return client.FilePreview(ctx, id)
		}),
		Uses: []Use{{Operation: "GetFilePreview", Params: map[string]Coverage{"file_id": SetBy(fileArgument)}}},
	}
}

func pictureResourceSpec() ResourceSpec {
	return ResourceSpec{
		Name: "picture",
		Template: func(cfg config.Config) *mcp.ResourceTemplate {
			return &mcp.ResourceTemplate{
				Name:        "picture",
				Title:       "Mattermost profile picture",
				URITemplate: pictureResource(serverAddress(cfg), "{"+pictureArgument+"}"),
				Description: "A person's profile picture, as Mattermost shows it beside their posts: the one they set, or the one Mattermost " +
					"draws from their initials. A post's author_id is the user_id.",
			}
		},
		Read: pictureReader(func(server, address *url.URL) (string, bool) {
			id, ok := resourcePart(server, address, "/users/", "/image")
			return id, ok && mattermostID.MatchString(id)
		}, func(ctx context.Context, client *mattermost.Client, id string) ([]byte, error) {
			return client.ProfileImage(ctx, id)
		}),
		Uses: []Use{{Operation: "GetProfileImage", Params: map[string]Coverage{
			"user_id": SetBy(pictureArgument),
			"_":       Omitted("it only stops a browser's cache from answering, and mm-mcp keeps no browser cache"),
		}}},
	}
}

func emojiResourceSpec() ResourceSpec {
	return ResourceSpec{
		Name: "emoji",
		Template: func(cfg config.Config) *mcp.ResourceTemplate {
			return &mcp.ResourceTemplate{
				Name:        "emoji",
				Title:       "Mattermost custom emoji",
				URITemplate: emojiResource(serverAddress(cfg), "{"+emojiArgument+"}"),
				Description: "A custom emoji's picture, by the name a reaction or a message uses for it, without the colons. Only an emoji " +
					"someone added to this Mattermost has one; one that comes with Mattermost is a Unicode character.",
			}
		},
		Read: pictureReader(func(server, address *url.URL) (string, bool) {
			name, ok := resourcePart(server, address, "/emoji/", "")
			return name, ok && customEmojiName.MatchString(name)
		}, func(ctx context.Context, client *mattermost.Client, name string) ([]byte, error) {
			emoji, err := client.Emoji(ctx, name)
			if err != nil {
				return nil, err
			}
			return client.EmojiImage(ctx, emoji.Id)
		}),
		Uses: []Use{
			{Operation: "GetEmojiByName", Params: map[string]Coverage{"emoji_name": SetBy(emojiArgument)}},
			{Operation: "GetEmojiImage", Params: map[string]Coverage{
				"emoji_id": Fixed("the id of the emoji the name names", "Mattermost serves an emoji's picture by its id, which it answers the name with"),
			}},
		},
	}
}

// pictureReader reads a picture resource: the address names what to fetch,
// and fetch fetches it. What is not a picture of this server is not found,
// and nothing is asked of Mattermost for it.
func pictureReader(
	named func(server, address *url.URL) (string, bool),
	fetch func(ctx context.Context, client *mattermost.Client, what string) ([]byte, error),
) func(clientFor ClientFor, cfg config.Config) mcp.ResourceHandler {
	return func(clientFor ClientFor, cfg config.Config) mcp.ResourceHandler {
		server := serverAddress(cfg)
		kept := newPictures(picturesKept, pictureTime)
		return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			uri := request.Params.URI
			given, err := url.Parse(uri)
			if err != nil {
				return nil, mcp.ResourceNotFoundError(uri)
			}
			what, ok := named(server, given)
			if !ok {
				return nil, mcp.ResourceNotFoundError(uri)
			}
			client, err := clientFor(ctx, request)
			if err != nil {
				return nil, err
			}
			contents, ok := kept.get(client, uri)
			if !ok {
				data, err := fetch(ctx, client, what)
				if notFound(err) || badRequest(err) {
					return nil, mcp.ResourceNotFoundError(uri)
				}
				if err != nil {
					return nil, err
				}
				contents, err = picture(uri, data, cfg)
				if err != nil {
					return nil, err
				}
				kept.put(client, uri, contents)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
		}
	}
}

// badRequest says Mattermost refused the request as made, as it refuses the
// preview of a file that has none.
func badRequest(err error) bool {
	var answered *mattermost.Error
	return errors.As(err, &answered) && answered.Status == http.StatusBadRequest
}

// picture is a picture's bytes as a resource's contents, when they are a
// raster image within the server's cap on a resource.
func picture(uri string, data []byte, cfg config.Config) (*mcp.ResourceContents, error) {
	mimeType := http.DetectContentType(data)
	if !rasterTypes[mimeType] {
		return nil, fmt.Errorf("what Mattermost answered %s with is %s, not a picture mm-mcp passes on: only PNG, JPEG, GIF and WebP", uri, mimeType)
	}
	contents := &mcp.ResourceContents{URI: uri, MIMEType: mimeType, Blob: data}
	if most := cfg.ResourceBytes(); most > 0 {
		if sent := messageSize(contents); sent > most {
			return nil, fmt.Errorf("the picture at %s is %s as one message, more than the %s a resource is sent in (%s)",
				uri, sizeOf(sent), sizeOf(most), config.EnvResourceMaxMB)
		}
	}
	return contents, nil
}

// How much of the pictures read a server keeps, and for how long: a thread
// names the same people again and again, and a picture changes seldom.
const (
	picturesKept = 32 << 20
	pictureTime  = 10 * time.Minute
)

// pictures keeps the pictures read lately, up to most bytes of them, for a
// later read with the same client: what one credential may see, another may
// not.
type pictures struct {
	mu      sync.Mutex
	most    int
	held    int
	keep    time.Duration
	entries map[pictureKey]pictureEntry
	order   []pictureKey
}

type pictureKey struct {
	client *mattermost.Client
	uri    string
}

type pictureEntry struct {
	contents *mcp.ResourceContents
	until    time.Time
}

func newPictures(most int, keep time.Duration) *pictures {
	return &pictures{most: most, keep: keep, entries: map[pictureKey]pictureEntry{}}
}

func (p *pictures) get(client *mattermost.Client, uri string) (*mcp.ResourceContents, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[pictureKey{client, uri}]
	if !ok || time.Now().After(entry.until) {
		return nil, false
	}
	return entry.contents, true
}

func (p *pictures) put(client *mattermost.Client, uri string, contents *mcp.ResourceContents) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := pictureKey{client, uri}
	if old, ok := p.entries[key]; ok {
		p.held -= len(old.contents.Blob)
	} else {
		p.order = append(p.order, key)
	}
	p.entries[key] = pictureEntry{contents: contents, until: time.Now().Add(p.keep)}
	p.held += len(contents.Blob)
	for p.held > p.most && len(p.order) > 0 {
		p.held -= len(p.entries[p.order[0]].contents.Blob)
		delete(p.entries, p.order[0])
		p.order = p.order[1:]
	}
}
