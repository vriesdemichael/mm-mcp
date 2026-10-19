// Package views is mm-mcp's MCP Apps view page (ADR-022): one page that draws
// every view, plain JavaScript and CSS embedded in the binary, with no build
// step and no third-party code. The page holds no Mattermost data: what a
// view draws arrives as the result of the call it belongs to, and the
// pictures as resources the host reads from mm-mcp.
//
// Page assembles it into one self-contained document: the style, then one
// script that runs every file in Scripts in one scope, in that order, after
// the data the page needs: where this Mattermost's resources start, what the
// server lets a view change, the emoji that come with Mattermost, from
// Mattermost's own table, and the catalogs of the page's own words.
package views

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
)

// The MCP Apps names mm-mcp speaks.
const (
	// Extension is the capability a client declares when it renders views.
	Extension = "io.modelcontextprotocol/ui"
	// MIMEType is the type of a view page.
	MIMEType = "text/html;profile=mcp-app"
	// URI is the page's address.
	URI = "ui://mm-mcp/view"
)

//go:embed page/page.html page/view.css page/*.js page/catalogs/*.json
var assets embed.FS

// Scripts is every script of the page, in the order they run; main.js, which
// starts the view, runs last.
var Scripts = []string{"bridge.js", "dom.js", "i18n.js", "format.js", "pictures.js", "markdown.js", "thread.js", "unreads.js", "proposal.js", "main.js"}

const (
	stylePlaceholder  = "/*MM_VIEW_STYLE*/"
	scriptPlaceholder = "/*MM_VIEW_SCRIPT*/"
)

// Asset is one of the page's files, by its name under page/.
func Asset(name string) (string, error) {
	data, err := assets.ReadFile(path.Join("page", name))
	return string(data), err
}

// Catalogs is the page's own words, by language tag, as the catalogs under
// page/catalogs hold them.
func Catalogs() (map[string]map[string]any, error) {
	// The pattern is fixed, so Glob cannot refuse it.
	names, _ := fs.Glob(assets, "page/catalogs/*.json")
	catalogs := map[string]map[string]any{}
	for _, name := range names {
		var words map[string]any
		if err := json.Unmarshal([]byte(mustAsset(strings.TrimPrefix(name, "page/"))), &words); err != nil {
			return nil, fmt.Errorf("the catalog %s: %w", path.Base(name), err)
		}
		catalogs[strings.TrimSuffix(path.Base(name), ".json")] = words
	}
	return catalogs, nil
}

// shell is the page with everything but where this Mattermost's resources
// start and what it lets a view change: the same for every server, so made
// once.
var shell = sync.OnceValue(func() string {
	page := mustAsset("page.html")
	style := mustAsset("view.css")
	catalogs, err := Catalogs()
	if err != nil {
		panic(err)
	}
	var script strings.Builder
	script.WriteString("(() => {\n\"use strict\";\n")
	script.WriteString("const RESOURCE_BASE = " + baseMarker + ";\n")
	script.WriteString("const CHANGES = " + changesMarker + ";\n")
	script.WriteString("const POSTS = " + postsMarker + ";\n")
	script.WriteString("const SYSTEM_EMOJI = " + mustJSON(systemEmoji()) + ";\n")
	script.WriteString("const CATALOGS = " + mustJSON(catalogs) + ";\n")
	for _, name := range Scripts {
		script.WriteString("// ---- " + name + " ----\n")
		script.WriteString(mustAsset(name))
		script.WriteString("\n")
	}
	script.WriteString("})();\n")
	if !strings.Contains(page, stylePlaceholder) || !strings.Contains(page, scriptPlaceholder) {
		panic("the page has lost a placeholder")
	}
	return strings.Replace(strings.Replace(page, stylePlaceholder, style, 1), scriptPlaceholder, script.String(), 1)
})

// baseMarker, changesMarker and postsMarker stand in the shell for where
// this Mattermost's resources start and what a view may change, which Page
// puts in.
const (
	baseMarker    = `"\u0000MM_RESOURCE_BASE\u0000"`
	changesMarker = `"\u0000MM_CHANGES\u0000"`
	postsMarker   = `"\u0000MM_POSTS\u0000"`
)

// Settings is what a server lets its views change.
type Settings struct {
	// Changes says the server offers the tool a view changes something
	// through, which it does where writes are allowed: the view offers Mark
	// read, and a proposal's Edit, Discard and Replace mine, only then.
	Changes bool
	// Posts says a proposal may be posted from its view, as
	// MM_MCP_POST_FROM_VIEWS allows: the view offers Post only then.
	Posts bool
}

// Page is the view page for the Mattermost whose resources start at base, as
// mattermost://chat.example.com does, on a server whose settings these are.
func Page(base string, settings Settings) string {
	return strings.NewReplacer(baseMarker, mustJSON(base), changesMarker, mustJSON(settings.Changes),
		postsMarker, mustJSON(settings.Changes && settings.Posts)).Replace(shell())
}

// systemEmoji is Mattermost's table of the emoji that come with it: each
// name, with the code points of its character in hex, joined by dashes.
func systemEmoji() map[string]string {
	return model.SystemEmojis
}

func mustAsset(name string) string {
	text, err := Asset(name)
	if err != nil {
		panic(err)
	}
	return text
}

// mustJSON is value as JSON, which Go's encoder writes with <, > and &
// escaped, so it cannot end the script it stands in.
func mustJSON(value any) string {
	// Strings and maps of them always encode.
	data, _ := json.Marshal(value)
	return string(data)
}
