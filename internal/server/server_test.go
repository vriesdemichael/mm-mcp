package server_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
	"github.com/vriesdemichael/mm-mcp/internal/network"
	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// What the server offers a client, read through an in-memory MCP client. No
// tool is called here: a call reaches Mattermost, and that is the live suite's
// (ADR-004).

var readOnlyConfig = config.Config{URL: "https://chat.example.com", Token: "t"}

func connect(t *testing.T, cfg config.Config) *mcp.ClientSession {
	t.Helper()
	client := mattermost.New(cfg.URL, cfg.Token, network.NewSafeTransport())
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.New(cfg, server.Single(client)).Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listTools(t *testing.T, cfg config.Config) []*mcp.Tool {
	t.Helper()
	result, err := connect(t, cfg).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return result.Tools
}

func TestTheServerIntroducesItself(t *testing.T) {
	t.Parallel()
	result := connect(t, readOnlyConfig).InitializeResult()
	if result.ServerInfo.Name != server.Name || result.Instructions != server.Instructions {
		t.Fatalf("got %+v", result)
	}
}

// A server that does not ask says so; one that cannot write has nothing to say.
func TestAServerThatDoesNotAskSaysSo(t *testing.T) {
	t.Parallel()
	skipping := config.Config{URL: "https://chat.example.com", Token: "t", AllowWrites: true, SkipAsking: true}
	if got := connect(t, skipping).InitializeResult().Instructions; got != server.Instructions+"\n\n"+server.SkippingInstructions {
		t.Errorf("writing without asking: %q", got)
	}
	skipping.AllowWrites = false
	if got := connect(t, skipping).InitializeResult().Instructions; got != server.Instructions {
		t.Errorf("read-only: %q", got)
	}
}

func TestGetMeIsOfferedWithAnOutputSchema(t *testing.T) {
	t.Parallel()
	for _, tool := range listTools(t, readOnlyConfig) {
		if tool.Name == "get_me" {
			if tool.OutputSchema == nil {
				t.Fatal("get_me declares no output schema")
			}
			return
		}
	}
	t.Fatal("get_me is not offered")
}

// A file is offered as a resource on every server, read-only or not, under
// an address of its own scheme that names the server (ADR-029).
func TestFilesAndPicturesAreOfferedAsResourcesOfTheirServer(t *testing.T) {
	t.Parallel()
	for _, cfg := range []config.Config{readOnlyConfig, writingConfig, {URL: "http://Localhost:8065/mm/", Token: "t"}} {
		session := connect(t, cfg)
		if session.InitializeResult().Capabilities.Completions == nil {
			t.Errorf("%s: no completions offered", cfg.URL)
		}
		result, err := session.ListResourceTemplates(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		base := map[string]string{
			readOnlyConfig.URL:          "mattermost://chat.example.com",
			"http://Localhost:8065/mm/": "mattermost://localhost:8065/mm",
		}[cfg.URL]
		want := map[string]string{
			"file":    base + "/files/{file_id}",
			"preview": base + "/files/{file_id}/preview",
			"picture": base + "/users/{user_id}/image",
			"emoji":   base + "/emoji/{emoji_name}",
		}
		got := map[string]string{}
		for _, template := range result.ResourceTemplates {
			got[template.Name] = template.URITemplate
		}
		if !maps.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", cfg.URL, got, want)
		}
	}
}

// What is not a file or a picture of this server is not found, and nothing
// asks Mattermost: the unit tests block the network, which would fail
// otherwise.
func TestAResourceThatIsNothingOfThisServerIsNotFound(t *testing.T) {
	t.Parallel()
	session := connect(t, readOnlyConfig)
	for _, uri := range []string{
		"mattermost://chat.example.com/files/not-an-id",
		"mattermost://other.example.com/files/abcdefghijklmnopqrstuvwxyz",
		"mattermost://chat.example.com/posts/abcdefghijklmnopqrstuvwxyz",
		"mattermost://chat.example.com/files/not-an-id/preview",
		"mattermost://other.example.com/files/abcdefghijklmnopqrstuvwxyz/preview",
		"mattermost://chat.example.com/users/not-an-id/image",
		"mattermost://chat.example.com/users/abcdefghijklmnopqrstuvwxyz/picture",
		"mattermost://other.example.com/users/abcdefghijklmnopqrstuvwxyz/image",
		"mattermost://chat.example.com/emoji/not%20a%20name",
		"mattermost://chat.example.com/emoji/a/b",
		"mattermost://other.example.com/emoji/tada",
	} {
		_, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		var refused *jsonrpc.Error
		if !errors.As(err, &refused) || refused.Code != jsonrpc.CodeInvalidParams || refused.Message != "Resource not found" {
			t.Errorf("%s: got %v; want resource not found", uri, err)
		}
	}
}

// Only the file template's argument is completed; anything else gets no
// suggestion, without asking Mattermost.
func TestOnlyAFilesArgumentIsCompleted(t *testing.T) {
	t.Parallel()
	session := connect(t, readOnlyConfig)
	for _, params := range []*mcp.CompleteParams{
		{Ref: &mcp.CompleteReference{Type: "ref/resource", URI: "mattermost://other.example.com/files/{file_id}"}, Argument: mcp.CompleteParamsArgument{Name: "file_id", Value: "report"}},
		{Ref: &mcp.CompleteReference{Type: "ref/resource", URI: "mattermost://chat.example.com/files/{file_id}"}, Argument: mcp.CompleteParamsArgument{Name: "other", Value: "report"}},
		{Ref: &mcp.CompleteReference{Type: "ref/resource", URI: "mattermost://chat.example.com/files/{file_id}"}, Argument: mcp.CompleteParamsArgument{Name: "file_id", Value: " "}},
		{Ref: &mcp.CompleteReference{Type: "ref/resource", URI: "mattermost://chat.example.com/files/{file_id}"}, Argument: mcp.CompleteParamsArgument{Name: "file_id", Value: "abcdefghijklmnopqrstuvwxyz"}},
		{Ref: &mcp.CompleteReference{Type: "ref/prompt", Name: "anything"}, Argument: mcp.CompleteParamsArgument{Name: "file_id", Value: "report"}},
	} {
		result, err := session.Complete(t.Context(), params)
		if err != nil || len(result.Completion.Values) != 0 {
			t.Errorf("%+v %+v: got %+v, %v", params.Ref, params.Argument, result, err)
		}
	}
}

// A resource's message is capped unless the server is told otherwise, and its
// template says by how much (ADR-029).
func TestAResourceTemplateSaysWhatBoundsIt(t *testing.T) {
	t.Parallel()
	for cfg, want := range map[*config.Config]string{
		&readOnlyConfig: "A resource is read whole, in one message of up to 10.0 MiB; read_file",
		{URL: "https://chat.example.com", Token: "t", ResourceMaxMB: 4}:  "A resource is read whole, in one message of up to 4.0 MiB; read_file",
		{URL: "https://chat.example.com", Token: "t", ResourceMaxMB: -1}: "A resource is read whole; read_file",
	} {
		result, err := connect(t, *cfg).ListResourceTemplates(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, template := range result.ResourceTemplates {
			if got := template.Description; template.Name == "file" && !strings.Contains(got, want) {
				t.Errorf("%v: %q does not say %q", cfg, got, want)
			}
		}
	}
}
