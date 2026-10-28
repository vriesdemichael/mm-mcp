package server

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

// Message attachments, where no request is involved: a post as Mattermost
// answered it is shaped as a tool answers it, and a list cuts it short.

// recordedAlert is a post an incoming webhook made from Alertmanager's
// payload, as Mattermost answered GetPostsForChannel with it.
func recordedAlert(t *testing.T) *model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/alert-post.json")
	if err != nil {
		t.Fatal(err)
	}
	var post model.Post
	if err := json.Unmarshal(data, &post); err != nil {
		t.Fatal(err)
	}
	return &post
}

func TestAWebhookPostCarriesItsMessageAttachments(t *testing.T) {
	t.Parallel()
	post := recordedAlert(t)
	shaped := surroundings{client: clientAt("https://chat.example.com"), zone: time.UTC}.post(post)
	want := []MessageAttachment{{
		Color:     "#e40303",
		Title:     "[FIRING:1] HighErrorRate api (critical)",
		TitleLink: "https://alertmanager.example.com/#/alerts?receiver=mattermost",
		Text:      "*Alert:* API error rate above 5% - `critical`\n*Description:* 7.2% of requests to api failed in the last 5 minutes",
		Fields: []MessageAttachmentField{
			{Title: "severity", Value: "critical", Short: true},
			{Title: "service", Value: "api", Short: true},
			{Title: "replicas", Value: "3", Short: true},
		},
		Footer:   "Alertmanager",
		Fallback: "[FIRING:1] HighErrorRate api (critical)",
	}}
	if !reflect.DeepEqual(shaped.MessageAttachments, want) {
		t.Fatalf("got %+v\nwant %+v", shaped.MessageAttachments, want)
	}
	if shaped.Message != "" || shaped.Type != model.PostTypeMessageAttachment {
		t.Errorf("message %q, type %q", shaped.Message, shaped.Type)
	}
	data, err := json.Marshal(shaped)
	if err != nil {
		t.Fatal(err)
	}
	// What Mattermost does not show stays out: an id, empty icons and images.
	for _, left := range []string{`"id":0`, "author_icon", "image_url", "thumb_url", "footer_icon", `"ts"`, "actions"} {
		if strings.Contains(string(data), left) {
			t.Errorf("the post carries %s: %s", left, data)
		}
	}
}

func TestAMessageAttachmentKeepsOnlyAColorMattermostDraws(t *testing.T) {
	t.Parallel()
	for color, kept := range map[string]bool{
		"good": true, "warning": true, "danger": true, "#e40303": true, "#E40": true, "#e40303cc": true,
		"red": false, "#e4030": false, "#zzzzzz": false, "Good": false, "#e40303; background: red": false,
	} {
		post := &model.Post{}
		post.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{Color: color, Text: "disk full"}})
		got := messageAttachments(post)
		if len(got) != 1 || (got[0].Color == color) != kept || (!kept && got[0].Color != "") {
			t.Errorf("%q: got %+v", color, got)
		}
	}
}

func TestAMessageAttachmentWithOnlyButtonsIsLeftOut(t *testing.T) {
	t.Parallel()
	post := &model.Post{}
	post.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{
		{Actions: []*model.PostAction{{Name: "Acknowledge"}}},
		nil,
		{Title: "Deploy", Fields: []*model.MessageAttachmentField{nil, {Title: "pods", Value: 3}, {Title: "labels", Value: map[string]any{"app": "api"}}, {Title: "note"}}},
	})
	got := messageAttachments(post)
	want := []MessageAttachment{{Title: "Deploy", Fields: []MessageAttachmentField{{Title: "pods", Value: "3"}, {Title: "labels", Value: `{"app":"api"}`}, {Title: "note"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got := messageAttachments(&model.Post{Message: "plain"}); got != nil {
		t.Errorf("a plain post: got %+v", got)
	}
}

func TestAListCutsMessageAttachmentsWithTheMessage(t *testing.T) {
	t.Parallel()
	alert := func(text string) MessageAttachment {
		return MessageAttachment{Color: "danger", Title: "Alert", TitleLink: "https://alerts.example.com/1", Text: text,
			Fields: []MessageAttachmentField{{Title: "status", Value: "firing", Short: true}}}
	}
	long := strings.Repeat("x", 300)

	short := clip([]Post{{Message: "hello", MessageAttachments: []MessageAttachment{alert("disk full")}}})[0]
	if short.MessageAttachmentsLength != 0 || !reflect.DeepEqual(short.MessageAttachments, []MessageAttachment{alert("disk full")}) {
		t.Errorf("a short post was cut: %+v", short)
	}

	// Thirty alerts of 345 characters each: eleven whole, 3,795 characters,
	// and the twelfth's title and link, and its text cut at the 4,000th, with
	// its fields left out.
	many := make([]MessageAttachment, 30)
	for i := range many {
		many[i] = alert(long)
	}
	whole := 30 * (len("Alert") + len("https://alerts.example.com/1") + 300 + len("status") + len("firing"))
	cut := clip([]Post{{MessageAttachments: many}})[0]
	if cut.MessageAttachmentsLength != whole {
		t.Errorf("length %d, want %d", cut.MessageAttachmentsLength, whole)
	}
	// The … that ends what is cut is one character more.
	if shown := attachmentsLength(cut.MessageAttachments); shown != listedMessageRunes+1 {
		t.Errorf("the list shows %d characters of attachments", shown)
	}
	last := cut.MessageAttachments[len(cut.MessageAttachments)-1]
	if len(cut.MessageAttachments) != 12 || !strings.HasSuffix(last.Text, "…") || len(last.Fields) != 0 || last.Color != "danger" || last.TitleLink == "" {
		t.Errorf("%d attachments, the last %+v", len(cut.MessageAttachments), last)
	}
	for i, attachment := range many {
		if !reflect.DeepEqual(attachment, alert(long)) {
			t.Fatalf("clip changed attachment %d of the post it cut: %+v", i, attachment)
		}
	}

	// A message as long as a list shows leaves no room for attachments.
	message := strings.Repeat("m", listedMessageRunes+10)
	full := clip([]Post{{Message: message, MessageAttachments: []MessageAttachment{alert("disk full")}}})[0]
	if full.MessageLength != len(message) || len(full.MessageAttachments) != 0 || full.MessageAttachmentsLength != attachmentsLength([]MessageAttachment{alert("disk full")}) {
		t.Errorf("got %d attachments, length %d", len(full.MessageAttachments), full.MessageAttachmentsLength)
	}

	// A link that does not fit is left out rather than cut to lead elsewhere.
	room := listedMessageRunes - len("Alert") - 10
	linked := clip([]Post{{Message: strings.Repeat("m", room), MessageAttachments: []MessageAttachment{alert("disk full")}}})[0]
	if got := linked.MessageAttachments; len(got) != 1 || got[0].Title != "Alert" || got[0].TitleLink != "" || got[0].Text != "" {
		t.Errorf("got %+v", got)
	}
}

// attachmentsLength is how many characters of text the attachments hold.
func attachmentsLength(attachments []MessageAttachment) int {
	length := 0
	for _, attachment := range attachments {
		for _, text := range attachment.texts() {
			length += utf8.RuneCountInString(*text.text)
		}
	}
	return length
}
