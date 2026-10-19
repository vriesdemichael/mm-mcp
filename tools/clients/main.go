// Command clients installs the MCP clients the client tests drive (ADR-036).
//
//	go run ./tools/clients install   # the versions .github/tool-versions.env pins
//	go run ./tools/clients bump      # moves those pins to each client's newest release
//
// install unpacks each client under .tmp/clients/, checked against the
// SHA-256 its publisher gives. bump is the scheduled run's first step: it
// rewrites the pins, and says which moved, so the run tests what people
// install today and proposes the new pins when the tests pass. Dependabot
// does not follow these clients. GITHUB_TOKEN, when set, raises GitHub's
// limit on API reads.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vriesdemichael/mm-mcp/internal/testclients"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: clients install|bump")
		os.Exit(2)
	}
	root, err := filepath.Abs(".")
	if err != nil {
		fail(err)
	}
	installer := testclients.Installer{Root: root, GitHubToken: os.Getenv("GITHUB_TOKEN")}
	ctx := context.Background()
	switch os.Args[1] {
	case "install":
		pinned, err := testclients.Pinned(root)
		if err != nil {
			fail(err)
		}
		for _, client := range testclients.Clients {
			program, err := installer.Install(ctx, client, pinned[client])
			if err != nil {
				fail(err)
			}
			fmt.Printf("%s %s: %s\n", client, pinned[client], program)
		}
	case "bump":
		if err := bump(ctx, installer); err != nil {
			fail(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: clients install|bump")
		os.Exit(2)
	}
}

// bump sets each client's pin to its newest release, and prints the ones that
// moved as "client old new".
func bump(ctx context.Context, installer testclients.Installer) error {
	pinned, err := testclients.Pinned(installer.Root)
	if err != nil {
		return err
	}
	path := filepath.Join(installer.Root, ".github", "tool-versions.env")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	for _, client := range testclients.Clients {
		newest, err := installer.Newest(ctx, client)
		if err != nil {
			return fmt.Errorf("finding %s's newest release: %w", client, err)
		}
		if newest == pinned[client] {
			continue
		}
		pin := regexp.MustCompile(`(?m)^(` + testclients.Pin(client) + `=).*$`)
		text = pin.ReplaceAllString(text, "${1}"+newest)
		fmt.Printf("%s %s %s\n", client, pinned[client], newest)
	}
	if text == string(content) {
		return nil
	}
	return os.WriteFile(path, []byte(strings.ReplaceAll(text, "\r\n", "\n")), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "clients:", err)
	os.Exit(1)
}
