package server

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// A post carries the post, page or picture Mattermost shows below its
// message, and an attachment its pictures and links (mm-mcp#76).

// previewPosts are posts as Mattermost 11.11.1 answered GetPostsForChannel
// with them: one quoting an important post in the same channel, one linking a
// web page, one a picture, and an alert from a webhook, Grafana's shape, with
// its pictures and links.
func previewPosts(t *testing.T) map[string]*model.Post {
	t.Helper()
	data, err := os.ReadFile("testdata/preview-posts.json")
	if err != nil {
		t.Fatal(err)
	}
	var posts map[string]*model.Post
	if err := json.Unmarshal(data, &posts); err != nil {
		t.Fatal(err)
	}
	return posts
}

func TestALinkToAPostCarriesThePostMattermostQuotes(t *testing.T) {
	t.Parallel()
	post := previewPosts(t)["quote"]
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}
	author := &model.User{Id: "y6d8b4ebgbfjjenyatq36utqgy", Username: "sysadmin", FirstName: "Sam", LastName: "Admin"}

	got := shownWith(post, display{priority: true}, reader, author).QuotedPost
	if got == nil {
		t.Fatal("the post quotes nothing")
	}
	want := QuotedPost{
		ID: "us13e1etyib58f3wwd1xicg51c", URL: "https://chat.example.com/rec-priority-2978/pl/us13e1etyib58f3wwd1xicg51c",
		Channel: "Incidents", Author: "sysadmin", AuthorName: "Sam Admin", CreatedAt: "2026-10-29T11:40:52Z",
		Message: "The **retry budget** is 3 attempts per request.", Priority: "important", Files: []Attachment{},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("the quote is %+v\nwant %+v", *got, want)
	}
	if shownWith(post, display{priority: true}, reader, author).LinkPreview != nil {
		t.Error("a quote is a link preview too")
	}

	// A quote is cut as a list cuts a message, wherever it is read.
	long := strings.Repeat("q", listedMessageRunes+5)
	post.Metadata.Embeds[0].Data.(map[string]any)["post"].(map[string]any)["message"] = long
	cut := shownWith(post, display{}, reader, author).QuotedPost
	if cut.MessageLength != len(long) || !strings.HasSuffix(cut.Message, "…") || cut.Priority != "" {
		t.Errorf("a long quote is %d characters, length %d, priority %q", len([]rune(cut.Message)), cut.MessageLength, cut.Priority)
	}
}

func TestALinkToAPageOrAPictureCarriesWhatMattermostShowsOfIt(t *testing.T) {
	t.Parallel()
	posts := previewPosts(t)
	reader := &model.User{Id: "k1a7m3cz5tgn8qxbdwpse9hjzo", Username: "alice"}

	page := shownWith(posts["page"], display{}, reader).LinkPreview
	if page == nil || *page != (LinkPreview{URL: "https://github.com/mattermost/mattermost/releases", SiteName: "GitHub", Title: "Releases · mattermost/mattermost",
		Description: "Mattermost is an open source platform for secure collaboration across the entire software development lifecycle.. - mattermost/mattermost"}) {
		t.Errorf("a page is previewed as %+v", page)
	}
	image := shownWith(posts["image"], display{}, reader).LinkPreview
	if image == nil || *image != (LinkPreview{URL: "https://raw.githubusercontent.com/mattermost/mattermost/master/webapp/channels/src/images/logo.png", Image: true, Width: 465, Height: 91}) {
		t.Errorf("a picture is previewed as %+v", image)
	}

	// Mattermost shows the first embed alone, and a post with attachments has
	// those first.
	alert := posts["grafana"]
	alert.Metadata.Embeds = append(alert.Metadata.Embeds, posts["page"].Metadata.Embeds[0])
	if got := shownWith(alert, display{}, reader); got.LinkPreview != nil || got.QuotedPost != nil {
		t.Errorf("a post with attachments previews %+v %+v", got.LinkPreview, got.QuotedPost)
	}
	// A page Mattermost could read nothing of shows nothing.
	bare := &model.Post{Id: "p1", Metadata: &model.PostMetadata{Embeds: []*model.PostEmbed{{Type: model.PostEmbedOpengraph, URL: "https://example.com", Data: map[string]any{"title": ""}}}}}
	if got := linkPreview(bare); got != nil {
		t.Errorf("an empty page is previewed as %+v", got)
	}
}

func TestAnAttachmentCarriesItsPicturesAndLinksButNotItsIcons(t *testing.T) {
	t.Parallel()
	alert := previewPosts(t)["grafana"]
	got := (display{}).messageAttachments(alert)
	if len(got) != 1 || got[0].ImageURL != "https://grafana.example.com/render/d-solo/api?panelId=4" ||
		got[0].ThumbURL != "https://grafana.example.com/public/img/thumb.png" || got[0].AuthorLink != "https://grafana.example.com/" {
		t.Fatalf("the alert's attachment is %+v", got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "icon") {
		t.Errorf("the attachment carries its icons: %s", data)
	}
	// An attachment of a picture alone, as a chart is posted, is shown.
	chart := &model.Post{}
	chart.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{ImageURL: "https://grafana.example.com/render/1"}})
	if got := (display{}).messageAttachments(chart); len(got) != 1 {
		t.Errorf("a chart alone is %+v", got)
	}
	// A list leaves out a link that does not fit rather than cut it.
	clipped, length := clipAttachments(got, len(got[0].AuthorName)+5)
	if length == 0 || len(clipped) != 1 || clipped[0].AuthorName != "Grafana" || clipped[0].AuthorLink != "" || clipped[0].ImageURL != "" || clipped[0].Title != "" {
		t.Errorf("cut short, the alert's attachment is %+v, of %d", clipped, length)
	}
}
