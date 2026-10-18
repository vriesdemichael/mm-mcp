package server

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// declaring is a call whose request declares capabilities, as a client on
// the 2026-07-28 revision declares them on each request.
func declaring(capabilities *mcp.ClientCapabilities) *mcp.CallToolRequest {
	meta := mcp.Meta{}
	if capabilities != nil {
		meta[mcp.MetaKeyClientCapabilities] = capabilities
	}
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Meta: meta}}
}

func withViews(settings map[string]any) *mcp.ClientCapabilities {
	capabilities := &mcp.ClientCapabilities{}
	capabilities.AddExtension(views.Extension, settings)
	return capabilities
}

func TestWhetherAClientRendersViewsIsReadFromItsRequest(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		request  *mcp.CallToolRequest
		renders  bool
		declared []string
		says     string
	}{
		{"a request with no capabilities", declaring(nil), false, nil, "renders no MCP Apps views"},
		{"a client without the extension", declaring(&mcp.ClientCapabilities{}), false, nil, "renders no MCP Apps views"},
		{"a client of mm-mcp's views", declaring(withViews(map[string]any{"mimeTypes": []string{views.MIMEType}})), true, []string{views.MIMEType}, "renders MCP Apps views, so show"},
		{"a client of other views", declaring(withViews(map[string]any{"mimeTypes": []string{"text/html;profile=other"}})), false, []string{"text/html;profile=other"}, "of text/html;profile=other, not of"},
		{"a client that names no types", declaring(withViews(map[string]any{})), true, []string{}, "renders MCP Apps views, so show"},
	} {
		if got := rendersViews(c.request); got != c.renders {
			t.Errorf("%s: renders views %t, want %t", c.name, got, c.renders)
		}
		if got := declaredViewTypes(c.request); !slices.Equal(got, c.declared) || (got == nil) != (c.declared == nil) {
			t.Errorf("%s: declares %#v, want %#v", c.name, got, c.declared)
		}
		if got := viewsCheck(c.request); !strings.Contains(got.Detail, c.says) {
			t.Errorf("%s: diagnose says %q", c.name, got.Detail)
		}
	}
}

func TestWhatIsForViewsIsKeptFromListsAndAnErrorPassesThrough(t *testing.T) {
	t.Parallel()
	failed := errors.New("refused")
	_, err := viewsMiddleware(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return nil, failed
	})(t.Context(), "tools/list", &mcp.ListToolsRequest{})
	if !errors.Is(err, failed) {
		t.Errorf("an error came back as %v", err)
	}
	listed, err := viewsMiddleware(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return &mcp.ListResourcesResult{Resources: []*mcp.Resource{{URI: views.URI}, {URI: "mattermost://chat.example.com/files/x"}}}, nil
	})(t.Context(), "resources/list", &mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resources := listed.(*mcp.ListResourcesResult).Resources; len(resources) != 1 || resources[0].URI == views.URI {
		t.Errorf("the resources listed are %+v", resources)
	}
}
