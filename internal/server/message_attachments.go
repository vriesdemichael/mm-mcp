package server

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

// Message attachments are the cards an incoming webhook, a bot or an
// integration such as Alertmanager or a CI server posts its content in, in
// Slack's format, often with no message beside them. A post carries them as
// Mattermost shows them under its message, without their buttons and menus:
// pressing one is a write, and it calls the integration, not Mattermost.

// MessageAttachment is one of a post's message attachments.
type MessageAttachment struct {
	Color      string                   `json:"color,omitempty" jsonschema:"the color of the bar beside it, which the integration chose: good, warning, danger or a hex color"`
	Pretext    string                   `json:"pretext,omitempty" jsonschema:"the text above it, in Mattermost Markdown"`
	AuthorName string                   `json:"author_name,omitempty" jsonschema:"the name it shows at its top, which the integration chose"`
	Title      string                   `json:"title,omitempty"`
	TitleLink  string                   `json:"title_link,omitempty" jsonschema:"where its title links to"`
	Text       string                   `json:"text,omitempty" jsonschema:"in Mattermost Markdown"`
	Fields     []MessageAttachmentField `json:"fields,omitempty" jsonschema:"a table of titled values"`
	Footer     string                   `json:"footer,omitempty"`
	Fallback   string                   `json:"fallback,omitempty" jsonschema:"a plain-text summary, which Mattermost shows in a notification rather than in the channel"`
}

// MessageAttachmentField is one titled value of a message attachment.
type MessageAttachmentField struct {
	Title string `json:"title,omitempty"`
	Value string `json:"value,omitempty" jsonschema:"in Mattermost Markdown"`
	Short bool   `json:"short,omitempty" jsonschema:"whether Mattermost shows it beside the next field rather than on a row of its own"`
}

// attachmentColor is a color Mattermost draws the bar of an attachment in: a
// hex color, or one of the three it names.
var attachmentColor = regexp.MustCompile(`^(#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})|good|warning|danger)$`)

// messageAttachments are a post's message attachments, each with what
// Mattermost shows of it. One that only holds buttons or menus is left out.
func messageAttachments(post *model.Post) []MessageAttachment {
	var out []MessageAttachment
	for _, attachment := range post.Attachments() {
		if attachment == nil {
			continue
		}
		shaped := MessageAttachment{
			Pretext:    attachment.Pretext,
			AuthorName: attachment.AuthorName,
			Title:      attachment.Title,
			TitleLink:  attachment.TitleLink,
			Text:       attachment.Text,
			Footer:     attachment.Footer,
			Fallback:   attachment.Fallback,
		}
		// Mattermost draws no bar for a color it does not know.
		if attachmentColor.MatchString(attachment.Color) {
			shaped.Color = attachment.Color
		}
		for _, field := range attachment.Fields {
			if field != nil {
				shaped.Fields = append(shaped.Fields, MessageAttachmentField{Title: field.Title, Value: fieldValue(field.Value), Short: bool(field.Short)})
			}
		}
		if !shaped.empty() {
			out = append(out, shaped)
		}
	}
	return out
}

// fieldValue is a field's value as text: a string as it is, and a number or
// anything else an integration sent as JSON.
func fieldValue(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

// attachmentText is one of an attachment's texts, and whether it is whole or
// nothing, as a link is: a link cut short leads somewhere else.
type attachmentText struct {
	text  *string
	whole bool
}

// texts are the attachment's texts in the order Mattermost shows them, its
// fallback last, which it shows only in a notification. Its color is not
// among them: it is one of a few short words.
func (a *MessageAttachment) texts() []attachmentText {
	out := []attachmentText{{text: &a.Pretext}, {text: &a.AuthorName}, {text: &a.Title}, {text: &a.TitleLink, whole: true}, {text: &a.Text}}
	for i := range a.Fields {
		out = append(out, attachmentText{text: &a.Fields[i].Title}, attachmentText{text: &a.Fields[i].Value})
	}
	return append(out, attachmentText{text: &a.Footer}, attachmentText{text: &a.Fallback})
}

// empty says whether the attachment shows nothing.
func (a MessageAttachment) empty() bool {
	for _, text := range a.texts() {
		if *text.text != "" {
			return false
		}
	}
	return true
}

// clipAttachments is attachments cut to at most left characters of text, in
// the order Mattermost shows them, and the whole length of their text when it
// is more. What is cut ends in …, and what follows is left out. The
// attachments are copied rather than changed, since a post's are shared with
// the copy of it a thread lists.
func clipAttachments(attachments []MessageAttachment, left int) ([]MessageAttachment, int) {
	whole := 0
	for _, attachment := range attachments {
		for _, text := range attachment.texts() {
			whole += utf8.RuneCountInString(*text.text)
		}
	}
	if whole <= left {
		return attachments, 0
	}
	var out []MessageAttachment
	for _, attachment := range attachments {
		if left <= 0 {
			break
		}
		attachment.Fields = slices.Clone(attachment.Fields)
		for _, text := range attachment.texts() {
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
		attachment.Fields = slices.DeleteFunc(attachment.Fields, func(field MessageAttachmentField) bool {
			return field.Title == "" && field.Value == ""
		})
		if !attachment.empty() {
			out = append(out, attachment)
		}
	}
	return out, whole
}
