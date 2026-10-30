package server

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// show puts what the person asked to see in front of them, laid out as
// Mattermost lays it out, in a client that renders MCP Apps views (ADR-022):
// a thread, or what is unread for them. It reads what it draws itself, so its
// counts are Mattermost's and the model cannot put words in a view, and it
// answers with what the data tools answer, built by the same code: the view
// draws that, unchanged. A client that renders no views is told nothing was
// shown, and nothing is read for it.
//
// A view calls two tools for what the person does in it: view_action for
// what changes nothing, more of a thread, or a thread or a channel opened in
// place, and view_change for what changes something, marking a channel read,
// or posting, changing or discarding a proposal, offered only where writes are
// allowed. Both are for views alone: listed only to a request that declared
// views, and refusing a call whose request declared none.

// defaultShownPage is how many posts of a thread a view shows at first: about
// a screen, where read_post reads a hundred for the model.
const defaultShownPage = 30

// The unread view's ceilings: how many channels, mentions and followed
// threads it lists before it says Mattermost has more, a screen or two where
// the tools a model calls list more (ADR-022). A channel's messages are not
// listed with it: the view opens them in place.
const (
	shownChannels = 20
	shownMentions = 10
	shownThreads  = 10
)

// noViewsHere is show's answer to a client that renders no views.
const noViewsHere = "Nothing was shown: this MCP client renders no MCP Apps views. Give the person what they asked to see in your answer."

// The kinds of view show draws, and what a person does in one. The input's
// schema holds each to these, and a call that names another is refused
// before it is answered.
var (
	showKinds   = []any{"thread", "unreads"}
	viewActions = []any{"more", "thread", "unread"}
	viewChanges = []any{"mark_read", "post", "edit", "discard", "replace"}
)

type showInput struct {
	Kind          string `json:"kind" jsonschema:"what to show; thread: a post and the thread it is in, or the post alone with include_thread false; unreads: what is unread for the person, as Mattermost counts it: their channels with something unread, what mentions them, and the threads they follow with unread replies"`
	PostID        string `json:"post_id,omitempty" jsonschema:"for kind thread: the post to show, one that starts a thread or a reply in one"`
	IncludeThread *bool  `json:"include_thread,omitempty" jsonschema:"for kind thread: false shows the post alone; true, the default, the thread it is in"`
	Limit         int    `json:"limit,omitempty" jsonschema:"for kind thread: how many posts of the thread the view shows at first, at most 200; 30 when not given"`
	pageArgs
}

// Unreads is what show answers for kind unreads: a page of what each of
// get_user_channels with unread_only, list_mentions and list_threads with
// unread_only answers, unchanged (ADR-022).
type Unreads struct {
	Channels Channels `json:"channels" jsonschema:"what get_user_channels with unread_only answers: the channels with something unread, most recently active first"`
	Mentions Mentions `json:"mentions" jsonschema:"what list_mentions answers: the posts that mention the person, most recent first"`
	Threads  Threads  `json:"threads" jsonschema:"what list_threads with unread_only answers: the threads the person follows with replies they have not read"`
}

type viewActionInput struct {
	Action    string `json:"action" jsonschema:"what the person did in the view; more: the next page of a thread; thread: a thread opened in place; unread: a channel's unread messages opened in place"`
	PostID    string `json:"post_id,omitempty" ids:"only" jsonschema:"for more and thread: the post whose thread the view shows, by its id"`
	ChannelID string `json:"channel_id,omitempty" ids:"only" jsonschema:"for unread: the channel opened, by its id"`
	Limit     int    `json:"limit,omitempty" jsonschema:"how many posts a page holds, at most 200; 30 when not given"`
	pageArgs
}

type viewChangeInput struct {
	Action    string `json:"action" jsonschema:"what the person did in the view; mark_read: marked a channel read; post: posted a proposal; edit: changed its text; discard: discarded it; replace: put a proposal in place of their own draft"`
	ChannelID string `json:"channel_id" ids:"only" jsonschema:"the channel, by its id"`
	RootID    string `json:"root_id,omitempty" ids:"only" jsonschema:"for a draft that replies in a thread: the thread's first post, by its id"`
	Shown     string `json:"shown,omitempty" jsonschema:"for a draft: the text the view shows it holding; the press acts only while the draft holds it"`
	Message   string `json:"message,omitempty" jsonschema:"for edit, the draft's new text; for replace, the proposal that takes the place of the person's draft"`
	Confirmed bool   `json:"confirmed,omitempty" jsonschema:"for post: the person confirmed, as Mattermost asks, that the message alerts the people a press without it was told it does"`
}

// show's and the view tools' arguments that set no parameter.
var (
	showShapes       = withShapes(readPostShapes, map[string]string{"kind": "chooses the view show draws, and what it reads for it; it sets no parameter"})
	viewActionShapes = map[string]string{"action": "chooses what a view reads for the person: more of a thread, GetPostThread, or a channel from where they stopped, read_unread's operations"}
	viewChangeShapes = map[string]string{
		"action":    "names what the person did in a view; it sets no parameter",
		"confirmed": "says the person confirmed alerting the people a post notifies, which Mattermost asks first; it sets no parameter",
	}
)

// eitherOutput is the output schema of a tool that answers as one data tool
// or as another, by what it was asked for: one of theirs, each unchanged.
func eitherOutput(types ...reflect.Type) *jsonschema.Schema {
	schema := &jsonschema.Schema{Type: "object"}
	for _, t := range types {
		one, err := jsonschema.ForType(t, &jsonschema.ForOptions{})
		if err != nil {
			panic(fmt.Sprintf("the output schema of %v: %v", t, err))
		}
		schema.OneOf = append(schema.OneOf, one)
	}
	return schema
}

// showUses are the operations show calls: read_post's for a thread, and for
// what is unread, get_user_channels', a mention search across every team, and
// the followed threads of every team.
func showUses() []Use {
	every := Fixed("each of the user's teams", "the unread view lists what is unread in every team the user belongs to")
	return uses(readPostUses(), userChannelsUses(), []Use{mentionSearchUse(mentionSearchParams(Coverage{}))},
		followedThreadUses(every), describeUses(true))
}

func showSpec() Spec {
	tool := &mcp.Tool{
		Name: "show",
		Description: "Show the person what they asked to see, laid out as Mattermost lays it out, in an MCP client that renders MCP Apps " +
			"views: kind thread shows a post and the thread it is in, and kind unreads what is unread for them, their channels, mentions " +
			"and followed threads. Call it when the person asked to see something, once you have found it, or asked what is waiting for " +
			"them. Never call it to read for yourself: read_post, get_user_channels, list_mentions and list_threads do that. It answers " +
			"with what those answer. In a client that renders no views it shows nothing, and says so.",
		Annotations:  readOnly("Show"),
		Meta:         viewToolMeta(),
		OutputSchema: eitherOutput(reflect.TypeFor[PostWithThread](), reflect.TypeFor[Unreads]()),
	}
	spec := shaping(toolSpec(tool, showUses(),
		func(clientFor ClientFor) mcp.ToolHandlerFor[showInput, any] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input showInput) (*mcp.CallToolResult, any, error) {
				if !rendersViews(request) {
					// An error, since the tool declares what it answers and a
					// client refuses an answer that does not hold it: Claude
					// Code would tell the model only that, and not this.
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: noViewsHere}}}, nil, nil
				}
				if input.Kind == "thread" && input.PostID == "" {
					return nil, nil, fmt.Errorf("give post_id, the post whose thread to show")
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, nil, err
				}
				if input.Kind == "unreads" {
					out, err := readUnreads(ctx, client)
					if err != nil {
						return nil, nil, err
					}
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: shownUnreads(out)}}}, out, nil
				}
				out, err := readThread(ctx, client, readPostInput{PostID: input.PostID, IncludeThread: input.IncludeThread, Limit: input.Limit, pageArgs: input.pageArgs}, defaultShownPage)
				if err != nil {
					return nil, nil, err
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: shown(out)}}}, out, nil
			}
		},
	), showShapes)
	only(spec, "kind", showKinds)
	return spec
}

// readUnreads is what is unread for the person, as get_user_channels with
// unread_only, list_mentions and list_threads with unread_only answer it, a
// page of each, as long as the view lists.
func readUnreads(ctx context.Context, client *mattermost.Client) (Unreads, error) {
	channels, err := listUserChannels(ctx, client, getUserChannelsInput{UnreadOnly: true, Limit: shownChannels})
	if err != nil {
		return Unreads{}, err
	}
	mentions, err := listMentions(ctx, client, listMentionsInput{Limit: shownMentions})
	if err != nil {
		return Unreads{}, err
	}
	threads, err := listThreads(ctx, client, listThreadsInput{UnreadOnly: true, Limit: shownThreads})
	if err != nil {
		return Unreads{}, err
	}
	return Unreads{Channels: channels, Mentions: mentions, Threads: threads}, nil
}

// only holds a tool's argument to the values given.
func only(spec Spec, argument string, values []any) {
	if schema, ok := spec.Tool.InputSchema.(*jsonschema.Schema); ok {
		schema.Properties[argument].Enum = values
	}
}

func viewActionSpec() Spec {
	tool := &mcp.Tool{
		Name:         "view_action",
		Description:  "Only mm-mcp's MCP Apps views call this, for what the person does in one. Never call it yourself.",
		Annotations:  readOnly("Act in a view"),
		Meta:         appOnlyMeta(),
		OutputSchema: eitherOutput(reflect.TypeFor[PostWithThread](), reflect.TypeFor[UnreadPosts]()),
	}
	spec := shaping(toolSpec(tool, uses(threadPageUses(), readUnreadUses()),
		func(clientFor ClientFor) mcp.ToolHandlerFor[viewActionInput, any] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input viewActionInput) (*mcp.CallToolResult, any, error) {
				if !rendersViews(request) {
					return nil, nil, fmt.Errorf("view_action is called by mm-mcp's views alone, from a client that renders them")
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, nil, err
				}
				if input.Action == "unread" {
					// A page of the channel from where the person stopped, as
					// read_unread reads it: its cursor is read_unread's.
					out, err := readUnread(ctx, client, readUnreadInput{ChannelID: input.ChannelID, Limit: input.Limit, pageArgs: input.pageArgs})
					if err != nil {
						return nil, nil, err
					}
					return nil, out, nil
				}
				// A page of the thread, as show's next page is: its cursor is
				// read_post's.
				out, err := readThread(ctx, client, readPostInput{PostID: input.PostID, Limit: input.Limit, pageArgs: input.pageArgs}, defaultShownPage)
				if err != nil {
					return nil, nil, err
				}
				return nil, out, nil
			}
		},
	), viewActionShapes)
	only(spec, "action", viewActions)
	return spec
}

func viewChangeSpec() Spec {
	tool := &mcp.Tool{
		Name:         "view_change",
		Description:  "Only mm-mcp's MCP Apps views call this, for what the person does in one that changes something. Never call it yourself.",
		Annotations:  pressed("Change in a view"),
		Meta:         appOnlyMeta(),
		OutputSchema: eitherOutput(reflect.TypeFor[Read](), reflect.TypeFor[DraftChange]()),
	}
	spec := unasked(shaping(configuredToolSpec(tool, uses(markReadUses(), pressUses()),
		func(clientFor ClientFor, cfg config.Config) mcp.ToolHandlerFor[viewChangeInput, any] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input viewChangeInput) (*mcp.CallToolResult, any, error) {
				if !rendersViews(request) {
					return nil, nil, fmt.Errorf("view_change is called by mm-mcp's views alone, from a client that renders them")
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, nil, err
				}
				if input.Action == "mark_read" {
					// Marked read as mark_channel_read marks it, on the person's
					// own press: what they have read is theirs alone.
					out, err := markRead(ctx, client, channelInput{ChannelID: input.ChannelID})
					if err != nil {
						return nil, nil, err
					}
					return nil, out, nil
				}
				out, err := pressDraft(ctx, client, cfg, input)
				if err != nil {
					return nil, nil, err
				}
				return nil, out, nil
			}
		},
	), viewChangeShapes),
		"every call is the person's own press in a view, which a call from a view never asks about (ADR-022, ADR-035): marking a channel read, or posting, changing or discarding what a proposal's view shows")
	only(spec, "action", viewChanges)
	return spec
}

// pressed is the annotation of the tool a view calls for what the person
// does in it that changes something: it posts what they press Post on, and
// replaces or deletes the draft they press Replace mine or Discard on.
func pressed(title string) *mcp.ToolAnnotations {
	annotations := writes(title, false)
	annotations.DestructiveHint = ptr(true)
	return annotations
}

// shown is what show tells the model it put in front of the person, without
// repeating it.
func shown(out PostWithThread) string {
	where := ""
	if out.Post != nil && out.Post.Channel != "" {
		where = " in " + out.Post.Channel
	}
	if len(out.Thread) == 0 {
		return "Shown to the person, as Mattermost lays it out: the post" + where + ". They see it; do not repeat it unless they ask."
	}
	total := int64(len(out.Thread))
	for _, post := range out.Thread {
		if post.ID == out.RootID {
			total = post.ReplyCount + 1
		}
	}
	return fmt.Sprintf("Shown to the person, as Mattermost lays it out: the thread%s, %d of its %d posts. They see it; do not repeat it unless they ask.",
		where, len(out.Thread), total)
}

// shownUnreads is what show tells the model of the unread view it showed.
func shownUnreads(out Unreads) string {
	count := func(n int, one, many string) string { return fmt.Sprintf("%d %s", n, plural(int64(n), one, many)) }
	return fmt.Sprintf("Shown to the person, as Mattermost lays it out, what is unread for them: %s with something unread%s, %s%s, and %s with unread replies%s. "+
		"They see it; do not repeat it unless they ask.",
		count(len(out.Channels.Channels), "channel", "channels"), more(out.Channels.NextCursor),
		count(len(out.Mentions.Posts), "mention", "mentions"), more(out.Mentions.NextCursor),
		count(len(out.Threads.Threads), "followed thread", "followed threads"), more(out.Threads.NextCursor))
}

// more says that Mattermost holds more than a list shows.
func more(cursor string) string {
	if cursor == "" {
		return ""
	}
	return ", and more in Mattermost"
}
