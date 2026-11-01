package server_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/doctor"
	"github.com/vriesdemichael/mm-mcp/internal/server"
	"github.com/vriesdemichael/mm-mcp/internal/views"
)

// The client check (ADR-022): what a server offers of it, and what reaches a
// server of a view's reads and calls. Nothing here reaches Mattermost: the unit
// tests block the network, so the person cannot be read.

func TestTheCheckPageDeclaresTheServersAddressAndIsNotListed(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	listed, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range listed.Resources {
		if strings.HasPrefix(resource.URI, views.CheckURI) {
			t.Errorf("the check is listed as a resource to attach: %+v", resource)
		}
	}
	read, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: views.CheckURI})
	if err != nil {
		t.Fatal(err)
	}
	page := read.Contents[0]
	if page.MIMEType != views.MIMEType || !strings.Contains(page.Text, `const SERVER = "https://chat.example.com";`) {
		t.Errorf("the check page is %s, %d characters, and does not know its Mattermost", page.MIMEType, len(page.Text))
	}
	policy, _ := json.Marshal(page.Meta["ui"])
	for _, want := range []string{`"connectDomains":["https://chat.example.com"]`, `"resourceDomains":["https://chat.example.com"]`, `"clipboardWrite":{}`} {
		if !strings.Contains(string(policy), want) {
			t.Errorf("the check page declares %s, without %s", policy, want)
		}
	}
	// The page that draws Mattermost declares none of it.
	read, err = session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: views.URI})
	if err != nil {
		t.Fatal(err)
	}
	if drawing, _ := json.Marshal(read.Contents[0].Meta["ui"]); strings.Contains(string(drawing), "Domains") || strings.Contains(string(drawing), "clipboard") {
		t.Errorf("the page that draws views declares %s", drawing)
	}
}

func TestEachFixtureReadsAsWhatThePageExpects(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	for _, fixture := range views.CheckFixtures() {
		read, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: fixture.URI})
		if err != nil {
			t.Errorf("%s: %v", fixture.Name, err)
			continue
		}
		content := read.Contents[0]
		if content.MIMEType != fixture.MIMEType || content.Text != fixture.Text || string(content.Blob) != string(fixture.Blob) {
			t.Errorf("%s reads as %s, %q, %d bytes", fixture.Name, content.MIMEType, content.Text, len(content.Blob))
		}
	}
}

func TestCheckViewsToAClientWithoutViewsShowsNothing(t *testing.T) {
	t.Parallel()
	result, err := connect(t, readOnlyConfig).CallTool(t.Context(), &mcp.CallToolParams{Name: "check_views"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "renders no MCP Apps views") {
		t.Errorf("check_views answered a client without views with %+v", result.Content)
	}
}

// A check of the client is worth having when Mattermost is what is failing.
func TestCheckViewsLinksItsPageAndWorksWithoutMattermost(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "check_views"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("check_views answered %+v", result.Content)
	}
	data, _ := json.Marshal(result.StructuredContent)
	var out server.ViewsCheck
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Stamp != views.Stamp() || out.AvatarURI != "" || out.CarriesPictures {
		t.Errorf("check_views answered %+v", out)
	}
	// Claude Code hands its model the structured answer, and not the text.
	if !strings.HasPrefix(out.Next, "Shown to the person") || !strings.Contains(out.Next, "ask the person to paste") {
		t.Errorf("check_views tells a model that reads only its structured answer %q", out.Next)
	}
	carried, _ := json.Marshal(result.Meta["mm-mcp/check"])
	if !strings.Contains(string(carried), views.Stamp()) {
		t.Errorf("its result's _meta holds %s", carried)
	}
	tools := listTools(t, readOnlyConfig)
	for _, tool := range tools {
		if tool.Name != "check_views" {
			continue
		}
		link, _ := json.Marshal(tool.Meta["ui"])
		if !strings.Contains(string(link), views.CheckURI) || tool.Meta["ui/resourceUri"] != views.CheckURI {
			t.Errorf("check_views links %v", tool.Meta)
		}
	}
}

func checkNamed(t *testing.T, result doctor.Result, name string) doctor.Check {
	t.Helper()
	for _, check := range result.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no %s check in %+v", name, result.Checks)
	return doctor.Check{}
}

func diagnoseThrough(t *testing.T, session *mcp.ClientSession) doctor.Result {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "diagnose"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result.StructuredContent)
	var out doctor.Result
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func viewCheckSeen(t *testing.T, session *mcp.ClientSession, arguments map[string]any) server.ViewCheckSeen {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "view_check", Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("view_check answered %+v", result.Content)
	}
	data, _ := json.Marshal(result.StructuredContent)
	var seen server.ViewCheckSeen
	if err := json.Unmarshal(data, &seen); err != nil {
		t.Fatal(err)
	}
	return seen
}

func TestDiagnoseSaysWhatReachedTheServerAndWhatTheCheckReported(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())

	// Nothing yet: it says so, and how to find out.
	before := checkNamed(t, diagnoseThrough(t, session), "view check")
	if !strings.Contains(before.Detail, "0 of the check's reads and 0 of its calls") || !strings.Contains(before.Detail, "the check's page has not been read") {
		t.Errorf("before anything, diagnose says %q", before.Detail)
	}
	if !strings.Contains(before.Next, "check_views measures it") {
		t.Errorf("before anything, diagnose advises %q", before.Next)
	}

	// The check's reads and calls are counted, and the view is told.
	for _, fixture := range views.CheckFixtures() {
		if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: fixture.URI}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: views.CheckURI}); err != nil {
		t.Fatal(err)
	}
	if seen := viewCheckSeen(t, session, nil); seen.FixtureReads != len(views.CheckFixtures()) || seen.Calls != 1 {
		t.Errorf("view_check says it has seen %+v", seen)
	}
	viewCheckSeen(t, session, map[string]any{"report": "the check's report"})

	after := checkNamed(t, diagnoseThrough(t, session), "view check")
	if !strings.Contains(after.Detail, "3 of the check's reads and 2 of its calls") {
		t.Errorf("afterwards, diagnose says %q", after.Detail)
	}
	if !strings.Contains(after.Detail, "the check's page was read at") || !strings.Contains(after.Detail, "the page that draws views has not been read") {
		t.Errorf("diagnose has the pages read wrong: %q", after.Detail)
	}
	if !strings.Contains(after.Detail, "as the page wrote it: the check's report") {
		t.Errorf("diagnose does not give the report: %q", after.Detail)
	}

	// One server's check is not another's.
	other := checkNamed(t, diagnoseThrough(t, connectWith(t, readOnlyConfig, rendersViews())), "view check")
	if !strings.Contains(other.Detail, "0 of the check's reads") || strings.Contains(other.Detail, "the check's report") {
		t.Errorf("another server says %q", other.Detail)
	}
}

func TestAReportTooLongForTheModelIsRefused(t *testing.T) {
	t.Parallel()
	session := connectWith(t, readOnlyConfig, rendersViews())
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "view_check", Arguments: map[string]any{"report": strings.Repeat("x", 16<<10+1)}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "at most") {
		t.Errorf("view_check answered %+v", result.Content)
	}
	if got := checkNamed(t, diagnoseThrough(t, session), "view check"); strings.Contains(got.Detail, "as the page wrote it") {
		t.Errorf("the report was kept: %q", got.Detail)
	}
}

func TestDiagnoseSaysWhatTheClientDeclaresAndOnlyAClientWithViewsHearsOfTheCheck(t *testing.T) {
	t.Parallel()
	got := diagnoseThrough(t, connectWith(t, readOnlyConfig, rendersViews()))
	declared := checkNamed(t, got, "client capabilities")
	if !strings.Contains(declared.Detail, views.Extension) || !strings.Contains(declared.Detail, views.MIMEType) {
		t.Errorf("it says the client declares %q", declared.Detail)
	}
	if detail := checkNamed(t, got, "views").Detail; !strings.Contains(detail, "declares that it renders") || !strings.Contains(detail, "check_views measures it") {
		t.Errorf("the views check says %q", detail)
	}

	got = diagnoseThrough(t, connect(t, readOnlyConfig))
	for _, check := range got.Checks {
		if check.Name == "view check" {
			t.Errorf("a client without views hears %q", check.Detail)
		}
	}
}
