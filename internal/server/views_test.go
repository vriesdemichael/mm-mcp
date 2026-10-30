package server_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// MCP Apps views (ADR-022): what a server offers a client that renders them,
// and what it offers one that does not. Nothing here reaches Mattermost: the
// unit tests block the network, and a client without views is read nothing.

func TestTheServerOffersViewsAndServesItsPageUnlisted(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	if _, ok := session.InitializeResult().Capabilities.Extensions[views.Extension]; !ok {
		t.Errorf("the server does not offer %s: %+v", views.Extension, session.InitializeResult().Capabilities)
	}
	listed, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range listed.Resources {
		if strings.HasPrefix(resource.URI, "ui://") {
			t.Errorf("the page is listed as a resource to attach: %+v", resource)
		}
	}
	read, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: views.URI})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Contents) != 1 {
		t.Fatalf("the page reads as %+v", read.Contents)
	}
	page := read.Contents[0]
	if page.MIMEType != views.MIMEType || !strings.Contains(page.Text, `const RESOURCE_BASE = "mattermost://chat.example.com";`) {
		t.Errorf("the page is %s, %d characters, and does not know its Mattermost's resources", page.MIMEType, len(page.Text))
	}
	policy, _ := json.Marshal(page.Meta["ui"])
	if !strings.Contains(string(policy), `"csp":{}`) {
		t.Errorf("the page declares %s; it reaches no network", policy)
	}
}

func TestOnlyAClientThatRendersViewsIsOfferedTheToolOnlyViewsCall(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		options *mcp.ClientOptions
		offered bool
	}{
		{"a client that declares nothing", nil, false},
		{"a client that renders other views", clientDeclaring(map[string]any{"mimeTypes": []string{"text/html;profile=something-else"}}), false},
		{"a client that renders views", rendersViews(), true},
		{"a client that names no types", clientDeclaring(map[string]any{}), true},
	} {
		listed, err := connectWith(t, readOnlyConfig, c.options).ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, tool := range listed.Tools {
			names[tool.Name] = true
		}
		if !names["show"] {
			t.Errorf("%s is not offered show, which every client is", c.name)
		}
		if names["view_action"] != c.offered {
			t.Errorf("%s: view_action offered %t, want %t", c.name, names["view_action"], c.offered)
		}
	}
}

func clientDeclaring(settings map[string]any) *mcp.ClientOptions {
	capabilities := &mcp.ClientCapabilities{}
	capabilities.AddExtension(views.Extension, settings)
	return &mcp.ClientOptions{Capabilities: capabilities}
}

func TestAClientWithoutViewsIsShownNothingAndReadNothing(t *testing.T) {
	t.Parallel()
	session := connect(t, readOnlyConfig)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "show", Arguments: map[string]any{"kind": "thread", "post_id": "abcdefghijklmnopqrstuvwxyz"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || len(result.Content) != 1 || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "renders no MCP Apps views") {
		t.Errorf("show answered a client without views with %+v", result.Content)
	}
	result, err = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "view_action", Arguments: map[string]any{"action": "more", "post_id": "abcdefghijklmnopqrstuvwxyz"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "called by mm-mcp's views alone") {
		t.Errorf("view_action answered a client without views with %+v", result.Content)
	}
}

func TestShowLinksThePageAndTheToolOnlyViewsCallIsMarkedForThem(t *testing.T) {
	t.Parallel()
	tools := map[string]*mcp.Tool{}
	for _, tool := range listTools(t, writingConfig) {
		tools[tool.Name] = tool
	}
	ui := func(name string) map[string]any {
		data, _ := json.Marshal(tools[name].Meta["ui"])
		var out map[string]any
		_ = json.Unmarshal(data, &out)
		return out
	}
	if got := ui("show")["resourceUri"]; got != views.URI || tools["show"].Meta["ui/resourceUri"] != views.URI {
		t.Errorf("show links %v", tools["show"].Meta)
	}
	for _, name := range []string{"view_action", "view_change"} {
		if got, _ := ui(name)["visibility"].([]any); !slices.Equal(got, []any{"app"}) {
			t.Errorf("%s is visible to %v; it is for views alone", name, got)
		}
		if _, linked := ui(name)["resourceUri"]; linked {
			t.Errorf("%s links the page; a call of it draws no view of its own", name)
		}
	}
	kinds, _ := json.Marshal(tools["show"].InputSchema)
	if !strings.Contains(string(kinds), `"enum":["thread","unreads"]`) {
		t.Errorf("show's kinds are %s", kinds)
	}
}

// What show and view_action refuse, they refuse before Mattermost is reached:
// the unit tests block the network.
func TestShowAndTheViewsToolRefuseWhatTheyDoNotDo(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	for _, c := range []struct {
		tool      string
		arguments map[string]any
	}{
		{"show", map[string]any{"kind": "list", "post_id": "abcdefghijklmnopqrstuvwxyz"}},
		{"show", map[string]any{"kind": "thread", "post_id": "abcdefghijklmnopqrstuvwxyz", "limit": 500}},
		{"view_action", map[string]any{"action": "post", "post_id": "abcdefghijklmnopqrstuvwxyz"}},
		{"view_action", map[string]any{"action": "more", "post_id": "abcdefghijklmnopqrstuvwxyz", "cursor": "not-a-cursor"}},
	} {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: c.tool, Arguments: c.arguments})
		if err == nil && !result.IsError {
			t.Errorf("%s %v was answered", c.tool, c.arguments)
		}
	}
}
