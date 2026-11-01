package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/doctor"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// check_views measures what a client lets a view do. diagnose cannot: it runs
// in the server, and the host's answer to a page goes to the page. So the check
// is a page of its own, shown as a view, which tries each thing and reports it
// (ADR-022).
//
// A client may pass a view's calls on or not, so the report leaves by every
// route there is: on the page, to paste; through ui/update-model-context, for a
// model that reads it; and through view_check to the server, where diagnose
// gives it to the model. What reached the server is counted here, because that
// is the part only the server sees.

const (
	// checkMetaKey holds in a result's _meta what the check looks for there.
	checkMetaKey = "mm-mcp/check"
	// reportMost bounds a report the server keeps, in bytes: it comes back
	// through diagnose, to the model.
	reportMost = 16 << 10
)

// noViewsToCheck is check_views' answer to a client that renders no views.
const noViewsToCheck = "Nothing was shown: this MCP client renders no MCP Apps views, so there is nothing for a view to measure."

// checkShown is what check_views tells the model it put in front of the person.
const checkShown = "Shown to the person: a check of what this client lets a view do. It runs in the view, and the person can press to try more. " +
	"Its report reaches you only where this client passes a view's context on; otherwise ask the person to paste it from the view. " +
	"diagnose then says what reached this server."

type checkViewsInput struct{}

// ViewsCheck is what check_views answers: what the check looks for, and what
// it reads.
type ViewsCheck struct {
	Stamp           string `json:"stamp" jsonschema:"what this build's pages were made from, which the page compares with its own"`
	AvatarURI       string `json:"avatar_uri,omitempty" jsonschema:"the person's own picture, which the check reads to see whether a view can; absent when the person could not be read"`
	CarriesPictures bool   `json:"carries_pictures" jsonschema:"whether a thread shown in this client brings its pictures with it"`
	// A client may hand its model this and not the result's text, so what the
	// model should do next is said here too.
	Next string `json:"next" jsonschema:"what was shown, and how the report reaches you"`
}

type viewCheckInput struct {
	Report string `json:"report,omitempty" jsonschema:"the check's report, for diagnose to give the model; left out, the call only shows that a view's call reaches the server"`
}

// ViewCheckSeen is what view_check answers: what has reached this server of
// the check, since it started.
type ViewCheckSeen struct {
	FixtureReads int `json:"fixture_reads" jsonschema:"how many of the check's reads of mm-mcp's own fixtures reached this server"`
	Calls        int `json:"calls" jsonschema:"how many calls from a view's check reached this server, this one included"`
}

var viewCheckShapes = map[string]string{
	"report": "hands the check's report to this server, for diagnose to give the model; it sets no parameter",
}

// checkLog is what has reached a server of the client check: only the server
// sees whether a read or a call the view made got this far.
type checkLog struct {
	mu           sync.Mutex
	pages        map[string]time.Time
	fixtureReads int
	calls        int
	report       string
	reportedAt   time.Time
}

type checkLogKey struct{}

// middleware makes the log reach the tools, which are registered apart from
// the server's own state.
func (log *checkLog) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		return next(context.WithValue(ctx, checkLogKey{}, log), method, request)
	}
}

// checkLogOf is the log of the server a request reached.
func checkLogOf(ctx context.Context) *checkLog {
	if log, ok := ctx.Value(checkLogKey{}).(*checkLog); ok {
		return log
	}
	return &checkLog{}
}

func (log *checkLog) pageRead(uri string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.pages == nil {
		log.pages = map[string]time.Time{}
	}
	log.pages[uri] = time.Now()
}

func (log *checkLog) fixtureRead() {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.fixtureReads++
}

func (log *checkLog) called(report string) ViewCheckSeen {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.calls++
	if report != "" {
		log.report, log.reportedAt = report, time.Now()
	}
	return ViewCheckSeen{FixtureReads: log.fixtureReads, Calls: log.calls}
}

// checkPageMeta says what the check page may do, which the page that draws
// Mattermost may not: reach the server's own address, and write to the
// clipboard, each to see whether a client lets it.
func checkPageMeta(server *url.URL) mcp.Meta {
	origin := server.Scheme + "://" + server.Host
	return mcp.Meta{"ui": map[string]any{
		"csp":           map[string]any{"connectDomains": []string{origin}, "resourceDomains": []string{origin}},
		"permissions":   map[string]any{"clipboardWrite": map[string]any{}},
		"prefersBorder": true,
	}}
}

// checkToolMeta links check_views to the check page.
func checkToolMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"resourceUri": views.CheckURI}, "ui/resourceUri": views.CheckURI}
}

// outputOf is the output schema of a tool that answers as t, or with an error
// that holds none of it.
func outputOf(t reflect.Type) *jsonschema.Schema {
	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("the output schema of %v: %v", t, err))
	}
	return schema
}

func checkViewsSpec() Spec {
	tool := &mcp.Tool{
		Name: "check_views",
		Description: "Measure what this MCP client lets mm-mcp's views do: it shows the person a check, as a view, that tries reading pictures " +
			"and files, calling tools, reaching the network, opening links and going fullscreen, and says what each means for mm-mcp. Call it " +
			"when the person asks what their client can do with views, or why a view shows no pictures, no buttons or no links; it is for " +
			"them, not for you, so call it only when they are there to see it. The one thing of theirs it reads is their own picture, of " +
			"which it reports only whether it arrived. In a client that renders no views it shows nothing, and says so.",
		Annotations:  readOnly("Check this client's views"),
		Meta:         checkToolMeta(),
		OutputSchema: outputOf(reflect.TypeFor[ViewsCheck]()),
	}
	return toolSpec(tool,
		[]Use{{
			Operation: "GetUser",
			Params:    map[string]Coverage{"user_id": Fixed("me", "the check reads the person's own picture, to see whether a view can read one")},
		}},
		func(clientFor ClientFor) mcp.ToolHandlerFor[checkViewsInput, any] {
			return func(ctx context.Context, request *mcp.CallToolRequest, _ checkViewsInput) (*mcp.CallToolResult, any, error) {
				if !rendersViews(request) {
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: noViewsToCheck}}}, nil, nil
				}
				out := ViewsCheck{Stamp: views.Stamp(), CarriesPictures: carriesPictures(request), AvatarURI: ownPicture(ctx, clientFor, request), Next: checkShown}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: checkShown}},
					Meta:    mcp.Meta{checkMetaKey: map[string]any{"stamp": out.Stamp}},
				}, out, nil
			}
		},
	)
}

// ownPicture is the address of the person's own picture, or nothing when they
// cannot be read: a check of the client is worth having when Mattermost is what
// is failing.
func ownPicture(ctx context.Context, clientFor ClientFor, request *mcp.CallToolRequest) string {
	client, err := clientFor(ctx, request)
	if err != nil {
		return ""
	}
	me, err := client.Me(ctx)
	if err != nil {
		return ""
	}
	server, err := url.Parse(client.Address())
	if err != nil {
		return ""
	}
	return pictureResource(server, me.Id)
}

func viewCheckSpec() Spec {
	tool := &mcp.Tool{
		Name:         "view_check",
		Description:  "Only mm-mcp's client check calls this, from its view. Never call it yourself.",
		Annotations:  readOnly("Report a view's check"),
		Meta:         appOnlyMeta(),
		OutputSchema: outputOf(reflect.TypeFor[ViewCheckSeen]()),
	}
	return shaping(toolSpec(tool, nil,
		func(ClientFor) mcp.ToolHandlerFor[viewCheckInput, any] {
			return func(ctx context.Context, request *mcp.CallToolRequest, input viewCheckInput) (*mcp.CallToolResult, any, error) {
				if !rendersViews(request) {
					return nil, nil, fmt.Errorf("view_check is called by mm-mcp's views alone, from a client that renders them")
				}
				if len(input.Report) > reportMost {
					return nil, nil, fmt.Errorf("the report is %d bytes; a report holds at most %d", len(input.Report), reportMost)
				}
				return nil, checkLogOf(ctx).called(input.Report), nil
			}
		},
	), viewCheckShapes)
}

// capabilitiesCheck says what the client declares on this request, as it
// declares it.
func capabilitiesCheck(request *mcp.CallToolRequest) doctor.Check {
	const name = "client capabilities"
	capabilities := request.ClientCapabilities()
	if capabilities == nil {
		return doctor.Check{Name: name, Status: doctor.OK, Detail: "this client declares no capabilities on this request"}
	}
	declared, err := json.Marshal(capabilities)
	if err != nil {
		return doctor.Check{Name: name, Status: doctor.Warning, Detail: fmt.Sprintf("what this client declares could not be read: %v", err)}
	}
	return doctor.Check{Name: name, Status: doctor.OK, Detail: "this client declares " + string(declared)}
}

// viewCheckCheck says what has reached this server of a view's reads and calls,
// and what the client check reported, where it could.
func viewCheckCheck(ctx context.Context) doctor.Check {
	log := checkLogOf(ctx)
	log.mu.Lock()
	defer log.mu.Unlock()
	read := func(uri string) string {
		if at, done := log.pages[uri]; done {
			return "was read at " + at.Format(time.RFC3339)
		}
		return "has not been read"
	}
	detail := fmt.Sprintf("since this server started, the page that draws views %s and the check's page %s; "+
		"%d of the check's reads and %d of its calls have reached this server, where a check that ran and shows none means this client passes on no read or call from a view",
		read(views.URI), read(views.CheckURI), log.fixtureReads, log.calls)
	if log.report == "" {
		return doctor.Check{
			Name: "view check", Status: doctor.OK, Detail: detail,
			Next: "What a view can do here shows nowhere in a request. When the person asks, check_views measures it; where this client passes nothing on, its report waits in the view for them to paste.",
		}
	}
	return doctor.Check{
		Name: "view check", Status: doctor.OK,
		Detail: detail + fmt.Sprintf(". The check's report from %s, as the page wrote it: %s", log.reportedAt.Format(time.RFC3339), log.report),
	}
}
