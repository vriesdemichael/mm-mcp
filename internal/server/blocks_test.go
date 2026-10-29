package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// What integrations post as blocks or cards, as Mattermost's web app draws it
// where the server turns it on (mm-mcp#75).

// blockPosts are posts incoming webhooks made, as Mattermost 11.11.1 answered
// GetPostsForChannel with them: a deploy in Mattermost's own blocks, an
// incident in Slack's Block Kit, a merge in Adaptive Cards, each with no
// message, and a report with blocks, message attachments and a link.
func blockPosts(t *testing.T) map[string]*model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/block-posts.json")
	if err != nil {
		t.Fatal(err)
	}
	var posts map[string]*model.Post
	if err := json.Unmarshal(data, &posts); err != nil {
		t.Fatal(err)
	}
	return posts
}

// drawn is an outline of blocks as lines a test reads: each block's depth,
// type and what it shows.
func drawn(blocks []Block) string {
	var lines []string
	for _, b := range blocks {
		line := strings.Repeat("  ", b.Depth) + b.Type
		for _, part := range []struct {
			name, value string
		}{{"", b.Text}, {"url", b.URL}, {"alt", b.AltText}, {"title", b.Title}, {"accent", b.AccentColor}} {
			if part.value != "" {
				line += fmt.Sprintf(" %s%q", strings.TrimSuffix(part.name+"=", "="), part.value)
			}
		}
		for name, set := range map[string]bool{" subtle": b.Subtle, " border": b.Border, " collapsed": b.Collapsed} {
			if set {
				line += name
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func TestEachFormatOfBlocksIsDrawnAsMattermostDrawsIt(t *testing.T) {
	t.Parallel()
	posts := blockPosts(t)
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	want := map[string]string{
		"mm_blocks": strings.Join([]string{
			`container accent"danger" border`,
			`  text "**Deploy 4821 failed** on ` + "`api`" + `"`,
			`  column_set`,
			`    column`,
			`      text "Stage" subtle`,
			`      text "migrate"`,
			`    column`,
			`      text "Duration" subtle`,
			`      text "4m 12s"`,
			`  divider`,
			`  collapsible "Log tail" collapsed`,
			`    text "` + "```\\nERROR: relation \\\"users\\\" already exists\\n```" + `"`,
			`  image url"https://ci.example.com/graphs/4821.png" alt"Build time graph"`,
		}, "\n"),
		// A header is a heading, two fields share a row, and context and the
		// button row are not drawn.
		"blocks": strings.Join([]string{
			`text "# Incident 312"`,
			`container`,
			`  text "*Payments* are degraded"`,
			`  column_set`,
			`    column`,
			`      text "*Severity*\nSEV-2"`,
			`    column`,
			`      text "*Owner*\n@oncall"`,
			`  text "*Started*\n09:12"`,
			`divider`,
		}, "\n"),
		// A fact set is not drawn, nor the submit button.
		"cards": strings.Join([]string{
			`text "Pull request #88 merged"`,
			`text "by alice into main" subtle`,
			`column_set`,
			`  column`,
			`    text "Checks: 42 passed"`,
			`  column`,
			`    image url"https://ci.example.com/badge.png" alt"passing"`,
			`container`,
			`  text "Deployed to staging"`,
		}, "\n"),
	}
	for format, outline := range want {
		got := shownWith(posts[format], display{blocks: true}, reader)
		if drawn(got.Blocks) != outline || got.Message != "" || got.MessageAttachments != nil {
			t.Errorf("%s is drawn as\n%s\nwant\n%s", format, drawn(got.Blocks), outline)
		}
		// Where the server draws no blocks, as the ESR does not, nor does a post.
		if got := shownWith(posts[format], display{}, reader); got.Blocks != nil {
			t.Errorf("%s without blocks drawn: %+v", format, got.Blocks)
		}
	}
}

func TestAPostWithBlocksAndAttachmentsCarriesWhatMattermostDraws(t *testing.T) {
	t.Parallel()
	post := blockPosts(t)["both"]
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}

	// Drawn, the blocks come first, in place of the attachments.
	got := shownWith(post, display{blocks: true}, reader)
	if drawn(got.Blocks) != "container accent\"good\"\n  text \"**Nightly**: all green\"" || got.MessageAttachments != nil || got.LinkPreview != nil {
		t.Errorf("with blocks drawn, the report is %s, %+v, %+v", drawn(got.Blocks), got.MessageAttachments, got.LinkPreview)
	}
	// Otherwise its attachments, as before.
	got = shownWith(post, display{}, reader)
	if got.Blocks != nil || len(got.MessageAttachments) != 1 || got.MessageAttachments[0].Title != "Nightly: all green" {
		t.Errorf("without blocks drawn, the report is %+v, %+v", got.Blocks, got.MessageAttachments)
	}
	// A post whose first format is attachments keeps them where blocks are
	// drawn, and nothing at all is drawn of blocks that are not valid.
	alert := recordedAlert(t)
	if got := shownWith(alert, display{blocks: true}, reader); got.Blocks != nil || len(got.MessageAttachments) != 1 {
		t.Errorf("an alert where blocks are drawn is %+v, %+v", got.Blocks, got.MessageAttachments)
	}
	broken := &model.Post{Id: "p1"}
	broken.AddProp(model.PostPropsMmBlocks, []any{map[string]any{"type": "text", "text": 42}})
	broken.AddProp(model.PostPropsAttachments, []any{map[string]any{"text": "hidden"}})
	if got := shownWith(broken, display{blocks: true}, reader); got.Blocks != nil || got.MessageAttachments != nil {
		t.Errorf("a post of invalid blocks is %+v, %+v", got.Blocks, got.MessageAttachments)
	}
}

func TestBlocksThatAreNotValidAreLeftOutAsMattermostLeavesThemOut(t *testing.T) {
	t.Parallel()
	var raw []any
	if err := json.Unmarshal([]byte(`[
		{"type": "text", "text": "kept", "is_subtle": "yes"},
		{"type": "text", "text": "kept", "size": "huge"},
		{"type": "text"},
		{"type": "image", "url": "https://example.com/a.png"},
		{"type": "image", "url": "https://example.com/a.png", "alt_text": "a", "title": 3},
		{"type": "column_set", "columns": [{"type": "column", "items": [{"type": "text", "text": "a"}]}, {"type": "text", "text": "b"}]},
		{"type": "container", "content": [{"type": "button", "text": "Approve", "action_id": "approve"}]},
		{"type": "container", "content": [], "accent_color": "danger"},
		{"type": "collapsible", "header": [], "content": [{"type": "text", "text": "a"}]},
		{"type": "container", "accent_color": "rgb(1,2,3)", "content": [{"type": "text", "text": "a color the view does not draw"}]},
		{"type": "carousel", "items": []},
		"text"
	]`), &raw); err != nil {
		t.Fatal(err)
	}
	got := drawn(outline(limitBlocks(mmBlocks(raw, 1), 1, &blockCounts{}), 0))
	if got != "container\n  text \"a color the view does not draw\"" {
		t.Errorf("invalid blocks are drawn as\n%s", got)
	}
}

func TestBlocksAreHeldToMattermostsLimitsAndAListCutsThem(t *testing.T) {
	t.Parallel()
	// A hundred blocks in all, the containers that hold them counted.
	var many []any
	for i := range 150 {
		many = append(many, map[string]any{"type": "text", "text": fmt.Sprint(i)})
	}
	if got := outline(limitBlocks(mmBlocks(many, 1), 1, &blockCounts{}), 0); len(got) != blocksMost {
		t.Errorf("150 blocks are cut to %d", len(got))
	}
	// Nested 40 deep, the texts below 32 are not drawn.
	var deep any = map[string]any{"type": "text", "text": "bottom"}
	for range 40 {
		deep = map[string]any{"type": "container", "content": []any{map[string]any{"type": "text", "text": "level"}, deep}}
	}
	got := outline(limitBlocks(mmBlocks([]any{deep}, 1), 1, &blockCounts{}), 0)
	if deepest := got[len(got)-1].Depth; deepest != blocksDeepest-1 || strings.Contains(drawn(got), "bottom") {
		t.Errorf("40 deep is drawn %d deep", deepest)
	}
	// 16,000 characters of Markdown in all.
	long := strings.Repeat("m", 10000)
	got = outline(limitBlocks(mmBlocks([]any{map[string]any{"type": "text", "text": long}, map[string]any{"type": "text", "text": long}, map[string]any{"type": "text", "text": "gone"}}, 1), 1, &blockCounts{}), 0)
	if len(got) != 2 || len(got[1].Text) != blocksMostRunes-len(long) {
		t.Errorf("Markdown over the limit is drawn as %d blocks", len(got))
	}

	// A list cuts them with the message, and leaves out what holds nothing.
	blocks := []Block{
		{Type: "container", AccentColor: "danger"},
		{Type: "text", Depth: 1, Text: strings.Repeat("a", 3000)},
		{Type: "column_set", Depth: 1},
		{Type: "column", Depth: 2},
		{Type: "image", Depth: 3, URL: "https://example.com/" + strings.Repeat("i", 2000) + ".png", AltText: "chart"},
		{Type: "divider"},
	}
	cut := clip([]Post{{Message: "hello", Blocks: blocks}})[0]
	if cut.BlocksLength != 3000+len(blocks[4].URL)+len("chart") || drawn(cut.Blocks) != "container accent\"danger\"\n  text \""+strings.Repeat("a", 3000)+"\"" {
		t.Errorf("a list cuts the blocks to\n%s\nof %d", drawn(cut.Blocks), cut.BlocksLength)
	}
	if blocks[4].URL == "" {
		t.Error("clip changed the post's own blocks")
	}
}
