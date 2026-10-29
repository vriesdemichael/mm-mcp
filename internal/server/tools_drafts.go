package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// Drafts: the model proposes a post as the person's draft, in their own
// message box, and the person sends it (ADR-035), from Mattermost or from the
// proposal's view. A proposal is the user's alone and never asks, and never
// replaces a draft the person wrote; deleting one loses words the person may
// have written, so it states how many and asks where mm-mcp is set to
// (ADR-031, ADR-033).

// Draft is a draft in the user's message box.
type Draft struct {
	ChannelID  string   `json:"channel_id"`
	Channel    string   `json:"channel" jsonschema:"the channel's display name; a direct message is named after the person on the other side"`
	ChannelURL string   `json:"channel_url,omitempty" jsonschema:"the channel's link, which opens it in Mattermost"`
	Team       string   `json:"team,omitempty"`
	RootID     string   `json:"root_id,omitempty" jsonschema:"the thread the draft replies in, if it does"`
	Address    string   `json:"address,omitempty" jsonschema:"where the draft waits, as delete_draft takes it: its channel's address, or for a reply its thread's post's"`
	URL        string   `json:"url,omitempty" jsonschema:"where the draft waits, as a link that opens it in Mattermost: its channel, or for a reply its thread"`
	Message    string   `json:"message"`
	Length     int      `json:"length" jsonschema:"how many characters the draft holds, as Mattermost counts a message's length: what delete_draft states as discards"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
	Notes      []string `json:"notes,omitempty" jsonschema:"what the message's mentions will do once sent"`
}

// draftTarget is where a draft goes: a channel, or a thread in it.
type draftTarget struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"the channel, as get_user_channels or get_channel_info gives it"`
	RootID    string `json:"root_id,omitempty" jsonschema:"the thread instead: any post in it"`
}

// draftsUses are the operations that read the user's drafts where one goes.
func draftsUses() []Use {
	return []Use{
		{
			Operation: "GetDrafts",
			Params: map[string]Coverage{
				"user_id": Fixed("me", "a user's drafts are theirs"),
				"team_id": Fixed("the channel's team", "Mattermost files a draft under its channel's team, and a direct message's under each of the user's teams"),
			},
			Releases: "11.7 serves GET /api/v4/users/{user_id}/teams/{team_id}/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
		},
		{
			Operation: "GetPost",
			Params: map[string]Coverage{
				"post_id":         SetBy("root_id"),
				"include_deleted": Omitted("a deleted post has no thread to reply in"),
			},
		},
		{Operation: "GetChannel", Params: map[string]Coverage{"channel_id": SetBy("channel_id")}},
		{
			Operation: "GetTeamsForUser",
			Params:    map[string]Coverage{"user_id": Fixed("me", "a direct message belongs to no team, and its drafts are filed under the user's")},
		},
		{
			Operation: "GetUser",
			Params:    map[string]Coverage{"user_id": Fixed("me", "a user's drafts are theirs")},
		},
	}
}

// existingDraft is the user's draft in a channel or thread, or nil.
func existingDraft(ctx context.Context, client *mattermost.Client, userID, teamID, channelID, rootID string) (*model.Draft, error) {
	drafts, err := client.Drafts(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}
	for _, draft := range drafts {
		if draft.ChannelId == channelID && draft.RootId == rootID && (strings.TrimSpace(draft.Message) != "" || len(draft.FileIds) > 0) {
			return draft, nil
		}
	}
	return nil, nil
}

// onlyText reports whether a draft is text alone, with no files, priority,
// kind of post or properties a person set in Mattermost: a proposal they
// added any of these to is theirs. The mark a proposal carries is mm-mcp's
// own, and what Mattermost reads from the text, such as a link's preview, is
// the text's.
func onlyText(draft *model.Draft) bool {
	for key := range draft.GetProps() {
		if key != proposalKey {
			return false
		}
	}
	if metadata := draft.Metadata; metadata != nil && (len(metadata.Files) > 0 || metadata.Priority != nil || len(metadata.Acknowledgements) > 0) {
		return false
	}
	return len(draft.FileIds) == 0 && len(draft.Priority) == 0 && draft.Type == ""
}

// draftsSync refuses a draft the person would never see: one on a server that
// keeps no drafts, or for a person who turned off syncing their drafts, whose
// clients keep their own. GetClientConfig, GetPreferencesByCategory.
func draftsSync(ctx context.Context, client *mattermost.Client, userID string) error {
	config, err := client.ClientConfig(ctx)
	if err != nil {
		return err
	}
	if config["AllowSyncedDrafts"] == "false" {
		return fmt.Errorf("this server does not keep drafts for its users' clients, so a draft would reach none of them; post with create_post instead, or give the person the text")
	}
	preference, err := client.Preference(ctx, userID, model.PreferenceCategoryAdvancedSettings, "sync_drafts")
	if err != nil {
		return err
	}
	if preference != nil && preference.Value == "false" {
		return fmt.Errorf("the person turned off syncing their drafts in Mattermost's settings, so a draft saved here would not reach their clients; give them the text instead, or post with create_post")
	}
	return nil
}

// proposalKey is the property a draft mm-mcp proposed carries: a mark of the
// text it proposed, so the draft is known for mm-mcp's own until the person
// changes it, and then for theirs (ADR-035). Mattermost's own client sends a
// draft's properties with the post, where nothing shows this one. A proposal
// carries no AI marker: what the person sends from Mattermost is theirs
// (ADR-031).
const proposalKey = "mm_mcp_proposal"

// proposalMark is the mark of a proposed text.
func proposalMark(message string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(message)))
	return hex.EncodeToString(sum[:8])
}

// proposed reports whether a draft is mm-mcp's own proposal, unchanged.
func proposed(draft *model.Draft) bool {
	mark, _ := draft.GetProps()[proposalKey].(string)
	return mark != "" && mark == proposalMark(draft.Message)
}

// editedMark is the mark of a text the person wrote in a proposal's view.
// The draft is theirs, which no proposal replaces, and still the view's,
// which the view's presses reach until the person types on in Mattermost.
func editedMark(message string) string {
	return "edited " + proposalMark(message)
}

// fromView reports whether a draft holds, unchanged, what a proposal's view
// showed: mm-mcp's proposal, or the person's text from the view's editor. A
// press acts on nothing else; a draft the person wrote in Mattermost is sent
// from there.
func fromView(draft *model.Draft) bool {
	mark, _ := draft.GetProps()[proposalKey].(string)
	return mark != "" && (mark == proposalMark(draft.Message) || mark == editedMark(draft.Message))
}

// slashCommand refuses a proposal Mattermost would run as a slash command
// when the person sends it from their message box: one that starts with a
// slash, which reads as a message in the proposal and acts as a command.
func slashCommand(tool, message string) error {
	if strings.HasPrefix(strings.TrimLeft(message, " \t\r\n"), "/") {
		return fmt.Errorf("%s did not run: a message that starts with / runs as a slash command when the person sends it from Mattermost; "+
			"begin it with other words, or post it as text with create_post", tool)
	}
	return nil
}

type proposePostInput struct {
	draftTarget
	Message      string `json:"message" jsonschema:"the post to propose, in Mattermost Markdown, as it would be sent"`
	RevisesDraft bool   `json:"revises_draft,omitempty" jsonschema:"set when the person asked you to change the draft they have there, which list_drafts reads: where the client renders views, the view shows your text as a revision of theirs, for them to apply or not. It replaces nothing itself"`
}

// Proposal is what propose_post answers: the draft as it waits for the
// person, or, where a draft of theirs is in the way and the client renders
// the proposal's view, the proposal and their draft, for them to choose
// between in the view.
type Proposal struct {
	Draft
	Saved  bool   `json:"saved" jsonschema:"whether the proposal waits as the person's draft; false while a draft of their own is in the way, which the view asks them about"`
	Theirs *Draft `json:"theirs,omitempty" jsonschema:"the person's own draft where the proposal would go, left as it is, when the proposal was not saved"`
}

// Notifies is whom a message alerts at once, as Mattermost's web app tells
// its sender before it sends the message.
type Notifies struct {
	People    int64    `json:"people" jsonschema:"how many people it alerts"`
	Mentions  []string `json:"mentions" jsonschema:"the mentions that alert them, as the message has them"`
	Timezones int      `json:"timezones,omitempty" jsonschema:"how many timezones they are in, for a mention of the whole channel, as Mattermost counts them"`
}

// notifiesOf is whom a message alerts at once, where Mattermost's web app has
// its sender confirm it, or nil: its decision and its counts, the timezones of
// the channel's members for a mention of all of them (use_submit.tsx).
// GetChannelMembersTimezones.
func notifiesOf(ctx context.Context, client *mattermost.Client, channelID string, loud loudness, specials bool) (*Notifies, error) {
	if loud.count == 0 {
		return nil, nil
	}
	out := &Notifies{People: loud.count, Mentions: loud.mentions}
	if specials {
		zones, err := client.ChannelTimezones(ctx, channelID)
		if err != nil {
			return nil, err
		}
		out.Timezones = len(zones)
	}
	return out, nil
}

// notifiesUses are the operations notifiesOf calls, and loudnessIn before it.
func notifiesUses() []Use {
	return append(impactUses(), Use{
		Operation: "GetChannelMembersTimezones",
		Params:    map[string]Coverage{"channel_id": Fixed("the draft's channel", "Mattermost's confirmation names the timezones of the members a mention of the whole channel alerts")},
	})
}

// proposedShown is what propose_post tells the model it showed the person,
// in a client that renders the proposal's view.
func proposedShown(out Proposal) string {
	if !out.Saved {
		return "Not saved: the person has a draft of their own in " + out.Channel + ". The view shows it beside your proposal, and asks " +
			"them which to keep. Learn what they chose from Mattermost, with list_drafts, rather than proposing it again."
	}
	return "Proposed as the person's draft in " + out.Channel + ", and shown to them as it will be posted: they post, change or " +
		"discard it in the view, or in Mattermost. Learn what they did from Mattermost, with list_drafts or by reading the " +
		"channel, rather than proposing it again; do not repeat the message unless they ask."
}

func proposePostSpec() Spec {
	return unasked(shaping(toolSpec(
		&mcp.Tool{
			Name: "propose_post",
			Description: "Propose a post for the person to send themselves: the message waits as their draft in a channel or a thread, " +
				"in their message box in every Mattermost client, for them to read, change and send. Nothing is sent, and only the " +
				"person sees it. Use it whenever they should have the last word on the text, and where their MCP client does not let " +
				"a post through. A direct or group message is proposed in by its channel, as get_user_channels gives it; one that " +
				"does not exist yet is not started here, since starting one shows it to the other people at once. mm-mcp's own " +
				"proposal there, unchanged, is replaced; a draft of the person's never is. In a client that renders MCP Apps views, " +
				"the person sees the proposal as it will be posted, and posts, changes or discards it there; where a draft of " +
				"theirs is in the way, the view asks them which to keep. The message is checked as a post's is, and one that starts with / is " +
				"refused, since Mattermost runs it as a slash command when the person sends it. It answers with " +
				"the draft and the link where it waits.",
			Annotations: personal("Propose post"),
			Meta:        viewToolMeta(),
		},
		uses([]Use{
			{
				Operation: "UpsertDraft",
				Params: map[string]Coverage{
					"body.channel_id": SetBy("channel_id"),
					"body.root_id":    SetBy("root_id"),
					"body.message":    SetBy("message"),
					"body.file_ids":   Omitted("a proposal carries text; the person attaches files in Mattermost"),
					"body.priority":   Omitted("a message's priority is the person's to set when they send it"),
					"body.props": Fixed(`{"mm_mcp_proposal": a mark of the message}`,
						"marks the draft as mm-mcp's proposal, so it is replaced only while unchanged; no AI marker, since what the person sends from Mattermost is theirs"),
					"body.type": Omitted("a draft is an ordinary message, which is the default"),
				},
				Releases: "11.7 serves POST /api/v4/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
			},
			{
				Operation: "GetPreferencesByCategory",
				Params: map[string]Coverage{
					"user_id":  Fixed("me", "whether drafts sync is the user's own setting"),
					"category": Fixed("advanced_settings", "where Mattermost keeps sync_drafts, the setting that keeps drafts on the server"),
				},
			},
			{
				Operation: "GetChannelStats",
				Params:    map[string]Coverage{"channel_id": Fixed("the draft's channel", "the notes say how many people @channel, @all and @here would reach")},
			},
		}, draftsUses(), messageUses(false), describeUses(true)),
		func(clientFor ClientFor) mcp.ToolHandlerFor[proposePostInput, Proposal] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input proposePostInput) (*mcp.CallToolResult, Proposal, error) {
				if err := slashCommand("propose_post", input.Message); err != nil {
					return nil, Proposal{}, err
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, Proposal{}, err
				}
				channelID, rootID, err := threadOf(ctx, client, input.ChannelID, input.RootID)
				if err != nil {
					return nil, Proposal{}, err
				}
				self, err := client.Me(ctx)
				if err != nil {
					return nil, Proposal{}, err
				}
				if err := draftsSync(ctx, client, self.Id); err != nil {
					return nil, Proposal{}, err
				}
				channel, err := client.Channel(ctx, channelID)
				if err != nil {
					return nil, Proposal{}, err
				}
				groups, err := groupNames(ctx, client, []*model.Channel{channel}, self.Id)
				if err != nil {
					return nil, Proposal{}, err
				}
				checked, err := checkMessage(ctx, client, input.Message, place(channel, self.Id, nil, groups[channel.Id]), func(ctx context.Context) (int64, error) {
					stats, err := client.ChannelStats(ctx, channelID)
					if err != nil {
						return 0, err
					}
					return stats.MemberCount, nil
				})
				if err != nil {
					return nil, Proposal{}, err
				}
				teamID, err := teamOf(ctx, client, channelID)
				if err != nil {
					return nil, Proposal{}, err
				}
				// A draft of the person's is their unsent words, and maybe files,
				// which a proposal never replaces; deleting it is delete_draft's,
				// which states what goes, and in the proposal's view the person's
				// own press. mm-mcp's own proposal, unchanged, is mm-mcp's to
				// replace. The same text is there already, and is left as it is
				// rather than written again, which would drop its files; where
				// the person wrote it, the view asks them, since its presses
				// reach only what a view put there. It takes its turn with the
				// presses on the draft, so that one posting the draft before it
				// does not delete it.
				release, err := holdDraft(ctx, self.Id, channelID, rootID)
				if err != nil {
					return nil, Proposal{}, err
				}
				defer release()
				there, err := existingDraft(ctx, client, self.Id, teamID, channelID, rootID)
				if err != nil {
					return nil, Proposal{}, err
				}
				views := rendersViews(request)
				write, theirs := false, (*model.Draft)(nil)
				switch {
				case there == nil, there.Message != input.Message && proposed(there) && onlyText(there):
					write = true
				case there.Message == input.Message && (fromView(there) || !views):
				case views:
					theirs = there
				default:
					held := ""
					if files := int64(len(there.FileIds)); files > 0 {
						held = fmt.Sprintf(", with %d %s", files, plural(files, "file", "files"))
					}
					return nil, Proposal{}, fmt.Errorf("propose_post did not run: the person has a draft there, which was left as it is: “%s”%s. "+
						"If they asked you to change it, delete it with delete_draft, stating discards: %d, and propose again. If not, "+
						"they can finish or clear it in Mattermost, or post with create_post", excerpt(there.Message), held, utf8.RuneCountInString(there.Message))
				}
				drafts := []*model.Draft{there}
				switch {
				case write:
					props := model.StringInterface{proposalKey: proposalMark(input.Message)}
					if drafts[0], err = client.Draft(ctx, self.Id, channelID, rootID, input.Message, props); err != nil {
						return nil, Proposal{}, err
					}
				case theirs != nil:
					// Nothing is written: the person chooses in the view.
					drafts = []*model.Draft{{UserId: self.Id, ChannelId: channelID, RootId: rootID, Message: input.Message}, theirs}
				}
				described, err := describeDrafts(ctx, client, drafts)
				if err != nil {
					return nil, Proposal{}, err
				}
				out := Proposal{Draft: described[0], Saved: theirs == nil}
				out.Notes = checked.notes
				if theirs != nil {
					out.Theirs = &described[1]
				}
				if !views {
					return nil, out, nil
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: proposedShown(out)}}}, out, nil
			}
		},
	), map[string]string{"revises_draft": "says the proposal revises the person's own draft there, which the view shows as a revision; it sets no parameter"}),
		"what it writes is the person's own draft, which only they see and only they send, and it never replaces one they wrote")
}

// describeDrafts is drafts with their channels and teams named, as posts'
// are. GetUser, GetChannel, GetTeam, GetUsersByIds.
func describeDrafts(ctx context.Context, client *mattermost.Client, drafts []*model.Draft) ([]Draft, error) {
	posts := make([]*model.Post, 0, len(drafts))
	for _, draft := range drafts {
		posts = append(posts, &model.Post{Id: draft.RootId, ChannelId: draft.ChannelId, UserId: draft.UserId})
	}
	described, zone, err := describe(ctx, client, posts)
	if err != nil {
		return nil, err
	}
	out := make([]Draft, 0, len(drafts))
	for i, draft := range drafts {
		out = append(out, Draft{
			ChannelID: draft.ChannelId, Channel: described[i].Channel, ChannelURL: described[i].ChannelURL, Team: described[i].Team,
			RootID: draft.RootId, Message: draft.Message, Length: utf8.RuneCountInString(draft.Message), UpdatedAt: timestamp(draft.UpdateAt, zone),
			Address: draftAddress(asAddress(described[i].ChannelURL), draft.RootId),
			URL:     described[i].ChannelURL,
		})
		if draft.RootId != "" {
			out[i].URL = described[i].URL
		}
	}
	return out, nil
}

// Drafts is a page of the user's drafts.
type Drafts struct {
	Drafts []Draft `json:"drafts" jsonschema:"most recently changed first"`
	pageInfo
}

// The most drafts one call returns, and how many when not told.
const (
	maxDrafts     = 100
	defaultDrafts = 20
)

type listDraftsInput struct {
	TeamID string `json:"team_id,omitempty" jsonschema:"only this team's drafts, with those in direct and group messages; every team's when not given"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many drafts a page holds, at most 100; 20 when not given"`
	pageArgs
}

func listDraftsSpec() Spec {
	return shaping(toolSpec(
		&mcp.Tool{
			Name:        "list_drafts",
			Description: "The drafts in the user's message boxes, in channels and threads, with where each is: what they have started and not sent.",
			Annotations: readOnly("List drafts"),
		},
		uses([]Use{
			{
				Operation: "GetDrafts",
				Params: map[string]Coverage{
					"user_id": Fixed("me", "a user's drafts are theirs"),
					"team_id": SetBy("team_id"),
				},
				Releases: "11.7 serves GET /api/v4/users/{user_id}/teams/{team_id}/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
			},
			{
				Operation: "GetTeamsForUser",
				Params:    map[string]Coverage{"user_id": Fixed("me", "without team_id, every team the user belongs to is read")},
			},
		}, describeUses(true)),
		func(clientFor ClientFor) mcp.ToolHandlerFor[listDraftsInput, Drafts] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input listDraftsInput) (*mcp.CallToolResult, Drafts, error) {
				limit, err := limitOf(input.Limit, defaultDrafts, maxDrafts)
				if err != nil {
					return nil, Drafts{}, err
				}
				at, err := openCursor("list_drafts", input, input.Cursor)
				if err != nil {
					return nil, Drafts{}, err
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, Drafts{}, err
				}
				self, err := client.Me(ctx)
				if err != nil {
					return nil, Drafts{}, err
				}
				teams := []string{input.TeamID}
				if input.TeamID == "" {
					all, err := client.Teams(ctx)
					if err != nil {
						return nil, Drafts{}, err
					}
					teams = teams[:0]
					for _, team := range all {
						teams = append(teams, team.Id)
					}
				}
				// A direct message's draft is listed under every team; keep it once.
				seen := map[string]bool{}
				var drafts []*model.Draft
				for _, team := range teams {
					found, err := client.Drafts(ctx, self.Id, team)
					if err != nil {
						return nil, Drafts{}, err
					}
					for _, draft := range found {
						if key := draft.ChannelId + "/" + draft.RootId; !seen[key] && strings.TrimSpace(draft.Message) != "" {
							seen[key] = true
							drafts = append(drafts, draft)
						}
					}
				}
				slices.SortStableFunc(drafts, func(a, b *model.Draft) int { return int(b.UpdateAt - a.UpdateAt) })
				page, next := offsetPage(drafts, at, limit)
				described, err := describeDrafts(ctx, client, page)
				if err != nil {
					return nil, Drafts{}, err
				}
				return nil, Drafts{Drafts: described, pageInfo: pageInfo{NextCursor: next}}, nil
			}
		},
	), pagingShapes)
}

// deleteDraftInput is the draft to delete, and how much of the person's text
// goes with it.
type deleteDraftInput struct {
	draftTarget
	discardField
}

// draftAddress is where a draft waits: its channel's address, or for a reply
// the address of the post its thread starts with.
func draftAddress(channelAddress, rootID string) string {
	if rootID == "" {
		return channelAddress
	}
	return postAddress(channelAddress, rootID)
}

// draftPlace names where a draft waits, as a question names it.
func draftPlace(described Draft, draft *model.Draft) string {
	where := oneLine(described.Channel)
	if described.Team != "" {
		where += " in " + oneLine(described.Team)
	}
	if draft.RootId != "" {
		where = "a thread in " + where
	}
	return where
}

func deleteDraftSpec() Spec {
	return asked(checking(toolSpec(
		&mcp.Tool{
			Name: "delete_draft",
			Description: "Delete the user's draft in a channel or a thread. A draft may hold words the person wrote and has not sent, " +
				"so every call states discards, how many characters the draft holds, checked against mm-mcp's own count just " +
				"before it is deleted; read it with list_drafts first.",
			Annotations: overwrites("Delete draft"),
		},
		uses([]Use{
			{
				Operation: "DeleteDraft",
				Params: map[string]Coverage{
					"user_id":    Fixed("me", "a user's drafts are theirs"),
					"channel_id": SetBy("channel_id"),
				},
				Releases: "11.7 serves DELETE /api/v4/users/{user_id}/channels/{channel_id}/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
			},
			{
				Operation: "DeleteDraftForThread",
				Params: map[string]Coverage{
					"user_id":    Fixed("me", "a user's drafts are theirs"),
					"channel_id": SetBy("channel_id"),
					"thread_id":  SetBy("root_id"),
				},
				Releases: "11.7 serves DELETE /api/v4/users/{user_id}/channels/{channel_id}/drafts/{thread_id}, as its router shows, though its specification leaves it out; nothing differs in use",
			},
		}, draftsUses(), describeUses(true)),
		func(clientFor ClientFor) mcp.ToolHandlerFor[deleteDraftInput, Draft] {
			find := func(ctx context.Context, client *mattermost.Client, input deleteDraftInput) (*model.User, *model.Draft, error) {
				channelID, rootID, err := threadOf(ctx, client, input.ChannelID, input.RootID)
				if err != nil {
					return nil, nil, err
				}
				self, err := client.Me(ctx)
				if err != nil {
					return nil, nil, err
				}
				teamID, err := teamOf(ctx, client, channelID)
				if err != nil {
					return nil, nil, err
				}
				draft, err := existingDraft(ctx, client, self.Id, teamID, channelID, rootID)
				if err != nil {
					return nil, nil, err
				}
				if draft == nil {
					return nil, nil, fmt.Errorf("there is no draft there; list_drafts lists the user's drafts")
				}
				return self, draft, nil
			}
			// A deletion is bound to the draft's text: the person may be writing
			// it in Mattermost while they are asked, and words added since are not
			// theirs to lose unseen.
			bind := func(ctx context.Context, request *mcp.CallToolRequest, input deleteDraftInput) (string, error) {
				client, err := clientFor(ctx, request)
				if err != nil {
					return "", err
				}
				_, draft, err := find(ctx, client, input)
				if err != nil {
					return "", err
				}
				return draft.Message, nil
			}
			return askingBound("delete_draft", bind,
				func(ctx context.Context, request *mcp.CallToolRequest, input deleteDraftInput) (confirmation, error) {
					client, err := clientFor(ctx, request)
					if err != nil {
						return confirmation{}, err
					}
					_, draft, err := find(ctx, client, input)
					if err != nil {
						return confirmation{}, err
					}
					described, err := describeDrafts(ctx, client, []*model.Draft{draft})
					if err != nil {
						return confirmation{}, err
					}
					where := draftPlace(described[0], draft)
					if err := checkDiscards("the unsent draft in "+where, draft.Message, len(draft.FileIds), input.Discards); err != nil {
						return confirmation{}, err
					}
					return confirmation{
						Message: fmt.Sprintf("Delete your unsent draft in %s:\n\n%s", where, draft.Message),
						Label:   "Delete the draft in " + where + " and its text",
					}, nil
				},
				func(ctx context.Context, request *mcp.CallToolRequest, input deleteDraftInput) (*mcp.CallToolResult, Draft, error) {
					client, err := clientFor(ctx, request)
					if err != nil {
						return nil, Draft{}, err
					}
					self, draft, err := find(ctx, client, input)
					if err != nil {
						return nil, Draft{}, err
					}
					if err := stillAsAsked(ctx, "delete_draft", draft.Message); err != nil {
						return nil, Draft{}, err
					}
					// Named before it goes, when there is still a draft to read.
					described, err := describeDrafts(ctx, client, []*model.Draft{draft})
					if err != nil {
						return nil, Draft{}, err
					}
					if err := checkDiscards("the unsent draft in "+draftPlace(described[0], draft), draft.Message, len(draft.FileIds), input.Discards); err != nil {
						return nil, Draft{}, err
					}
					if err := client.DeleteDraft(ctx, self.Id, draft.ChannelId, draft.RootId); err != nil {
						return nil, Draft{}, err
					}
					return nil, described[0], nil
				})
		},
	), map[string]string{"discards": discardCheck}), "The person is asked to confirm, seeing the draft; if they decline or close the question, do not try again unless they ask.")
}

// DraftChange is what came of the person's press on a proposal's view.
type DraftChange struct {
	Status   string    `json:"status" jsonschema:"what came of the press: posted; confirm, nothing was posted, since Mattermost has the sender confirm a message that alerts this many people first; saved, the draft holds the new text; discarded; changed, the draft holds a newer proposal than the view showed, or, for replace, the person's draft changed, and was left as it is; elsewhere, the person wrote in it in Mattermost, or gave it files or settings there, and it is sent from there; gone, there is no draft there"`
	Draft    Draft     `json:"draft,omitzero" jsonschema:"the draft as it is now, when it was saved, has changed, or is sent from Mattermost"`
	Post     Post      `json:"post,omitzero" jsonschema:"the post, once posted"`
	Notifies *Notifies `json:"notifies,omitempty" jsonschema:"for confirm: whom the message alerts, as Mattermost's confirmation names them"`
}

// pressDraft does what the person pressed in a proposal's view: post the
// draft, change its text, discard it, or put the proposal in place of their
// own (ADR-035). Post, Edit and Discard act only on what a view put there,
// unchanged, and only while it holds the text the view shows, text alone: a
// newer proposal is drawn again, and a draft the person wrote in, or gave
// files or settings, in Mattermost is sent from there. Replace acts only
// while the person's draft holds what the view shows of it. Posting checks
// the message again; one that alerts as many people as Mattermost has its
// sender confirm is posted only on a press that confirmed it, counted as the
// press is made. It marks the post as written with AI, and deletes the draft,
// so a second press finds nothing to post. Presses on one draft take turns.
func pressDraft(ctx context.Context, client *mattermost.Client, cfg config.Config, input viewChangeInput) (DraftChange, error) {
	if input.Action == "post" && !cfg.PostFromViews {
		return DraftChange{}, fmt.Errorf("this server does not post from a view (%s=false): the person sends the draft from Mattermost", config.EnvPostFromViews)
	}
	if input.Action == "edit" && strings.TrimSpace(input.Message) == "" {
		return DraftChange{}, fmt.Errorf("the draft would be empty; Discard deletes it")
	}
	if input.Action == "replace" {
		if err := slashCommand("view_change", input.Message); err != nil {
			return DraftChange{}, err
		}
	}
	if input.Action == "edit" || input.Action == "replace" {
		if err := checkLength(ctx, client, input.Message); err != nil {
			return DraftChange{}, err
		}
	}
	self, err := client.Me(ctx)
	if err != nil {
		return DraftChange{}, err
	}
	release, err := holdDraft(ctx, self.Id, input.ChannelID, input.RootID)
	if err != nil {
		return DraftChange{}, err
	}
	defer release()
	teamID, err := teamOf(ctx, client, input.ChannelID)
	if err != nil {
		return DraftChange{}, err
	}
	there, err := existingDraft(ctx, client, self.Id, teamID, input.ChannelID, input.RootID)
	if err != nil {
		return DraftChange{}, err
	}
	now := func(draft *model.Draft, status string) (DraftChange, error) {
		described, err := describeDrafts(ctx, client, []*model.Draft{draft})
		if err != nil {
			return DraftChange{}, err
		}
		return DraftChange{Status: status, Draft: described[0]}, nil
	}
	switch {
	case there == nil:
		return DraftChange{Status: "gone"}, nil
	case input.Action == "replace":
		// The person's own draft, which they chose to put the proposal in
		// place of, as the view shows it.
		if there.Message != input.Shown {
			return now(there, "changed")
		}
	case !fromView(there):
		return now(there, "elsewhere")
	case there.Message != input.Shown:
		return now(there, "changed")
	}
	if !onlyText(there) {
		return now(there, "elsewhere")
	}
	switch input.Action {
	case "post":
		channel, err := client.Channel(ctx, input.ChannelID)
		if err != nil {
			return DraftChange{}, err
		}
		groups, err := groupNames(ctx, client, []*model.Channel{channel}, self.Id)
		if err != nil {
			return DraftChange{}, err
		}
		// Checked again at the press: someone mentioned may be gone, and an
		// edit in the view may have changed what it mentions.
		where := place(channel, self.Id, nil, groups[channel.Id])
		members := func(ctx context.Context) (int64, error) {
			stats, err := client.ChannelStats(ctx, input.ChannelID)
			if err != nil {
				return 0, err
			}
			return stats.MemberCount, nil
		}
		checked, err := checkMessage(ctx, client, there.Message, where, members)
		if err != nil {
			return DraftChange{}, err
		}
		if !input.Confirmed && (len(checked.specials) > 0 || len(checked.groups) > 0) {
			count, err := members(ctx)
			if err != nil {
				return DraftChange{}, err
			}
			loud, err := loudnessIn(ctx, client, self, channel, where, checked, count)
			if err != nil {
				return DraftChange{}, err
			}
			notifies, err := notifiesOf(ctx, client, input.ChannelID, loud, len(checked.specials) > 0)
			if err != nil {
				return DraftChange{}, err
			}
			if notifies != nil {
				return DraftChange{Status: "confirm", Notifies: notifies}, nil
			}
		}
		// Bound to the draft as it was read, as Mattermost's own client binds
		// a post to its pending id: a press after this one, which finds the
		// draft still there if deleting it failed, is answered with this post.
		created, err := client.CreatePost(ctx, mattermost.NewPost{
			ChannelID: input.ChannelID, RootID: input.RootID, Message: there.Message,
			Props:     aiMarker(cfg.MarkAIGenerated, self.Id),
			PendingID: self.Id + ":" + strconv.FormatInt(there.UpdateAt, 10),
		})
		if err != nil {
			return DraftChange{}, err
		}
		// Gone once posted, as Mattermost's own client clears a draft it sends,
		// so a later press finds nothing to post. Tried twice; the post stands
		// whatever comes of it.
		if err := client.DeleteDraft(ctx, self.Id, input.ChannelID, input.RootID); err != nil {
			_ = client.DeleteDraft(ctx, self.Id, input.ChannelID, input.RootID)
		}
		posted, err := describePosts(ctx, client, []*model.Post{created})
		if err != nil {
			return DraftChange{Status: "posted", Post: Post{ID: created.Id, ChannelID: created.ChannelId, RootID: created.RootId, Message: created.Message, AuthorID: created.UserId}}, nil
		}
		return DraftChange{Status: "posted", Post: posted[0]}, nil
	case "edit":
		// Changed by the person, the draft is theirs: a later proposal leaves
		// it as it is. Its mark says the view wrote it, for the view's presses.
		saved, err := client.Draft(ctx, self.Id, input.ChannelID, input.RootID, input.Message, model.StringInterface{proposalKey: editedMark(input.Message)})
		if err != nil {
			return DraftChange{}, err
		}
		return now(saved, "saved")
	case "discard":
		if err := client.DeleteDraft(ctx, self.Id, input.ChannelID, input.RootID); err != nil {
			return DraftChange{}, err
		}
		return DraftChange{Status: "discarded"}, nil
	}
	// Replace mine, the one action left, which the input's schema holds it to.
	saved, err := client.Draft(ctx, self.Id, input.ChannelID, input.RootID, input.Message, model.StringInterface{proposalKey: proposalMark(input.Message)})
	if err != nil {
		return DraftChange{}, err
	}
	return now(saved, "saved")
}

// presses are the presses on each draft this server holds to one at a time,
// by user, channel and thread. A press posts a draft bound to a pending id,
// which Mattermost answers a second post with, but it reads and records the id
// in two steps (deduplicateCreatePost in server/channels/app/post.go, on both
// supported releases), so presses that reach it together each post: views of
// one proposal pressed at once would post it more than once. Taking turns, a
// press after the first finds the draft gone. A proposal takes its turn too:
// one saved while a press posts the draft before it would be deleted with that
// draft. A server over HTTP may act for several users (ADR-020).
var presses = struct {
	sync.Mutex
	held map[string]*pressTurns
}{held: map[string]*pressTurns{}}

// pressTurns is one draft's turn, and how many presses hold it or wait.
type pressTurns struct {
	turn    chan struct{}
	waiting int
}

// holdDraft waits for the presses on the user's draft in a channel, or in a
// thread of it, that came before, and answers with the release that lets the
// next one go.
func holdDraft(ctx context.Context, userID, channelID, rootID string) (func(), error) {
	key := userID + "/" + channelID + "/" + rootID
	presses.Lock()
	turns := presses.held[key]
	if turns == nil {
		turns = &pressTurns{turn: make(chan struct{}, 1)}
		presses.held[key] = turns
	}
	turns.waiting++
	presses.Unlock()
	leave := func() {
		presses.Lock()
		defer presses.Unlock()
		if turns.waiting--; turns.waiting == 0 {
			delete(presses.held, key)
		}
	}
	select {
	case turns.turn <- struct{}{}:
		return func() {
			<-turns.turn
			leave()
		}, nil
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
}

// pressUses are the operations pressDraft calls.
func pressUses() []Use {
	return uses([]Use{
		{
			Operation: "GetDrafts",
			Params: map[string]Coverage{
				"user_id": Fixed("me", "a user's drafts are theirs"),
				"team_id": Fixed("the channel's team", "Mattermost files a draft under its channel's team, and a direct message's under each of the user's teams"),
			},
			Releases: "11.7 serves GET /api/v4/users/{user_id}/teams/{team_id}/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
		},
		{Operation: "GetChannel", Params: map[string]Coverage{"channel_id": SetBy("channel_id")}},
		{
			Operation: "GetTeamsForUser",
			Params:    map[string]Coverage{"user_id": Fixed("me", "a direct message belongs to no team, and its drafts are filed under the user's")},
		},
		{
			Operation: "CreatePost",
			Params: map[string]Coverage{
				"body.channel_id": SetBy("channel_id"),
				"body.message":    SetBy("shown"),
				"body.root_id":    SetBy("root_id"),
				"body.file_ids":   Omitted("a proposal carries text; a draft given files in Mattermost is sent from there"),
				"body.props": Fixed(`{"ai_generated_by": the user's id}`,
					"marks the post as written with AI, as every post mm-mcp sends; MM_MCP_MARK_AI_GENERATED=false leaves it out"),
				"body.metadata": Omitted("metadata such as a post's priority is a Mattermost feature beyond posting text, which the person is not shown"),
				"set_online":    Omitted("Mattermost's default marks the user online when they post, as its own client does"),
				"silent":        Omitted("a silent post notifies nobody it mentions, unlike a post from Mattermost's own message box"),
				"body.pending_post_id": Undocumented("", "createPost in server/channels/app/post.go deduplicates by it on both supported releases: "+
					"a second post with the same id within 30 seconds answers with the first, though not one that reaches it together with the first, which it reads and records in two steps; "+
					"set to the user and the draft's update time, so a press after the first, which presses take turns for, posts nothing new"),
			},
			Releases: "11.7 has no silent parameter, which the tool never sends",
		},
		{
			Operation: "UpsertDraft",
			Params: map[string]Coverage{
				"body.channel_id": SetBy("channel_id"),
				"body.root_id":    SetBy("root_id"),
				"body.message":    SetBy("message"),
				"body.file_ids":   Omitted("a proposal carries text; the person attaches files in Mattermost"),
				"body.priority":   Omitted("a message's priority is the person's to set when they send it"),
				"body.props": Fixed(`{"mm_mcp_proposal": a mark of the message}, as a proposal's for replace and as the person's text for edit`,
					"a proposal put in place of the person's draft is mm-mcp's, unchanged; a draft the person changed in the view is theirs, which only the view's presses reach"),
				"body.type": Omitted("a draft is an ordinary message, which is the default"),
			},
			Releases: "11.7 serves POST /api/v4/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
		},
		{
			Operation: "DeleteDraft",
			Params: map[string]Coverage{
				"user_id":    Fixed("me", "a user's drafts are theirs"),
				"channel_id": SetBy("channel_id"),
			},
			Releases: "11.7 serves DELETE /api/v4/users/{user_id}/channels/{channel_id}/drafts, as its router shows, though its specification leaves it out; nothing differs in use",
		},
		{
			Operation: "DeleteDraftForThread",
			Params: map[string]Coverage{
				"user_id":    Fixed("me", "a user's drafts are theirs"),
				"channel_id": SetBy("channel_id"),
				"thread_id":  SetBy("root_id"),
			},
			Releases: "11.7 serves DELETE /api/v4/users/{user_id}/channels/{channel_id}/drafts/{thread_id}, as its router shows, though its specification leaves it out; nothing differs in use",
		},
		{
			Operation: "GetChannelStats",
			Params:    map[string]Coverage{"channel_id": Fixed("the draft's channel", "the message is checked again at the press, its mentions of everyone with it")},
		},
	}, messageUses(false), notifiesUses(), describeUses(true))
}
