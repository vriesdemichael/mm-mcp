//go:build spike

package server

// The MCP Apps spike (#37): a throwaway view, built only with -tags spike, to
// find out what clients do with a view on every call of a tool. It never
// merges. Every request a client sends is logged to stderr and, when SPIKE_LOG
// names a file, to that file too, so a client's behaviour can be read back
// whether a person or a machine drove it.

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

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
	// Question 9: resources a view reads through the host, of any size, as
	// avatars and images would be. The text one carries a marker, to see
	// whether a view's read reaches the model or the stored conversation.
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: spikeBlobPrefix + "{size}",
		Name:        "spike-blob",
		Title:       "Spike: a blob of the given size",
		Description: "Spike for #37, read by the spike's view: random bytes, size in bytes.",
		MIMEType:    "application/octet-stream",
	}, spikeReadResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: spikeTextPrefix + "{size}",
		Name:        "spike-text",
		Title:       "Spike: a text of the given size",
		Description: "Spike for #37, read by the spike's view: text with a marker, size in bytes.",
		MIMEType:    "text/plain",
	}, spikeReadResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: spikeImagePrefix + "{size}",
		Name:        "spike-image",
		Title:       "Spike: an image of the given size",
		Description: "Spike for #37, read by the spike's view: a PNG of about the size in bytes.",
		MIMEType:    "image/png",
	}, spikeReadResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: spikeImageTextPrefix + "{size}",
		Name:        "spike-image-text",
		Title:       "Spike: an image as base64 text",
		Description: "Spike for #37, read by the spike's view: a PNG of about the size in bytes, base64 in a text resource.",
		MIMEType:    "text/plain",
	}, spikeReadResource)
	spikeLog.Printf("registered: resource %s, tools spike_show_post, spike_press (app only), spike_token (app only), spike_ask, spike_write; resource templates %s{size}, %s{size}; filtering app-only tools by declared views: %t",
		spikeURI, spikeBlobPrefix, spikeTextPrefix, spikeFilter)
}

const (
	spikeBlobPrefix      = "mattermost://spike/blob/"
	spikeTextPrefix      = "mattermost://spike/text/"
	spikeImagePrefix     = "mattermost://spike/image/"
	spikeImageTextPrefix = "mattermost://spike/imagetext/"
	spikeMaxSize         = 64 << 20
)

// spikeFilter hides the app-only tools from a tools/list whose request did not
// declare views, as ADR-022 plans, to see whether a client that declares views
// only on some requests can still call them from its view (question 10).
var spikeFilter = os.Getenv("SPIKE_FILTER_APP_TOOLS") == "1"

func spikeReadResource(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := request.Params.URI
	views, declared := spikeDeclared(request)
	text := strings.HasPrefix(uri, spikeTextPrefix)
	image := strings.HasPrefix(uri, spikeImagePrefix)
	imageText := strings.HasPrefix(uri, spikeImageTextPrefix)
	uri2 := strings.TrimPrefix(uri, spikeImageTextPrefix)
	size, err := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(uri2, spikeBlobPrefix), spikeTextPrefix), spikeImagePrefix))
	if err != nil || size < 0 || size > spikeMaxSize {
		spikeLog.Printf("resources/read %s: refused, size must be 0 to %d", uri, spikeMaxSize)
		return nil, mcp.ResourceNotFoundError(uri)
	}
	contents := &mcp.ResourceContents{URI: uri}
	if imageText {
		contents.MIMEType = "text/plain"
		contents.Text = base64.StdEncoding.EncodeToString(spikePNG(size))
		spikeLog.Printf("resources/read %s: PNG of %d bytes as %d base64 characters; views declared on this request: %t (%s)", uri, size, len(contents.Text), views, declared)
	} else if text {
		marker := spikeCanary("SPIKE-RES-")
		contents.MIMEType = "text/plain"
		contents.Text = strings.Repeat(marker+" ", size/len(marker+" ")+1)[:size]
		spikeLog.Printf("resources/read %s: text of %d bytes with %s; views declared on this request: %t (%s)", uri, size, marker, views, declared)
	} else if image {
		contents.MIMEType = "image/png"
		contents.Blob = spikePNG(size)
		spikeLog.Printf("resources/read %s: PNG of %d bytes; views declared on this request: %t (%s)", uri, len(contents.Blob), views, declared)
	} else {
		contents.MIMEType = "application/octet-stream"
		contents.Blob = make([]byte, size)
		_, _ = rand.Read(contents.Blob)
		spikeLog.Printf("resources/read %s: blob of %d bytes; views declared on this request: %t (%s)", uri, size, views, declared)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
}

// spikeDeclared reports whether any request declared views: on protocol
// 2026-07-28 from the request's own _meta, before it from initialize.
func spikeDeclared(request mcp.Request) (bool, string) {
	carrier, ok := request.(interface {
		ClientCapabilities() *mcp.ClientCapabilities
	})
	if !ok {
		return false, "not a request from the client"
	}
	return spikeViewsIn(carrier.ClientCapabilities())
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
		views, declared := spikeDeclared(request)
		spikeLog.Printf("received %s %s", method, text)
		spikeLog.Printf("  views declared on this %s: %t (%s)", method, views, declared)
		result, err := next(ctx, method, request)
		if err != nil {
			spikeLog.Printf("answered %s with error: %v", method, err)
		}
		if listed, ok := result.(*mcp.ListToolsResult); ok && err == nil {
			if spikeFilter && !views {
				kept := listed.Tools[:0:0]
				for _, tool := range listed.Tools {
					if !spikeIsAppOnly(tool) {
						kept = append(kept, tool)
					}
				}
				listed.Tools = kept
			}
			names := make([]string, 0, len(listed.Tools))
			for _, tool := range listed.Tools {
				names = append(names, tool.Name)
			}
			spikeLog.Printf("  listed %d tools: %s", len(names), strings.Join(names, ", "))
		}
		return result, err
	}
}

func spikeIsAppOnly(tool *mcp.Tool) bool {
	ui, _ := tool.Meta["ui"].(map[string]any)
	visibility, _ := ui["visibility"].([]string)
	return len(visibility) == 1 && visibility[0] == "app"
}

// spikeViews reports whether the client declared it renders MCP Apps views.
func spikeViews(request *mcp.CallToolRequest) (bool, string) {
	return spikeDeclared(request)
}

func spikeViewsIn(capabilities *mcp.ClientCapabilities) (bool, string) {
	// Per request: clients on protocol 2026-07-28 send no initialize, and put
	// their capabilities in each request's _meta.
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

// spikePNG is a valid PNG of about size bytes: a 1x1 image followed by a
// private ancillary chunk of random bytes, which decoders skip.
func spikePNG(size int) []byte {
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	pixel := encoded.Bytes()
	pad := size - len(pixel) - 12
	if pad <= 0 {
		return pixel
	}
	chunk := make([]byte, 8+pad+4)
	binary.BigEndian.PutUint32(chunk, uint32(pad))
	copy(chunk[4:], "spKe")
	_, _ = rand.Read(chunk[8 : 8+pad])
	binary.BigEndian.PutUint32(chunk[8+pad:], crc32.ChecksumIEEE(chunk[4:8+pad]))
	end := len(pixel) - 12
	out := append([]byte{}, pixel[:end]...)
	out = append(out, chunk...)
	return append(out, pixel[end:]...)
}
