package views_test

import (
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The page's governance tests hold what ADR-022 asks of it: it builds no HTML
// from strings, reaches no network, never speaks for the person, and every
// word it shows comes from a catalog every language keeps whole.

func scripts(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	for _, name := range views.Scripts {
		source, err := views.Asset(name)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = source
	}
	// A scan that finds too little to scan has stopped guarding.
	if len(sources) < 8 {
		t.Fatalf("only %d scripts to scan", len(sources))
	}
	return sources
}

func TestThePageBuildsNoHTMLFromStrings(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment", "DOMParser",
		"eval(", "new Function", "Function(", ".srcdoc", "setAttribute(\"srcdoc\"", "javascript:", "setAttribute(\"on", ".setHTML", "parseHTML",
	}
	for name, source := range scripts(t) {
		for _, word := range forbidden {
			if strings.Contains(source, word) {
				t.Errorf("%s uses %s: the page builds elements, never HTML from a string", name, word)
			}
		}
		// A picture is set only where it is checked to be one the view holds.
		if name != "dom.js" && (strings.Contains(source, "setAttribute(\"src\"") || regexp.MustCompile(`\.src\s*=`).MatchString(source)) {
			t.Errorf("%s sets an image's source itself; showImage checks it", name)
		}
	}
}

func TestThePageNeverSpeaksForThePerson(t *testing.T) {
	t.Parallel()
	for name, source := range scripts(t) {
		if strings.Contains(source, "ui/message") || strings.Contains(source, "sendMessage") {
			t.Errorf("%s sends a message in the person's name", name)
		}
	}
}

// external is an address on another host, which the page never names but for
// the namespace an SVG element is made in.
var external = regexp.MustCompile(`(?i)https?://[a-z0-9-]+\.[a-z0-9.-]+`)

func TestThePageIsOneSelfContainedDocument(t *testing.T) {
	t.Parallel()
	page := views.Page("mattermost://chat.example.com")
	if !strings.HasPrefix(page, "<!doctype html>") {
		t.Error("the page is not an HTML document")
	}
	for _, left := range []string{"MM_VIEW_STYLE", "MM_VIEW_SCRIPT", "MM_RESOURCE_BASE"} {
		if strings.Contains(page, left) {
			t.Errorf("the page still holds %s", left)
		}
	}
	if got := strings.Count(page, "<script"); got != 1 {
		t.Errorf("the page has %d scripts; it has one", got)
	}
	if got := strings.Count(strings.ToLower(page), "</script"); got != 1 {
		t.Errorf("the page ends a script %d times; something in it ends the script early", got)
	}
	if got := strings.Count(strings.ToLower(page), "</style"); got != 2 {
		t.Errorf("the page ends a style %d times; something in it ends the style early", got)
	}
	for _, found := range external.FindAllString(page, -1) {
		if found != "http://www.w3.org" {
			t.Errorf("the page names %s: it reaches no network", found)
		}
	}
	for _, reach := range []string{"@import", "url(", "fetch(", "XMLHttpRequest", "WebSocket", "EventSource", "sendBeacon", " src=", " href=", "<link", "<iframe", "<object", "<embed"} {
		if strings.Contains(page, reach) {
			t.Errorf("the page holds %q: it reaches no network and loads nothing", reach)
		}
	}
	if !strings.Contains(page, `const RESOURCE_BASE = "mattermost://chat.example.com";`) {
		t.Error("the page does not know where its Mattermost's resources start")
	}
}

// topLevel is a name a script declares in the scope every script shares.
var topLevel = regexp.MustCompile(`(?m)^(?:async\s+)?(?:function\s+([A-Za-z_$][\w$]*)|(?:const|let|var|class)\s+([A-Za-z_$][\w$]*))`)

func TestThePageDeclaresEachNameOnce(t *testing.T) {
	t.Parallel()
	declared := map[string]string{"RESOURCE_BASE": "the page", "SYSTEM_EMOJI": "the page", "CATALOGS": "the page"}
	count := 0
	for _, name := range views.Scripts {
		source, err := views.Asset(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range topLevel.FindAllStringSubmatch(source, -1) {
			count++
			found := match[1] + match[2]
			if before, ok := declared[found]; ok {
				t.Errorf("%s declares %s, which %s declares too: the scripts share one scope", name, found, before)
			}
			declared[found] = name
		}
	}
	if count < 40 {
		t.Fatalf("only %d names found; the scan has stopped matching", count)
	}
}

// used is a word the page asks its catalog for.
var used = regexp.MustCompile(`\bt\("([a-z_]+\.[a-z_]+)"`)

func TestEveryWordThePageShowsIsInEnglishAndEveryEnglishWordIsShown(t *testing.T) {
	t.Parallel()
	catalogs, err := views.Catalogs()
	if err != nil {
		t.Fatal(err)
	}
	english := catalogs["en"]
	asked := map[string]bool{}
	for name, source := range scripts(t) {
		for _, match := range used.FindAllStringSubmatch(source, -1) {
			asked[match[1]] = true
			if _, ok := english[match[1]]; !ok {
				t.Errorf("%s shows %s, which English does not have", name, match[1])
			}
		}
	}
	if len(asked) < 30 {
		t.Fatalf("only %d words found; the scan has stopped matching", len(asked))
	}
	for key := range english {
		if !asked[key] {
			t.Errorf("English has %s, which the page never shows", key)
		}
	}
}

// placeholder is a value a word puts in its place.
var placeholder = regexp.MustCompile(`\{([a-z_]+)\}`)

func placeholders(text string) []string {
	var found []string
	for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
		found = append(found, match[1])
	}
	sort.Strings(found)
	return slices.Compact(found)
}

func TestEveryCatalogHasTheEnglishWords(t *testing.T) {
	t.Parallel()
	catalogs, err := views.Catalogs()
	if err != nil {
		t.Fatal(err)
	}
	english, ok := catalogs["en"]
	if !ok || len(english) < 30 {
		t.Fatalf("English has %d words", len(english))
	}
	for language, words := range catalogs {
		if got, want := slices.Sorted(maps.Keys(words)), slices.Sorted(maps.Keys(english)); !slices.Equal(got, want) {
			t.Errorf("%s has the words %v; English has %v", language, got, want)
			continue
		}
		for key, word := range words {
			switch english := english[key].(type) {
			case string:
				text, ok := word.(string)
				if !ok {
					t.Errorf("%s: %s counts, where English does not", language, key)
					continue
				}
				if got, want := placeholders(text), placeholders(english); !slices.Equal(got, want) {
					t.Errorf("%s: %s puts in %v; English puts in %v", language, key, got, want)
				}
			case map[string]any:
				forms, ok := word.(map[string]any)
				if !ok {
					t.Errorf("%s: %s does not count, where English does", language, key)
					continue
				}
				if _, ok := forms["other"].(string); !ok {
					t.Errorf("%s: %s has no other form", language, key)
				}
				if _, ok := english["one"]; ok {
					if _, ok := forms["one"].(string); !ok {
						t.Errorf("%s: %s has no one form, which English has", language, key)
					}
				}
				want := placeholders(english["other"].(string))
				for form, text := range forms {
					text, ok := text.(string)
					if !ok {
						t.Errorf("%s: %s's %s form is not a word", language, key, form)
						continue
					}
					if got := placeholders(text); !slices.Equal(got, want) && !(form != "other" && slices.Equal(got, without(want, "count"))) {
						t.Errorf("%s: %s's %s form puts in %v; English puts in %v", language, key, form, got, want)
					}
				}
			default:
				t.Errorf("English's %s is neither a word nor its plural forms", key)
			}
		}
	}
}

// without is list less one value: a plural form for one may say "a" or "one"
// rather than put in the count.
func without(list []string, value string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(item string) bool { return item == value })
}
