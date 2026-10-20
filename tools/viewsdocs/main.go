// Command viewsdocs writes the view page for the documentation site, which
// shows mm-mcp's views live (mm-mcp#38): the page the binary embeds, as a
// server whose writes are allowed serves it, for the Mattermost at
// chat.example.com the recorded data names. `task docs:views` runs it, with
// the host it runs in, before the site is built. The file is never
// committed, so every version of the docs shows its own release's page.
//
//	go run ./tools/viewsdocs [-out docs/site/assets/views/page.html]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// recordedServer is the Mattermost the documentation's recordings name.
const recordedServer = "mattermost://chat.example.com"

func main() {
	out := flag.String("out", filepath.Join("docs", "site", "assets", "views", "page.html"), "where the page is written")
	flag.Parse()
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	page := views.Page(recordedServer, views.Settings{Changes: true, Posts: true})
	if err := os.WriteFile(*out, []byte(page), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "viewsdocs:", err)
	os.Exit(1)
}
