//go:build live

package live

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The client check (ADR-022): what check_views answers, and what diagnose says
// reached the server, against a real Mattermost.

func TestCheckViewsGivesThePersonsPictureAndDiagnoseSaysWhatReachedTheServer(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	token := personalAccessToken(t, admin, user.Id).Token
	session := viewsSession(t, token)

	result := callTool(t, session, &mcp.CallToolParams{Name: "check_views"})
	var out server.ViewsCheck
	structured(t, result, &out)
	if out.Stamp != views.Stamp() || out.CarriesPictures {
		t.Errorf("check_views answered %+v", out)
	}
	if want := liveResource("/users/" + user.Id + "/image"); out.AvatarURI != want {
		t.Errorf("the person's picture is at %q, want %q", out.AvatarURI, want)
	}
	if text := result.Content[0].(*mcp.TextContent).Text; !strings.HasPrefix(text, "Shown to the person") || !strings.Contains(text, "diagnose") {
		t.Errorf("check_views told the model %q", text)
	}

	// The address the check reads is a picture Mattermost holds.
	picture, err := readResource(t, session, "picture", out.AvatarURI)
	if err != nil {
		t.Fatalf("the person's picture: %v", err)
	}
	if !strings.HasPrefix(picture.Contents[0].MIMEType, "image/") || len(picture.Contents[0].Blob) == 0 {
		t.Errorf("the person's picture reads as %s, %d bytes", picture.Contents[0].MIMEType, len(picture.Contents[0].Blob))
	}

	// What a view reads and calls is counted, and reaches nothing in Mattermost.
	for _, fixture := range views.CheckFixtures() {
		if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: fixture.URI}); err != nil {
			t.Errorf("%s: %v", fixture.Name, err)
		}
	}
	var seen server.ViewCheckSeen
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "view_check"}), &seen)
	if seen.FixtureReads != len(views.CheckFixtures()) || seen.Calls != 1 {
		t.Errorf("view_check has seen %+v", seen)
	}
	if reached := lastReached(t, session); len(reached) != 0 {
		t.Errorf("view_check reached %v", reached)
	}
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "view_check", Arguments: map[string]any{"report": `{"check":"mm-mcp client check"}`}}), &seen)

	report := diagnosed(t, session, token, false)
	check := checkNamed(t, report, "view check")
	if !strings.Contains(check.Detail, "3 of the check's reads and 2 of its calls") || !strings.Contains(check.Detail, `{"check":"mm-mcp client check"}`) {
		t.Errorf("diagnose says %q", check.Detail)
	}
	if declared := checkNamed(t, report, "client capabilities"); !strings.Contains(declared.Detail, views.MIMEType) {
		t.Errorf("diagnose says the client declares %q", declared.Detail)
	}
	if got := checkNamed(t, report, "views"); !strings.Contains(got.Detail, "check_views measures it") {
		t.Errorf("the views check says %q", got.Detail)
	}
}

func TestCheckViewsReadsNothingForAClientWithoutViews(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	session := sessionFor(t, admin, seedUser(t, admin))
	result := callTool(t, session, &mcp.CallToolParams{Name: "check_views"})
	if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "renders no MCP Apps views") {
		t.Errorf("check_views answered %+v", result.Content)
	}
	if reached := lastReached(t, session); len(reached) != 0 {
		t.Errorf("check_views read %v for a client that renders no views", reached)
	}
}
