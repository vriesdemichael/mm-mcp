package server

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// show puts what the person asked to see in front of them, laid out as
// Mattermost lays it out, in a client that renders MCP Apps views (ADR-022).
// It reads what it draws itself, so its counts are Mattermost's and the model
// cannot put words in a view, and it answers with what the data tool answers,
// built by the same code: the view draws that, unchanged. A client that
// renders no views is told nothing was shown, and nothing is read for it.
//
// view_action is the one tool a view calls, for what the person does in it.
// It is for views alone: listed only to a request that declared views, and
// refusing a call whose request declared none.

// defaultShownPage is how many posts of a thread a view shows at first: about
// a screen, where read_post reads a hundred for the model.
const defaultShownPage = 30

// noViewsHere is show's answer to a client that renders no views.
const noViewsHere = "Nothing was shown: this MCP client renders no MCP Apps views. Give the person what they asked to see in your answer."

// The kinds of view show draws.
var showKinds = []any{"thread"}

type showInput struct {
	Kind          string `json:"kind" jsonschema:"what to show; thread: a post and the thread it is in, or the post alone with include_thread false"`
	PostID        string `json:"post_id" jsonschema:"the post to show: one that starts a thread, or a reply in one"`
	IncludeThread *bool  `json:"include_thread,omitempty" jsonschema:"false shows the post alone; true, the default, the thread it is in"`
	Limit         int    `json:"limit,omitempty" jsonschema:"how many posts of the thread the view shows at first, at most 200; 30 when not given"`
	pageArgs
}

type viewActionInput struct {
	Action string `json:"action" jsonschema:"what the person did in the view; more: the next page of a thread"`
	PostID string `json:"post_id" jsonschema:"the post the view shows"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many posts a page holds, at most 200; 30 when not given"`
	pageArgs
}

// show's and view_action's arguments that set no parameter.
var (
	showShapes       = withShapes(readPostShapes, map[string]string{"kind": "chooses the view show draws; it sets no parameter"})
	viewActionShapes = map[string]string{"action": "names what the person did in a view; it sets no parameter"}
)

func showSpec() Spec {
	spec := shaping(toolSpec(
		&mcp.Tool{
			Name: "show",
			Description: "Show the person what they asked to see, laid out as Mattermost lays it out, in an MCP client that renders MCP Apps " +
				"views: kind thread shows a post and the thread it is in. Call it when the person asked to see something, once you have " +
				"found it. Never call it to read for yourself: read_post does that. It answers with what read_post answers. In a client " +
				"that renders no views it shows nothing, and says so.",
			Annotations: readOnly("Show"),
			Meta:        viewToolMeta(),
		},
		readPostUses(),
		func(clientFor ClientFor) mcp.ToolHandlerFor[showInput, PostWithThread] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input showInput) (*mcp.CallToolResult, PostWithThread, error) {
				if input.Kind != "thread" {
					return nil, PostWithThread{}, fmt.Errorf("show shows kind thread; %q is not a kind of view it draws", input.Kind)
				}
				if !rendersViews(request) {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: noViewsHere}}}, PostWithThread{}, nil
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, PostWithThread{}, err
				}
				out, err := readThread(ctx, client, readPostInput{PostID: input.PostID, IncludeThread: input.IncludeThread, Limit: input.Limit, pageArgs: input.pageArgs}, defaultShownPage)
				if err != nil {
					return nil, PostWithThread{}, err
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: shown(out)}}}, out, nil
			}
		},
	), showShapes)
	if schema, ok := spec.Tool.InputSchema.(*jsonschema.Schema); ok {
		schema.Properties["kind"].Enum = showKinds
	}
	return spec
}

func viewActionSpec() Spec {
	return shaping(toolSpec(
		&mcp.Tool{
			Name:        "view_action",
			Description: "Only mm-mcp's MCP Apps views call this, for what the person does in one. Never call it yourself.",
			Annotations: readOnly("Act in a view"),
			Meta:        appOnlyMeta(),
		},
		threadPageUses(),
		func(clientFor ClientFor) mcp.ToolHandlerFor[viewActionInput, PostWithThread] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input viewActionInput) (*mcp.CallToolResult, PostWithThread, error) {
				if !rendersViews(request) {
					return nil, PostWithThread{}, fmt.Errorf("view_action is called by mm-mcp's views alone, from a client that renders them")
				}
				if input.Action != "more" {
					return nil, PostWithThread{}, fmt.Errorf("view_action does %q in no view; a thread's view asks for more", input.Action)
				}
				client, err := clientFor(ctx, request)
				if err != nil {
					return nil, PostWithThread{}, err
				}
				// A page of the thread, as show's next page is: its cursor
				// is read_post's.
				out, err := readThread(ctx, client, readPostInput{PostID: input.PostID, Limit: input.Limit, pageArgs: input.pageArgs}, defaultShownPage)
				return nil, out, err
			}
		},
	), viewActionShapes)
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
