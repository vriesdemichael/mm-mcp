//go:build spike

package server

// The MCP Apps spike (#37): a throwaway view, built only with -tags spike, to
// find out what clients do with a view on every call of a tool. It never
// merges. Every request a client sends is logged to stderr and, when SPIKE_LOG
// names a file, to that file too, so a client's behaviour can be read back
// whether a person or a machine drove it.

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
)

//go:embed spike/page.html
var spikePage string

const (
	spikeExtension = "io.modelcontextprotocol/ui"
	spikeMIME      = "text/html;profile=mcp-app"
	spikeURI       = "ui://mm-mcp/spike"
	spikeKey       = "io.github.vriesdemichael.mm-mcp/spike"
)

var spikeLog = newSpikeLog()

func newSpikeLog() *log.Logger {
	var out io.Writer = os.Stderr
	if path := os.Getenv("SPIKE_LOG"); path != "" {
		if file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			out = io.MultiWriter(os.Stderr, file)
		}
	}
	return log.New(out, "mm-mcp spike: ", log.LstdFlags|log.Lmicroseconds)
}

func init() { spikeRegister = registerSpike }

func registerSpike(server *mcp.Server, clientFor ClientFor, _ config.Config) {
	server.AddReceivingMiddleware(spikeLogging)
	server.AddResource(&mcp.Resource{
		URI:         spikeURI,
		Name:        "spike-view",
		Title:       "mm-mcp spike view",
		Description: "The page the spike's views render in (#37).",
		MIMEType:    spikeMIME,
		Meta:        spikeResourceMeta(),
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: spikeMIME, Text: spikePage, Meta: spikeResourceMeta(),
		}}}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:  "spike_show_post",
		Title: "Spike: show a post",
		Description: "Spike for #37. Reads a post and shows it to the person as an MCP Apps view, in a client that renders views. " +
			"mode is full (the post, with buttons), line (one line saying what was read) or none (a view that draws nothing). " +
			"auto runs the view's checks without anyone pressing a button.",
		Annotations: readOnly("Spike: show a post"),
		Meta:        mcp.Meta{"ui": map[string]any{"resourceUri": spikeURI}, "ui/resourceUri": spikeURI},
	}, spikeShowPost(clientFor))
	mcp.AddTool(server, &mcp.Tool{
		Name:        "spike_press",
		Title:       "Spike: press",
		Description: "Spike for #37, called by the spike's view: records what reached the server, a token in the request's _meta or in the arguments, and a note.",
		// Not read-only, to see whether a host asks before a view's call
		// of a tool that writes.
		Annotations: writes("Spike: press", false),
		Meta:        spikeAppOnly(),
	}, spikePress)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "spike_write",
		Title:       "Spike: write",
		Description: "Spike for #37: pretends to write and changes nothing; it shows whether the client asks before a model's call of a tool that writes.",
		Annotations: writes("Spike: write", false),
	}, spikePress)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "spike_token",
		Title:       "Spike: token",
		Description: "Spike for #37, called by the spike's view: hands the view a token outside the result the model reads.",
		Annotations: readOnly("Spike: token"),
		Meta:        spikeAppOnly(),
	}, spikeToken)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "spike_ask",
		Title:       "Spike: ask",
		Description: "Spike for #37: asks the person a question through elicitation and answers with what came back.",
		Annotations: readOnly("Spike: ask"),
	}, spikeAsk)
	spikeLog.Printf("registered: resource %s, tools spike_show_post, spike_press (app only), spike_token (app only), spike_ask, spike_write", spikeURI)
}

// spikeAppOnly marks a tool for the view alone: a host that follows the
// specification leaves it out of the tools its model is offered.
func spikeAppOnly() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
}

type spikeTokenOutput struct {
	Token string `json:"token"`
}

func spikeToken(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, spikeTokenOutput, error) {
	token := spikeCanary("SPIKE-TOKEN-")
	spikeLog.Printf("spike_token: handed out %s", token)
	return nil, spikeTokenOutput{Token: token}, nil
}

func spikeResourceMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"csp": map[string]any{}, "prefersBorder": true}}
}

// spikeLogging logs every request and notification a client sends, with its
// parameters, so what each client declares and passes on can be read back.
func spikeLogging(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		params, _ := json.Marshal(request.GetParams())
		text := string(params)
		if method != "initialize" && len(text) > 4000 {
			text = text[:4000] + "…"
		}
		spikeLog.Printf("received %s %s", method, text)
		result, err := next(ctx, method, request)
		if err != nil {
			spikeLog.Printf("answered %s with error: %v", method, err)
		}
		return result, err
	}
}

// spikeViews reports whether the client declared it renders MCP Apps views.
func spikeViews(request *mcp.CallToolRequest) (bool, string) {
	// Per request: clients on protocol 2026-07-28 send no initialize, and put
	// their capabilities in each request's _meta.
	capabilities := request.ClientCapabilities()
	if capabilities == nil {
		return false, "no capabilities"
	}
	settings, ok := capabilities.Extensions[spikeExtension]
	declared, _ := json.Marshal(settings)
	if !ok {
		return false, "no " + spikeExtension
	}
	if values, ok := settings.(map[string]any); ok {
		if types, ok := values["mimeTypes"].([]any); ok {
			for _, value := range types {
				if value == spikeMIME {
					return true, string(declared)
				}
			}
			return false, string(declared)
		}
	}
	return true, string(declared)
}

func spikeCanary(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

type spikeShowInput struct {
	PostID string `json:"post_id" jsonschema:"the post to show"`
	Mode   string `json:"mode,omitempty" jsonschema:"full, line or none; full by default"`
	Auto   bool   `json:"auto,omitempty" jsonschema:"run the view's checks without a press"`
	Ask    bool   `json:"ask,omitempty" jsonschema:"with auto, also trigger the question from the view"`
	Force  bool   `json:"force_views,omitempty" jsonschema:"send the view's data even when the client declared no views"`
}

type spikeShowOutput struct {
	Shown            bool           `json:"shown"`
	Mode             string         `json:"mode"`
	StructuredCanary string         `json:"structured_canary"`
	View             map[string]any `json:"view,omitempty"`
}

func spikeShowPost(clientFor ClientFor) mcp.ToolHandlerFor[spikeShowInput, spikeShowOutput] {
	return func(ctx context.Context, request *mcp.CallToolRequest, input spikeShowInput) (*mcp.CallToolResult, spikeShowOutput, error) {
		mode := input.Mode
		if mode == "" {
			mode = "full"
		}
		if mode != "full" && mode != "line" && mode != "none" {
			return nil, spikeShowOutput{}, fmt.Errorf("mode is full, line or none, not %q", mode)
		}
		client, err := clientFor(ctx, request)
		if err != nil {
			return nil, spikeShowOutput{}, err
		}
		post, err := client.Post(ctx, input.PostID)
		if err != nil {
			return nil, spikeShowOutput{}, err
		}
		author, err := client.User(ctx, post.UserId)
		if err != nil {
			return nil, spikeShowOutput{}, err
		}
		views, declared := spikeViews(request)
		if input.Force && !views {
			spikeLog.Printf("spike_show_post: the client declared no views (%s); forced", declared)
			views = true
		}
		out := spikeShowOutput{Shown: views, Mode: mode, StructuredCanary: spikeCanary("SPIKE-SC-")}
		if !views {
			spikeLog.Printf("spike_show_post: the client declared no views (%s)", declared)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
				Text: "This client renders no views, so nothing was shown. Post by @" + author.Username + ": " + post.Message,
			}}}, out, nil
		}
		payload := map[string]any{
			"mode": mode, "auto": input.Auto, "ask": input.Ask,
			"author": author.Username, "message": post.Message, "channel_id": post.ChannelId,
			"token": spikeCanary("SPIKE-TOKEN-"),
		}
		// The same data in structuredContent too, to see which of the two a
		// client hands its view; the meta canary is in _meta alone, so its
		// reaching the model says _meta did.
		inMeta := map[string]any{"meta_canary": spikeCanary("SPIKE-META-")}
		for key, value := range payload {
			inMeta[key] = value
		}
		spikeLog.Printf("spike_show_post: views declared (%s); mode %s; canaries meta=%s structured=%s token=%s",
			declared, mode, inMeta["meta_canary"], out.StructuredCanary, payload["token"])
		out.View = payload
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Showed the person the post by @" + author.Username + " (mode " + mode + "): " + post.Message}},
			Meta:    mcp.Meta{spikeKey: inMeta},
		}, out, nil
	}
}

type spikePressInput struct {
	Token string `json:"token,omitempty" jsonschema:"a token, when the view sends it as an argument"`
	Note  string `json:"note,omitempty" jsonschema:"what the view reports"`
}

type spikePressOutput struct {
	MetaToken string `json:"meta_token,omitempty"`
	ArgToken  string `json:"arg_token,omitempty"`
	Note      string `json:"note,omitempty"`
}

func spikePress(_ context.Context, request *mcp.CallToolRequest, input spikePressInput) (*mcp.CallToolResult, spikePressOutput, error) {
	out := spikePressOutput{ArgToken: input.Token, Note: input.Note}
	if request.Params.Meta != nil {
		if token, ok := request.Params.Meta[spikeKey+"/token"].(string); ok {
			out.MetaToken = token
		}
	}
	meta, _ := json.Marshal(request.Params.Meta)
	spikeLog.Printf("spike_press: _meta=%s token in _meta=%q token in argument=%q note=%q", meta, out.MetaToken, out.ArgToken, input.Note)
	return nil, out, nil
}

type spikeAskOutput struct {
	Action  string         `json:"action"`
	Content map[string]any `json:"content,omitempty"`
}

func spikeAsk(_ context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, spikeAskOutput, error) {
	// As mm-mcp's own questions do (confirm.go): an input request in the
	// result, which the SDK sends as elicitation to a client on an older
	// protocol, and the answer in the retried call's input responses.
	if answer, ok := request.Params.InputResponses["seen"]; ok {
		result, _ := answer.(*mcp.ElicitResult)
		if result == nil {
			spikeLog.Printf("spike_ask: answered with something else: %#v", answer)
			return nil, spikeAskOutput{Action: "unreadable"}, nil
		}
		spikeLog.Printf("spike_ask: answered %s %v", result.Action, result.Content)
		return nil, spikeAskOutput{Action: result.Action, Content: result.Content}, nil
	}
	spikeLog.Printf("spike_ask: asking")
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{"seen": &mcp.ElicitParams{
			Message: "Spike #37: can you see this question? If you pressed Ask in the view, the view started this call.",
			RequestedSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{"seen": {Type: "boolean", Title: "I see this question"}},
				Required:   []string{"seen"},
			},
		}},
		RequestState: "spike-ask",
	}, spikeAskOutput{}, nil
}
