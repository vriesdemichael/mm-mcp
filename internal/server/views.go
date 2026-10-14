package server

import (
	"context"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// MCP Apps views (ADR-022). One page, internal/views, draws every view; show
// is the tool whose call puts a view in front of the person, and view_action
// the one tool a view calls for what the person does in it. Whether a client
// renders views is read from each request, since a client may declare it on
// every request rather than once.

// rendersViews reports whether the client declared, on this request, that it
// renders MCP Apps views of the page's type. A client that names no types is
// taken to render the one MCP Apps defines.
func rendersViews(request mcp.Request) bool {
	carrier, ok := request.(interface {
		ClientCapabilities() *mcp.ClientCapabilities
	})
	if !ok {
		return false
	}
	capabilities := carrier.ClientCapabilities()
	if capabilities == nil {
		return false
	}
	settings, ok := capabilities.Extensions[views.Extension]
	if !ok {
		return false
	}
	values, ok := settings.(map[string]any)
	if !ok || values["mimeTypes"] == nil {
		return true
	}
	switch types := values["mimeTypes"].(type) {
	case []string:
		return slices.Contains(types, views.MIMEType)
	case []any:
		return slices.Contains(types, any(views.MIMEType))
	}
	return false
}

// declaredViewTypes is what the client declared of views on this request, as
// diagnose reports it: nil when it declared nothing.
func declaredViewTypes(request mcp.Request) []string {
	carrier, ok := request.(interface {
		ClientCapabilities() *mcp.ClientCapabilities
	})
	if !ok || carrier.ClientCapabilities() == nil {
		return nil
	}
	settings, ok := carrier.ClientCapabilities().Extensions[views.Extension]
	if !ok {
		return nil
	}
	found := []string{}
	if values, ok := settings.(map[string]any); ok {
		switch types := values["mimeTypes"].(type) {
		case []string:
			found = append(found, types...)
		case []any:
			for _, value := range types {
				if text, ok := value.(string); ok {
					found = append(found, text)
				}
			}
		}
	}
	return found
}

// viewToolMeta links a tool to the page, nested as MCP Apps defines it and
// flat as hosts on an earlier draft read it.
func viewToolMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"resourceUri": views.URI}, "ui/resourceUri": views.URI}
}

// appOnlyMeta marks a tool for views alone: a host that follows MCP Apps
// leaves it out of what its model is offered.
func appOnlyMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
}

// isAppOnly reports whether a tool is for views alone.
func isAppOnly(tool *mcp.Tool) bool {
	ui, _ := tool.Meta["ui"].(map[string]any)
	visibility, _ := ui["visibility"].([]string)
	return len(visibility) == 1 && visibility[0] == "app"
}

// pageMeta says what the page may do: reach no network, which an empty
// policy says, and sit in a frame with a border.
func pageMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"csp": map[string]any{}, "prefersBorder": true}}
}

// registerView adds the page, and says the server offers views. The page is
// read by a host that renders the view; it is left out of the resources a
// client lists, where it would be offered as something to attach.
func registerView(server *mcp.Server, cfg config.Config) {
	page := views.Page(resourceBase(serverAddress(cfg)))
	server.AddResource(&mcp.Resource{
		URI:         views.URI,
		Name:        "view",
		Title:       "Mattermost view",
		Description: "The page mm-mcp's MCP Apps views are drawn in.",
		MIMEType:    views.MIMEType,
		Meta:        pageMeta(),
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: views.MIMEType, Text: page, Meta: pageMeta(),
		}}}, nil
	})
	server.AddReceivingMiddleware(viewsMiddleware)
}

// viewsMiddleware keeps what is for views alone from what a client lists: the
// page from its resources, and from a request that declared no views, the
// tool only views call. A client that does not know MCP Apps ignores the mark
// that keeps that tool from its model, and would offer its model a tool it
// has no use for (ADR-022).
func viewsMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil {
			return result, err
		}
		// A new list each time: the one answered with may be the server's own.
		switch listed := result.(type) {
		case *mcp.ListResourcesResult:
			kept := make([]*mcp.Resource, 0, len(listed.Resources))
			for _, resource := range listed.Resources {
				if !strings.HasPrefix(resource.URI, "ui://") {
					kept = append(kept, resource)
				}
			}
			listed.Resources = kept
		case *mcp.ListToolsResult:
			if !rendersViews(request) {
				kept := make([]*mcp.Tool, 0, len(listed.Tools))
				for _, tool := range listed.Tools {
					if !isAppOnly(tool) {
						kept = append(kept, tool)
					}
				}
				listed.Tools = kept
			}
		}
		return result, err
	}
}
