package server

import (
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

// Interactive Messages are what an integration posts as blocks rather than a
// message: Mattermost's own (props.mm_blocks), Slack's Block Kit
// (props.blocks) or Microsoft's Adaptive Cards (props.cards). Mattermost's web
// app draws them where the server turns on the feature flag MmBlocksEnabled,
// 11.11 by default, translating each format into its own blocks and drawing
// the first the post has, in the order mm_blocks, blocks, cards, attachments.
// A post carries what it draws, in one shape whichever format they came in,
// translated as the web app at 11.11.1 translates them
// (components/block_renderer/translation), without the buttons and menus:
// pressing one is a write, and it calls the integration.
//
// A schema cannot nest a type in itself, so the blocks come as an outline: a
// container, collapsible, column set or column holds the blocks after it with
// a greater depth.

// Block is one block of what an integration posted as blocks or cards.
type Block struct {
	Type        string `json:"type" jsonschema:"text, image, divider, container, collapsible, column_set or column"`
	Depth       int    `json:"depth,omitempty" jsonschema:"how deep it is nested: the blocks right after a container, collapsible, column_set or column with a depth one greater are in it"`
	Text        string `json:"text,omitempty" jsonschema:"a text's Markdown, or a collapsible's header, which Mattermost shows while its blocks are folded away"`
	Subtle      bool   `json:"subtle,omitempty" jsonschema:"set for a text Mattermost shows muted"`
	URL         string `json:"url,omitempty" jsonschema:"an image's address"`
	AltText     string `json:"alt_text,omitempty" jsonschema:"what an image shows, as the integration describes it"`
	Title       string `json:"title,omitempty" jsonschema:"an image's title"`
	AccentColor string `json:"accent_color,omitempty" jsonschema:"the color of the bar beside a container, which the integration chose: default, primary, good, warning, danger, or any CSS color, such as red or #e40303"`
	Border      bool   `json:"border,omitempty" jsonschema:"set for a container Mattermost draws with a border"`
	Collapsed   bool   `json:"collapsed,omitempty" jsonschema:"set for a collapsible Mattermost shows folded, its header alone, until the person opens it"`
}

// The limits the web app holds a post's blocks to, in limits.ts: so many
// blocks in all, nested so deep, with so much Markdown.
const (
	blocksMost      = 100
	blocksDeepest   = 32
	blocksMostRunes = 16000
)

// block is a block as the web app translates it, before the limits, with the
// buttons and menus that mm-mcp leaves out once they are counted.
type block struct {
	kind            string
	text            string
	subtle          bool
	url, alt, title string
	accent          string
	border          bool
	collapsed       bool
	// header holds a collapsible's header; content a container's or a
	// collapsible's blocks, a column's items, and a column set's columns.
	header, content []block
}

// interactive is the blocks Mattermost draws of the post, and whether it draws
// them in place of its message attachments and the preview of its link: where
// the server turns blocks on and the post has some, in the first format it has.
// A post whose first format is attachments keeps them as message attachments.
func (d display) interactive(post *model.Post) ([]Block, bool) {
	if !d.blocks {
		return nil, false
	}
	props := post.GetProps()
	var translated []block
	if raw := nonEmpty(props[model.PostPropsMmBlocks]); raw != nil {
		translated = mmBlocks(raw, 1)
	} else if raw := nonEmpty(props[model.PostPropsBlockKitBlocks]); raw != nil {
		translated = blockKit(raw)
	} else if raw := nonEmpty(props[model.PostPropsAdaptiveCards]); raw != nil {
		translated = adaptiveCards(raw)
	} else {
		return nil, false
	}
	return outline(limitBlocks(translated, 1, &blockCounts{}), 0), true
}

// nonEmpty is a prop that holds a list with something in it, or nil.
func nonEmpty(raw any) []any {
	if list, ok := raw.([]any); ok && len(list) > 0 {
		return list
	}
	return nil
}

// fields reads a block's JSON object, or nothing.
func fields(raw any) (map[string]any, bool) {
	object, ok := raw.(map[string]any)
	return object, ok
}

// text is a field that holds text, and whether it does.
func text(object map[string]any, key string) (string, bool) {
	value, ok := object[key].(string)
	return value, ok
}

// flag is a field that holds true or false, false when it is missing, and
// whether it holds anything else, which makes the block invalid.
func flag(object map[string]any, key string) (bool, bool) {
	raw, present := object[key]
	if !present {
		return false, true
	}
	value, ok := raw.(bool)
	return value, ok
}

// mmBlocks translates Mattermost's own blocks as translateMMBlocks does,
// dropping each that is not valid. depth stops at the deepest the limits keep.
func mmBlocks(raw []any, depth int) []block {
	var out []block
	for _, item := range raw {
		if translated, ok := mmBlock(item, depth); ok {
			out = append(out, translated)
		}
	}
	return out
}

func mmBlock(raw any, depth int) (block, bool) {
	object, ok := fields(raw)
	if !ok || depth > blocksDeepest {
		return block{}, false
	}
	kind, _ := text(object, "type")
	switch kind {
	case "text":
		value, ok := text(object, "text")
		subtle, valid := flag(object, "is_subtle")
		size, sized := object["size"]
		if !ok || !valid || sized && size != "small" && size != "default" {
			return block{}, false
		}
		return block{kind: kind, text: value, subtle: subtle}, true
	case "divider":
		return block{kind: kind}, true
	case "button", "static_select":
		// Counted as Mattermost counts them, and left out once counted.
		label, _ := text(object, "text")
		if kind == "static_select" {
			label, _ = text(object, "placeholder")
		}
		action, _ := text(object, "action_id")
		if strings.TrimSpace(label) == "" || strings.TrimSpace(action) == "" {
			return block{}, false
		}
		return block{kind: kind, text: label}, true
	case "image":
		url, ok := text(object, "url")
		alt, described := text(object, "alt_text")
		title, titled := object["title"]
		if !ok || !described || titled && !isString(title) {
			return block{}, false
		}
		named, _ := title.(string)
		return block{kind: kind, url: url, alt: alt, title: named}, true
	case "column":
		items, ok := object["items"].([]any)
		width, sized := object["width"]
		if !ok || sized && width != "auto" && width != "stretch" {
			return block{}, false
		}
		content := mmBlocks(items, depth+1)
		return block{kind: kind, content: content}, len(content) > 0
	case "column_set":
		raw, ok := object["columns"].([]any)
		if !ok {
			return block{}, false
		}
		var columns []block
		for _, item := range raw {
			column, ok := fields(item)
			if !ok || column["type"] != "column" {
				return block{}, false
			}
			translated, ok := mmBlock(column, depth+1)
			if !ok {
				return block{}, false
			}
			columns = append(columns, translated)
		}
		return block{kind: kind, content: columns}, len(columns) > 0
	case "container":
		items, ok := object["content"].([]any)
		border, valid := flag(object, "border")
		accent, colored := object["accent_color"]
		if !ok || !valid || colored && !isString(accent) {
			return block{}, false
		}
		content := mmBlocks(items, depth+1)
		color, _ := accent.(string)
		return block{kind: kind, accent: color, border: border, content: content}, len(content) > 0
	case "collapsible":
		header, ok := object["header"].([]any)
		items, holds := object["content"].([]any)
		collapsed, valid := flag(object, "collapsed")
		if !ok || !holds || !valid {
			return block{}, false
		}
		translated := block{kind: kind, collapsed: collapsed, header: mmBlocks(header, depth+1), content: mmBlocks(items, depth+1)}
		return translated, len(translated.header) > 0 && len(translated.content) > 0
	}
	return block{}, false
}

func isString(raw any) bool {
	_, ok := raw.(string)
	return ok
}

// blockKit translates Slack's Block Kit as translateBlockKit does: a section's
// text and fields, two fields to a row, a header as a heading, Markdown, a
// divider, an image, and buttons and menus. Anything else, such as context, is
// not drawn.
func blockKit(raw []any) []block {
	var out []block
	for _, item := range raw {
		object, ok := fields(item)
		if !ok {
			continue
		}
		switch object["type"] {
		case "section":
			if section, ok := blockKitSection(object); ok {
				out = append(out, section)
			}
		case "header":
			if plain := blockKitText(object["text"]); plain != "" {
				out = append(out, block{kind: "text", text: "# " + plain})
			}
		case "markdown":
			if value, _ := text(object, "text"); strings.TrimSpace(value) != "" {
				out = append(out, block{kind: "text", text: value})
			}
		case "divider":
			out = append(out, block{kind: "divider"})
		case "image":
			if image, ok := blockKitImage(object); ok {
				out = append(out, image)
			}
		case "actions":
			if controls := blockKitControls(object["elements"]); len(controls) > 0 {
				out = append(out, block{kind: "container", content: controls})
			}
		}
	}
	return out
}

// blockKitText is the text of a Block Kit text object.
func blockKitText(raw any) string {
	object, ok := fields(raw)
	if !ok {
		return ""
	}
	value, _ := text(object, "text")
	return value
}

func blockKitSection(object map[string]any) (block, bool) {
	main := blockKitText(object["text"])
	var cells []block
	var pending string
	if list, ok := object["fields"].([]any); ok {
		for _, field := range list {
			value := blockKitText(field)
			switch {
			case value == "":
			case pending == "":
				pending = value
			default:
				cells = append(cells, block{kind: "column_set", content: []block{
					{kind: "column", content: []block{{kind: "text", text: pending}}},
					{kind: "column", content: []block{{kind: "text", text: value}}},
				}})
				pending = ""
			}
		}
	}
	if pending != "" {
		cells = append(cells, block{kind: "text", text: pending})
	}
	if main == "" && len(cells) == 0 {
		return block{}, false
	}
	var content []block
	if main != "" {
		content = append(content, block{kind: "text", text: main})
	}
	content = append(content, cells...)
	if accessory, ok := blockKitAccessory(object["accessory"]); ok {
		content = []block{{kind: "column_set", content: []block{
			{kind: "column", content: content},
			{kind: "column", content: []block{accessory}},
		}}}
	}
	return block{kind: "container", content: content}, true
}

func blockKitAccessory(raw any) (block, bool) {
	object, ok := fields(raw)
	if !ok {
		return block{}, false
	}
	switch object["type"] {
	case "button":
		label := blockKitText(object["text"])
		action, _ := text(object, "action_id")
		return block{kind: "button", text: label}, label != "" && action != ""
	case "image":
		return blockKitImage(object)
	}
	return block{}, false
}

func blockKitImage(object map[string]any) (block, bool) {
	url, _ := text(object, "image_url")
	alt, _ := text(object, "alt_text")
	return block{kind: "image", url: url, alt: alt, title: blockKitText(object["title"])}, url != "" && alt != ""
}

func blockKitControls(raw any) []block {
	list, _ := raw.([]any)
	var out []block
	for _, item := range list {
		object, ok := fields(item)
		if !ok {
			continue
		}
		action, _ := text(object, "action_id")
		switch object["type"] {
		case "button":
			if label := blockKitText(object["text"]); label != "" && action != "" {
				out = append(out, block{kind: "button", text: label})
			}
		case "static_select":
			if label := blockKitText(object["placeholder"]); label != "" && action != "" && blockKitHasOption(object["options"]) {
				out = append(out, block{kind: "static_select", text: label})
			}
		}
	}
	return out
}

func blockKitHasOption(raw any) bool {
	list, _ := raw.([]any)
	for _, item := range list {
		if object, ok := fields(item); ok {
			if value, _ := text(object, "value"); value != "" && blockKitText(object["text"]) != "" {
				return true
			}
		}
	}
	return false
}

// adaptiveCards translates Microsoft's Adaptive Cards as
// translateAdaptiveCards does: a text block, a container, a column set, an
// image and submit buttons. Anything else, such as a fact set, is not drawn.
func adaptiveCards(raw []any) []block {
	var out []block
	for _, item := range raw {
		card, ok := fields(item)
		if !ok || card["type"] != "AdaptiveCard" {
			continue
		}
		if body, ok := card["body"].([]any); ok {
			out = append(out, adaptiveItems(body, 1)...)
		}
		if actions, ok := card["actions"].([]any); ok {
			if submits := adaptiveSubmits(actions); len(submits) > 0 {
				out = append(out, block{kind: "container", content: submits})
			}
		}
	}
	return out
}

func adaptiveItems(raw []any, depth int) []block {
	var out []block
	for _, item := range raw {
		if translated, ok := adaptiveItem(item, depth); ok {
			out = append(out, translated)
		}
	}
	return out
}

func adaptiveItem(raw any, depth int) (block, bool) {
	object, ok := fields(raw)
	if !ok || depth > blocksDeepest {
		return block{}, false
	}
	switch object["type"] {
	case "TextBlock":
		value, _ := text(object, "text")
		return block{kind: "text", text: value, subtle: object["isSubtle"] == true}, value != ""
	case "Container":
		items, ok := object["items"].([]any)
		if !ok {
			return block{}, false
		}
		content := adaptiveItems(items, depth+1)
		return block{kind: "container", content: content}, len(content) > 0
	case "ColumnSet":
		list, ok := object["columns"].([]any)
		if !ok {
			return block{}, false
		}
		var columns []block
		for _, item := range list {
			column, ok := fields(item)
			if !ok {
				continue
			}
			items, _ := column["items"].([]any)
			columns = append(columns, block{kind: "column", content: adaptiveItems(items, depth+2)})
		}
		return block{kind: "column_set", content: columns}, len(columns) > 0
	case "Image":
		url, _ := text(object, "url")
		alt, _ := text(object, "altText")
		return block{kind: "image", url: url, alt: alt}, url != ""
	case "ActionSet":
		actions, ok := object["actions"].([]any)
		if !ok {
			return block{}, false
		}
		submits := adaptiveSubmits(actions)
		return block{kind: "container", content: submits}, len(submits) > 0
	}
	return block{}, false
}

func adaptiveSubmits(raw []any) []block {
	var out []block
	for _, item := range raw {
		object, ok := fields(item)
		if !ok || object["type"] != "Action.Submit" {
			continue
		}
		title, _ := text(object, "title")
		id, _ := text(object, "id")
		if title != "" && id != "" {
			out = append(out, block{kind: "button", text: title})
		}
	}
	return out
}

// blockCounts is how much of the limits the blocks kept so far use.
type blockCounts struct {
	blocks, runes int
}

// limitBlocks holds blocks to the web app's limits as applyBlockTranslationLimits
// does: so many blocks in all, nested so deep, and so much Markdown, a text
// cut where it runs out; a block left with nothing in it goes.
func limitBlocks(blocks []block, depth int, counts *blockCounts) []block {
	if depth > blocksDeepest {
		return nil
	}
	var out []block
	for _, b := range blocks {
		if counts.blocks >= blocksMost {
			break
		}
		if limited, ok := limitBlock(b, depth, counts); ok {
			out = append(out, limited)
		}
	}
	return out
}

func limitBlock(b block, depth int, counts *blockCounts) (block, bool) {
	counts.blocks++
	kept := true
	switch b.kind {
	case "text", "button":
		left := blocksMostRunes - counts.runes
		if length := utf8.RuneCountInString(b.text); length > left {
			b.text = string([]rune(b.text)[:max(left, 0)])
		}
		counts.runes += utf8.RuneCountInString(b.text)
		kept = b.text != ""
	case "container", "column", "column_set":
		b.content = limitBlocks(b.content, depth+1, counts)
		kept = len(b.content) > 0
	case "collapsible":
		b.header = limitBlocks(b.header, depth+1, counts)
		b.content = limitBlocks(b.content, depth+1, counts)
		kept = len(b.header) > 0 && len(b.content) > 0
	}
	if !kept {
		counts.blocks--
	}
	return b, kept
}

// outline is blocks as a post carries them, from depth: each block, then
// those it holds, without the buttons and menus, and without a block that
// holds nothing else. A collapsible's header is the text it shows.
func outline(blocks []block, depth int) []Block {
	var out []Block
	for _, b := range blocks {
		shown := Block{Type: b.kind, Depth: depth}
		switch b.kind {
		case "button", "static_select":
			continue
		case "text":
			shown.Text, shown.Subtle = b.text, b.subtle
		case "image":
			shown.URL, shown.AltText, shown.Title = b.url, b.alt, b.title
		case "container":
			shown.AccentColor, shown.Border = containerAccent(strings.TrimSpace(b.accent)), b.border
		case "collapsible":
			shown.Text, shown.Collapsed = headerText(b.header), b.collapsed
		}
		if b.kind == "divider" || b.kind == "text" || b.kind == "image" {
			out = append(out, shown)
			continue
		}
		inside := outline(b.content, depth+1)
		if len(inside) == 0 {
			continue
		}
		out = append(append(out, shown), inside...)
	}
	return out
}

// headerText is the text of a collapsible's header.
func headerText(header []block) string {
	var texts []string
	for _, b := range header {
		switch {
		case b.kind == "text":
			texts = append(texts, b.text)
		case len(b.content) > 0:
			if inside := headerText(b.content); inside != "" {
				texts = append(texts, inside)
			}
		}
	}
	return strings.Join(texts, "\n")
}

// structural says whether a block holds the blocks after it.
func (b Block) structural() bool {
	return b.Type == "container" || b.Type == "collapsible" || b.Type == "column_set" || b.Type == "column"
}

// texts are the block's texts, and whether each is whole or nothing, as an
// address is.
func (b *Block) texts() []attachmentText {
	return []attachmentText{{text: &b.Text}, {text: &b.URL, whole: true}, {text: &b.AltText}, {text: &b.Title}}
}

// clipBlocks is blocks cut to at most left characters of text, in the order
// Mattermost shows them, and the whole length of their text when it is more.
// What is cut ends in …, what follows is left out, and so is a block left
// holding nothing.
func clipBlocks(blocks []Block, left int) ([]Block, int) {
	whole := 0
	for i := range blocks {
		for _, text := range blocks[i].texts() {
			whole += utf8.RuneCountInString(*text.text)
		}
	}
	if whole <= left {
		return blocks, 0
	}
	var cut []Block
	for _, b := range blocks {
		if left <= 0 {
			break
		}
		for _, text := range b.texts() {
			length := utf8.RuneCountInString(*text.text)
			switch {
			case length <= left:
				left -= length
			case text.whole || left == 0:
				*text.text, left = "", 0
			default:
				*text.text, left = string([]rune(*text.text)[:left])+"…", 0
			}
		}
		if b.structural() || b.Type == "divider" || b.Text != "" || b.URL != "" {
			cut = append(cut, b)
		}
	}
	// A block that holds nothing once the rest is cut goes, from the end, so
	// that one holding only those goes too.
	var kept []Block
	for i := len(cut) - 1; i >= 0; i-- {
		b := cut[i]
		if b.structural() && (len(kept) == 0 || kept[len(kept)-1].Depth <= b.Depth) {
			continue
		}
		kept = append(kept, b)
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept, whole
}
