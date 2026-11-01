package views

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io/fs"
	"strings"
	"sync"
)

// The client check's page (ADR-022): what check_views shows, to measure what a
// client lets a view do. It is a page of its own, because a host binds a page
// to a tool, and because it declares what the page that draws Mattermost
// promises never to: the server's own address, to see whether the client
// honours what it approves. It draws nothing from Mattermost.

// CheckURI is the check page's address.
const CheckURI = "ui://mm-mcp/check"

// CheckScripts is every script of the check page, in the order they run.
var CheckScripts = []string{"bridge.js", "dom.js", "i18n.js", "check.js"}

// Fixture is a resource the check reads, of a type a client may treat apart.
// It holds Text, or Blob for what is not text.
type Fixture struct {
	Name     string
	URI      string
	MIMEType string
	Text     string
	Blob     []byte
}

// fixtureText is what the text fixture holds.
const fixtureText = "mm-mcp's client check reads this."

// fixturePNG is a one-pixel picture.
const fixturePNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// fixturePDF is the least a PDF holds: an empty page.
const fixturePDF = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 72 72]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n"

// CheckFixtures is what the check reads through the host: mm-mcp's own, so a
// read of one reaches nothing in Mattermost.
func CheckFixtures() []Fixture {
	// The constant is valid base64, so decoding cannot fail.
	picture, _ := base64.StdEncoding.DecodeString(fixturePNG)
	return []Fixture{
		{Name: "image", URI: CheckURI + "/image.png", MIMEType: "image/png", Blob: picture},
		{Name: "text", URI: CheckURI + "/text.txt", MIMEType: "text/plain", Text: fixtureText},
		{Name: "pdf", URI: CheckURI + "/empty.pdf", MIMEType: "application/pdf", Blob: []byte(fixturePDF)},
	}
}

// Stamp names what this build's pages were made from. A page compares its own
// with the one a result carries, since a client may keep a page for a session
// and go on drawing it after the server has been rebuilt.
var Stamp = sync.OnceValue(func() string {
	sum := sha256.New()
	// The tree is embedded, so walking it cannot fail.
	_ = fs.WalkDir(assets, "page", func(name string, entry fs.DirEntry, _ error) error {
		if !entry.IsDir() {
			sum.Write([]byte(name))
			sum.Write([]byte(mustAsset(strings.TrimPrefix(name, "page/"))))
		}
		return nil
	})
	return hex.EncodeToString(sum.Sum(nil))[:16]
})

// CheckPage is the check page for the Mattermost at server, as
// https://chat.example.com, whose resources start at base, in the build named
// release.
func CheckPage(base, server, release string) string {
	style := mustAsset("view.css") + mustAsset("check.css")
	catalogs, err := Catalogs()
	if err != nil {
		panic(err)
	}
	fixtures := map[string]map[string]any{}
	for _, fixture := range CheckFixtures() {
		fixtures[fixture.Name] = map[string]any{"uri": fixture.URI, "type": fixture.MIMEType, "size": len(fixture.Blob), "text": fixture.Text}
	}
	var script strings.Builder
	script.WriteString("(() => {\n\"use strict\";\n")
	script.WriteString("const RESOURCE_BASE = " + mustJSON(base) + ";\n")
	script.WriteString("const SERVER = " + mustJSON(server) + ";\n")
	script.WriteString("const RELEASE = " + mustJSON(release) + ";\n")
	script.WriteString("const STAMP = " + mustJSON(Stamp()) + ";\n")
	script.WriteString("const PAGE_URI = " + mustJSON(CheckURI) + ";\n")
	script.WriteString("const FIXTURES = " + mustJSON(fixtures) + ";\n")
	script.WriteString("const DOT = " + mustJSON("data:image/png;base64,"+fixturePNG) + ";\n")
	script.WriteString("const CATALOGS = " + mustJSON(catalogs) + ";\n")
	for _, name := range CheckScripts {
		script.WriteString("// ---- " + name + " ----\n")
		script.WriteString(mustAsset(name))
		script.WriteString("\n")
	}
	script.WriteString("})();\n")
	return strings.NewReplacer(stylePlaceholder, style, scriptPlaceholder, script.String()).Replace(mustAsset("page.html"))
}
