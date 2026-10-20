package views_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The README shows pictures of the views, since GitHub runs no script
// (mm-mcp#38). `task docs:view-pictures` takes them from the documentation's
// page and recordings, and stamps them with what they were made from. A
// change to the page or to a recording leaves the pictures showing what the
// views no longer draw, until they are taken again.

var assets = filepath.Join("..", "..", "docs", "site", "assets", "views")

func TestTheREADMEsPicturesOfTheViewsAreCurrent(t *testing.T) {
	sum := func(data []byte) string {
		digest := sha256.Sum256(data)
		return hex.EncodeToString(digest[:])
	}
	want := []string{"page " + sum([]byte(views.DocsPage()))}
	recordings, err := filepath.Glob(filepath.Join(assets, "recordings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(recordings)
	for _, file := range recordings {
		data, err := os.ReadFile(file) //nolint:gosec // a recording of the repository's
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, "recording "+filepath.Base(file)+" "+sum(data))
	}
	raw, err := os.ReadFile(filepath.Join(assets, "pictures", "stamp.txt"))
	if err != nil {
		t.Fatalf("the README's pictures of the views have no stamp; task docs:view-pictures takes them: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i, line := range want {
		if i >= len(got) || got[i] != line {
			t.Errorf("the README's pictures of the views were taken from another %s; task docs:view-pictures takes them again", strings.Fields(line)[0])
			return
		}
	}
	if len(got) != len(want) {
		t.Errorf("the README's pictures were taken from %d recordings; there are %d; task docs:view-pictures takes them again", len(got)-1, len(want)-1)
	}
}
