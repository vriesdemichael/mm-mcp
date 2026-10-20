//go:build live && record

package live

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The documentation's recordings (mm-mcp#38): what each view draws, recorded
// from this checkout's Mattermost as a client that renders views receives
// it, from a scene seeded for it, and made anonymous: its people, team,
// channels and server named as the docs name them. `task docs:record-views`
// runs it. The docs play each recording back to the real view page, and a
// unit test in internal/server holds the recordings to what the tools answer
// today.

// recordingsDir is where the recordings go, from this package.
var recordingsDir = filepath.Join("..", "..", "docs", "site", "assets", "views", "recordings")

// recording is a view's data as the docs' host plays it: the call's input and
// result, the pictures the view reads, and the answers to the presses the
// docs offer, by tool and action.
type recording struct {
	Input     map[string]any             `json:"input"`
	Result    any                        `json:"result"`
	Resources map[string]json.RawMessage `json:"resources,omitempty"`
	Answers   map[string][]any           `json:"answers,omitempty"`
}

// cast is the scene's people: the person the agent works for first.
var cast = []struct{ username, first, last string }{
	{"riley", "Riley", "Chen"},
	{"sam", "Sam", "Okafor"},
	{"alex", "Alex", "Rivera"},
	{"jordan", "Jordan", "Lee"},
	{"priya", "Priya", "Natarajan"},
	{"morgan", "Morgan", "Blake"},
	{"casey", "Casey", "Nguyen"},
	{"taylor", "Taylor", "Brooks"},
}

func TestRecordTheViewsForTheDocs(t *testing.T) {
	admin := admin(t)
	people := map[string]*model.User{}
	var everyone []*model.User
	names := map[string]string{}
	for i, member := range cast {
		user := seedUser(t, admin)
		_, _, err := admin.PatchUser(t.Context(), user.Id, &model.UserPatch{FirstName: &member.first, LastName: &member.last})
		check(t, err)
		_, err = admin.SetProfileImage(t.Context(), user.Id, avatarPicture(i))
		check(t, err)
		people[member.username] = user
		everyone = append(everyone, user)
		names[user.Username] = member.username
	}
	riley, sam, alex, jordan, priya, morgan, casey := people["riley"], people["sam"], people["alex"], people["jordan"], people["priya"], people["morgan"], people["casey"]
	// at mentions someone by their username on this Mattermost, which the
	// recording names as the scene does.
	at := func(name string) string { return "@" + people[name].Username }
	as := map[string]*model.Client4{}
	for name, user := range people {
		as[name] = clientAs(t, user)
	}

	team := seedTeam(t, admin, everyone...)
	acme := "Acme"
	_, _, err := admin.PatchTeam(t.Context(), team.Id, &model.TeamPatch{DisplayName: &acme})
	check(t, err)
	names[team.Name] = "acme"
	// A channel of the scene, named as it shows, without the messages that
	// adding its members leaves, which would name the stack's administrator.
	channel := func(slug, display string, members ...*model.User) *model.Channel {
		name := uniqueName("channel")
		made, _, err := admin.CreateChannel(t.Context(), &model.Channel{TeamId: team.Id, Name: name, DisplayName: display, Type: model.ChannelTypeOpen})
		check(t, err)
		for _, member := range members {
			_, _, err := admin.AddChannelMember(t.Context(), made.Id, member.Id)
			check(t, err)
		}
		list, _, err := admin.GetPostsForChannel(t.Context(), made.Id, 0, 200, "", false, false)
		check(t, err)
		for _, post := range list.Posts {
			if post.Type != "" {
				_, err := admin.DeletePost(t.Context(), post.Id)
				check(t, err)
			}
		}
		names[made.Name] = slug
		return made
	}
	planning := channel("release-planning", "Release planning", everyone...)
	design := channel("design", "Design", riley, priya, morgan, casey)

	me, _, err := admin.GetMe(t.Context(), "")
	check(t, err)
	emojiName := uniqueName("emoji")
	_, _, err = admin.CreateEmoji(t.Context(), &model.Emoji{Name: emojiName, CreatorId: me.Id}, emojiPicture(), "shipit.png")
	check(t, err)
	names[emojiName] = "shipit"

	// Riley has read both channels; what comes after is unread for them.
	for _, place := range []*model.Channel{planning, design} {
		_, _, err := as["riley"].ViewChannel(t.Context(), riley.Id, &model.ChannelView{ChannelId: place.Id})
		check(t, err)
	}
	question := postAs(t, as["riley"], design.Id, "", "Should the 2.4 banner still say beta?")

	root := postAs(t, as["sam"], planning.Id, "", "The **2.4 release** moves to Thursday :tada: The notes are in [the release doc](https://docs.example.com/release-2.4). Anything blocking, say so here.")
	_, err = admin.PinPost(t.Context(), root.Id)
	check(t, err)
	for _, reaction := range []struct {
		who   *model.User
		emoji string
	}{{alex, "tada"}, {jordan, "tada"}, {priya, emojiName}, {sam, "eyes"}} {
		_, _, err := as[names[reaction.who.Username]].SaveReaction(t.Context(), &model.Reaction{UserId: reaction.who.Id, PostId: root.Id, EmojiName: reaction.emoji})
		check(t, err)
	}
	uploaded, _, err := as["alex"].UploadFile(t.Context(), chartPicture(), planning.Id, "load-test.png")
	check(t, err)
	_, _, err = as["alex"].CreatePost(t.Context(), &model.Post{ChannelId: planning.Id, RootId: root.Id,
		Message: "Last night's load test: p95 stayed under 200 ms :chart_with_upwards_trend:", FileIds: []string{uploaded.FileInfos[0].Id}})
	check(t, err)
	postAs(t, as["jordan"], planning.Id, root.Id, "Looks good to me. "+at("riley")+", is the migration done?")
	postAs(t, as["priya"], planning.Id, root.Id, "Docs for 2.4 are merged, and ~design has the new screenshots.")
	postAs(t, as["sam"], planning.Id, root.Id, "Thanks all, I'll cut the release candidate tomorrow morning.")
	postAs(t, as["priya"], design.Id, "", "New screenshots for 2.4 are up :frame_with_picture:")
	postAs(t, as["morgan"], design.Id, "", "Love the dark mode ones.")
	postAs(t, as["casey"], design.Id, question.Id, "Yes, until the docs land on Thursday.")

	session := proposingSession(t, admin, riley, true)
	call := func(name string, arguments map[string]any) *mcp.CallToolResult {
		t.Helper()
		result := callTool(t, session, &mcp.CallToolParams{Name: name, Arguments: arguments})
		if result.IsError {
			t.Fatalf("%s failed: %s", name, errorText(result))
		}
		return result
	}
	var saved []string

	// A thread, its first page, and the rest on a press.
	threadInput := map[string]any{"kind": "thread", "post_id": root.Id, "limit": 3}
	thread := call("show", threadInput)
	var firstPage struct {
		NextCursor string `json:"next_cursor"`
	}
	structured(t, thread, &firstPage)
	more := call("view_action", map[string]any{"action": "more", "post_id": root.Id, "cursor": firstPage.NextCursor, "limit": 30})
	saved = append(saved, write(t, session, "thread", recording{Input: threadInput, Result: thread,
		Answers: map[string][]any{"view_action:more": {more}}}, people, names))

	// What is unread, a channel opened in place, and marked read.
	unreadsInput := map[string]any{"kind": "unreads"}
	unreads := call("show", unreadsInput)
	opened := call("view_action", map[string]any{"action": "unread", "channel_id": design.Id, "limit": 30})
	marked := call("view_change", map[string]any{"action": "mark_read", "channel_id": design.Id})
	saved = append(saved, write(t, session, "unreads", recording{Input: unreadsInput, Result: unreads,
		Answers: map[string][]any{"view_action:unread": {opened}, "view_change:mark_read": {marked}}}, people, names))

	// A post as posted.
	postedInput := map[string]any{"channel_id": planning.Id, "message": "Release candidate 2.4.0-rc1 is building now :rocket:", "reaches": members(t, planning.Id)}
	saved = append(saved, write(t, session, "posted", recording{Input: postedInput, Result: call("create_post", postedInput)}, people, names))

	// A proposal, posted on the press.
	proposalText := "Thanks " + at("jordan") + ", the migration finished at 09:40. The steps are in [the runbook](https://docs.example.com/runbook-2.4)."
	proposalInput := map[string]any{"root_id": root.Id, "message": proposalText}
	proposal := call("propose_post", proposalInput)
	posted := call("view_change", map[string]any{"action": "post", "channel_id": planning.Id, "root_id": root.Id, "shown": proposalText})
	saved = append(saved, write(t, session, "proposal", recording{Input: proposalInput, Result: proposal,
		Answers: map[string][]any{"view_change:post": {posted}}}, people, names))

	// A proposal that alerts the whole channel, confirmed as Mattermost asks.
	loudText := "@channel the 2.4 release moves to Thursday. Please hold merges to main from Wednesday noon."
	loudInput := map[string]any{"channel_id": planning.Id, "message": loudText}
	loud := call("propose_post", loudInput)
	confirm := call("view_change", map[string]any{"action": "post", "channel_id": planning.Id, "shown": loudText})
	loudPosted := call("view_change", map[string]any{"action": "post", "channel_id": planning.Id, "shown": loudText, "confirmed": true})
	saved = append(saved, write(t, session, "confirm", recording{Input: loudInput, Result: loud,
		Answers: map[string][]any{"view_change:post": {confirm, loudPosted}}}, people, names))

	// The person's own draft in the way: as a revision of it, and beside it.
	for _, choice := range []struct {
		name, theirs, proposed string
		with                   *model.User
		revises                bool
	}{
		{"revision", "Thursday works", "Thursday works for me. I'll have the notes ready by noon.", sam, true},
		{"choice", "Can we move the retro?", "Can we move the retro to Friday at 15:00? Thursday is the release.", alex, false},
	} {
		direct, _, err := as["riley"].CreateDirectChannel(t.Context(), riley.Id, choice.with.Id)
		check(t, err)
		_, _, err = as["riley"].UpsertDraft(t.Context(), &model.Draft{UserId: riley.Id, ChannelId: direct.Id, Message: choice.theirs})
		check(t, err)
		input := map[string]any{"channel_id": direct.Id, "message": choice.proposed}
		if choice.revises {
			input["revises_draft"] = true
		}
		proposed := call("propose_post", input)
		replaced := call("view_change", map[string]any{"action": "replace", "channel_id": direct.Id, "shown": choice.theirs, "message": choice.proposed})
		posted := call("view_change", map[string]any{"action": "post", "channel_id": direct.Id, "shown": choice.proposed})
		saved = append(saved, write(t, session, choice.name, recording{Input: input, Result: proposed,
			Answers: map[string][]any{"view_change:replace": {replaced}, "view_change:post": {posted}}}, people, names))
	}

	// What the preview makes visible: where a link leads, and a character
	// that turns the text's direction, so the name of an executable reads as a PDF's.
	trickText := "Here is the signed invoice, invoice_\u202efdp.exe\u202c, and the [payment page](https://pay.example.net/acme-invoices)."
	trickInput := map[string]any{"channel_id": design.Id, "message": trickText}
	saved = append(saved, write(t, session, "preview", recording{Input: trickInput, Result: call("propose_post", trickInput)}, people, names))
	t.Logf("wrote %s", strings.Join(saved, ", "))
}

// resourceAddress is an address of a picture a view reads, in a recording.
var resourceAddress = regexp.MustCompile(`mattermost://[^"\s\\]+/(?:files/[a-z0-9]+(?:/preview)?|emoji/[A-Za-z0-9_+-]+)`)

// write reads the pictures a recording's view draws, makes it anonymous and
// writes it under name, answering with the file's name.
func write(t *testing.T, session *mcp.ClientSession, name string, r recording, people map[string]*model.User, names map[string]string) string {
	t.Helper()
	// A result as the host plays it to the view: its content and structured
	// content; what the server says of itself and of the protocol is no part
	// of what the view draws.
	r.Result = played(t, r.Result)
	for press, answers := range r.Answers {
		for i, answer := range answers {
			if result, ok := answer.(*mcp.CallToolResult); ok {
				r.Answers[press][i] = played(t, result)
			}
		}
	}
	raw, err := json.Marshal(r)
	check(t, err)
	base := "mattermost://" + hostOf(t, liveURL)
	wanted := map[string]bool{}
	for _, address := range resourceAddress.FindAllString(string(raw), -1) {
		wanted[address] = true
	}
	for _, user := range people {
		if strings.Contains(string(raw), user.Id) {
			wanted[base+"/users/"+user.Id+"/image"] = true
		}
	}
	for original := range names {
		if strings.HasPrefix(original, "lt-emoji-") && strings.Contains(string(raw), original) {
			wanted[base+"/emoji/"+original] = true
		}
	}
	r.Resources = map[string]json.RawMessage{}
	for address := range wanted {
		read, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: address})
		if err != nil || len(read.Contents) == 0 {
			continue
		}
		content := read.Contents[0]
		picture, err := json.Marshal(map[string]any{"mimeType": content.MIMEType, "blob": content.Blob})
		check(t, err)
		r.Resources[address] = picture
	}
	raw, err = json.MarshalIndent(r, "", "  ")
	check(t, err)
	text := anonymous(string(raw), names)
	// A character that turns the text's direction is written as an escape,
	// so no file of the repository hides one.
	text = strings.NewReplacer("\u202e", `\u202e`, "\u202c", `\u202c`).Replace(text)
	check(t, os.MkdirAll(recordingsDir, 0o755))
	file := filepath.Join(recordingsDir, name+".json")
	check(t, os.WriteFile(file, []byte(text+"\n"), 0o644))
	return filepath.Base(file)
}

// played is a tool's result as the host plays it to the view: its content
// and its structured content.
func played(t *testing.T, result any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(result)
	check(t, err)
	var whole map[string]any
	check(t, json.Unmarshal(raw, &whole))
	kept := map[string]any{"content": whole["content"]}
	if structured, ok := whole["structuredContent"]; ok {
		kept["structuredContent"] = structured
	}
	return kept
}

// anonymous names the scene as the docs do: the server chat.example.com, and
// each person, team, channel and emoji by its name in the scene, longest
// first, so no name is replaced inside another.
func anonymous(text string, names map[string]string) string {
	host := hostOf(nil, liveURL)
	text = strings.ReplaceAll(text, "http://"+host, "https://chat.example.com")
	text = strings.ReplaceAll(text, host, "chat.example.com")
	from := make([]string, 0, len(names))
	for name := range names {
		from = append(from, name)
	}
	sort.Slice(from, func(i, j int) bool { return len(from[i]) > len(from[j]) })
	for _, name := range from {
		text = strings.ReplaceAll(text, name, names[name])
	}
	return text
}

func hostOf(t *testing.T, address string) string {
	parsed, err := url.Parse(address)
	if err != nil && t != nil {
		t.Fatal(err)
	}
	return parsed.Host
}

// avatarPicture is a person's picture: a head and shoulders, light on a
// colour of their own.
func avatarPicture(i int) []byte {
	backgrounds := []color.RGBA{{0x2f, 0x6f, 0xd1, 255}, {0xc2, 0x4e, 0x2a, 255}, {0x2e, 0x8b, 0x6a, 255}, {0x7a, 0x4f, 0xc9, 255},
		{0xb8, 0x86, 0x0b, 255}, {0x1f, 0x7a, 0x8c, 255}, {0xa8, 0x3a, 0x6e, 255}, {0x4a, 0x5a, 0x6e, 255}}
	background := backgrounds[i%len(backgrounds)]
	figure := color.RGBA{0xf4, 0xf1, 0xea, 255}
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx, dy := float64(x-64), float64(y-52)
			sx, sy := float64(x-64)/46, float64(y-132)/42
			switch {
			case dx*dx+dy*dy <= 22*22, sx*sx+sy*sy <= 1:
				img.SetRGBA(x, y, figure)
			default:
				img.SetRGBA(x, y, background)
			}
		}
	}
	return encodePNG(img)
}

// emojiPicture is the custom emoji: an arrow pointing up on an orange disc.
func emojiPicture() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			dx, dy := float64(x)-31.5, float64(y)-31.5
			inDisc := dx*dx+dy*dy <= 30*30
			head := y >= 14 && y <= 34 && abs(x-32) <= (y-14)
			shaft := y > 34 && y <= 50 && abs(x-32) <= 6
			switch {
			case inDisc && (head || shaft):
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			case inDisc:
				img.SetRGBA(x, y, color.RGBA{0xf0, 0x8a, 0x24, 255})
			}
		}
	}
	return encodePNG(img)
}

// chartPicture is the load test's chart: a week of p95 latencies as bars,
// under the 200 ms line they stayed beneath.
func chartPicture() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 640, 320))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	fill(0, 0, 640, 320, color.RGBA{255, 255, 255, 255})
	for y := 40; y <= 280; y += 40 {
		fill(40, y, 620, y+1, color.RGBA{0xe3, 0xe4, 0xe8, 255})
	}
	for i, height := range []int{150, 172, 138, 165, 181, 142, 156} {
		x := 60 + i*80
		fill(x, 280-height, x+44, 280, color.RGBA{0x2f, 0x6f, 0xd1, 255})
	}
	fill(40, 280-200, 620, 280-198, color.RGBA{0xd2, 0x4b, 0x4e, 255})
	fill(40, 280, 620, 282, color.RGBA{0x6b, 0x6e, 0x78, 255})
	return encodePNG(img)
}

func encodePNG(img image.Image) []byte {
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
