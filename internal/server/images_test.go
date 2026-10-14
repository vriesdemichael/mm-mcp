package server

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func TestOnlyARasterPictureWithinTheCapIsPassedOn(t *testing.T) {
	t.Parallel()
	cfg := config.Config{URL: "https://chat.example.com", Token: "t"}
	png := append(append([]byte{}, pngSignature...), make([]byte, 64)...)
	if got, err := picture("mattermost://chat.example.com/users/x/image", png, cfg); err != nil || got.MIMEType != "image/png" || !bytes.Equal(got.Blob, png) {
		t.Errorf("a PNG: got %+v, %v", got, err)
	}
	for name, data := range map[string][]byte{
		"an SVG":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"HTML":      []byte(`<!doctype html><p>hello`),
		"some text": []byte("not a picture"),
	} {
		if got, err := picture("mattermost://chat.example.com/users/x/image", data, cfg); err == nil || !strings.Contains(err.Error(), "not a picture mm-mcp passes on") {
			t.Errorf("%s: got %+v, %v", name, got, err)
		}
	}
	large := append(append([]byte{}, pngSignature...), make([]byte, 1<<20)...)
	bounded := config.Config{URL: "https://chat.example.com", Token: "t", ResourceMaxMB: 1}
	if _, err := picture("mattermost://chat.example.com/files/x/preview", large, bounded); err == nil || !strings.Contains(err.Error(), "MM_MCP_RESOURCE_MAX_MB") {
		t.Errorf("a picture past the cap: %v", err)
	}
}

func TestPicturesAreKeptForTheSameClientWithinTheirBytes(t *testing.T) {
	t.Parallel()
	one, other := &mattermost.Client{}, &mattermost.Client{}
	contents := func(size int) *mcp.ResourceContents { return &mcp.ResourceContents{Blob: make([]byte, size)} }
	kept := newPictures(100, time.Minute)
	kept.put(one, "a", contents(60))
	if _, ok := kept.get(other, "a"); ok {
		t.Error("a picture read with one credential was handed to another")
	}
	if _, ok := kept.get(one, "a"); !ok {
		t.Error("a picture read was not kept")
	}
	kept.put(one, "b", contents(60))
	if _, ok := kept.get(one, "a"); ok {
		t.Error("the oldest picture was kept past the bytes allowed")
	}
	if _, ok := kept.get(one, "b"); !ok {
		t.Error("the newest picture was not kept")
	}
	kept.put(one, "b", contents(10))
	kept.put(one, "c", contents(80))
	if _, ok := kept.get(one, "b"); !ok {
		t.Error("a picture read again was counted twice")
	}
	stale := newPictures(100, -time.Second)
	stale.put(one, "a", contents(1))
	if _, ok := stale.get(one, "a"); ok {
		t.Error("a picture was kept past its time")
	}
}

func TestAPicturesAddressNamesWhatItShows(t *testing.T) {
	t.Parallel()
	for _, server := range []string{"https://chat.example.com", "http://Localhost:8065/mm/"} {
		base, _ := url.Parse(strings.TrimRight(server, "/"))
		for _, c := range []struct {
			address, prefix, suffix, want string
		}{
			{previewResource(base, "abcdefghijklmnopqrstuvwxyz"), "/files/", "/preview", "abcdefghijklmnopqrstuvwxyz"},
			{pictureResource(base, "abcdefghijklmnopqrstuvwxyz"), "/users/", "/image", "abcdefghijklmnopqrstuvwxyz"},
			{emojiResource(base, "party_parrot"), "/emoji/", "", "party_parrot"},
		} {
			address, err := url.Parse(c.address)
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := resourcePart(base, address, c.prefix, c.suffix); !ok || got != c.want {
				t.Errorf("%s: read back %q, %t", c.address, got, ok)
			}
		}
	}
}
