//go:build live && clients

package live

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/scriptedmodel"
	"github.com/vriesdemichael/mm-mcp/internal/testclients"
)

// The views in real clients (ADR-036): Goose's desktop app and VS Code, at the
// versions .github/tool-versions.env pins, each started with a debugging port
// and driven through clients/driver.mjs, with the scripted model as their
// model, so no inference is involved. mm-mcp runs as each client starts it,
// over stdio, against the live instance. `task test:clients` installs the
// clients and the driver first; these tests carry the clients tag as well as
// live, so `task test:live` never runs them.

// clientsRoot is the checkout, where the clients are installed.
const clientsRoot = "../.."

// mmMCP is mm-mcp, built once for every client the tests start.
var mmMCP = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "mm-mcp-clients-")
	if err != nil {
		return "", err
	}
	program := filepath.Join(dir, "mm-mcp")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	output, err := exec.Command("go", "build", "-o", program, "../../cmd/mm-mcp").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("building mm-mcp: %w: %s", err, output)
	}
	return program, nil
})

// clientRun is a client the test started, and the model it talks to.
type clientRun struct {
	t      *testing.T
	client string
	port   int
	model  *scriptedmodel.Model
	// logs holds mm-mcp's and the client's output, kept on failure.
	logs string
}

// clientOptions says how a client is set up.
type clientOptions struct {
	// approve has Goose ask before each of the model's calls, its approve mode.
	approve bool
	// writes has mm-mcp allow writes, as MM_MCP_ALLOW_WRITES does.
	writes bool
}

// startClient starts a client against the scripted model, with mm-mcp acting
// as the token's user, and ends it when the test ends.
func startClient(t *testing.T, client, token string, options clientOptions) *clientRun {
	t.Helper()
	pinned, err := testclients.Pinned(clientsRoot)
	check(t, err)
	program, err := testclients.Executable(clientsRoot, client, pinned[client], testclients.This())
	check(t, err)
	server, err := mmMCP()
	check(t, err)

	model := &scriptedmodel.Model{}
	modelServer := httptest.NewServer(model.Handler())
	t.Cleanup(modelServer.Close)

	home := t.TempDir()
	logs := filepath.Join(home, "logs")
	check(t, os.MkdirAll(logs, 0o755))
	wrapper := writeWrapper(t, home, server, token, filepath.Join(logs, "mm-mcp.log"), options.writes)
	run := &clientRun{t: t, client: client, port: freePort(t), model: model, logs: logs}

	var args []string
	environment := append(os.Environ(), "SCRIPTED_MODEL_URL="+modelServer.URL)
	switch client {
	case testclients.Goose:
		writeGooseConfig(t, home, modelServer.URL, wrapper, options)
		environment = append(environment, "GOOSE_PATH_ROOT="+home, "GOOSE_DISABLE_KEYRING=1", "OPENAI_API_KEY=scripted")
		args = []string{"--no-sandbox", "--disable-gpu", "--user-data-dir=" + filepath.Join(home, "electron"), fmt.Sprintf("--remote-debugging-port=%d", run.port)}
	case testclients.VSCode:
		data := filepath.Join(home, "vscode")
		writeVSCodeSettings(t, data, wrapper)
		model, err := filepath.Abs(filepath.Join("clients", "vscode-model"))
		check(t, err)
		workspace := filepath.Join(home, "workspace")
		check(t, os.MkdirAll(workspace, 0o755))
		// --password-store=basic: without a keyring VS Code shows a native
		// dialog no debugging client can see, and the MCP server waits on it.
		args = []string{
			"--no-sandbox", "--disable-gpu", "--password-store=basic", fmt.Sprintf("--remote-debugging-port=%d", run.port),
			"--user-data-dir", data, "--extensions-dir", filepath.Join(data, "extensions"), "--extensionDevelopmentPath=" + model,
			"--skip-welcome", "--skip-release-notes", "--disable-workspace-trust", "--new-window", workspace,
		}
	}
	output, err := os.Create(filepath.Join(logs, client+".log"))
	check(t, err)
	command := exec.Command(program, args...)
	command.Env, command.Stdout, command.Stderr = environment, output, output
	inGroup(command)
	check(t, command.Start())
	t.Cleanup(func() {
		if t.Failed() {
			run.keep()
		}
		endGroup(command)
		_ = output.Close()
	})
	run.waitForDebugging()
	return run
}

// writeWrapper writes the program a client starts as mm-mcp: mm-mcp with its
// configuration, since Goose's desktop app passes on no environment of its
// own. It holds the token, in the test's own folder.
func writeWrapper(t *testing.T, home, server, token, log string, writes bool) string {
	t.Helper()
	settings := [][2]string{{"MM_URL", liveURL}, {"MM_TOKEN", token}, {"MM_MCP_ALLOW_WRITES", fmt.Sprint(writes)}}
	var script, path string
	if runtime.GOOS == "windows" {
		path = filepath.Join(home, "mm-mcp.cmd")
		script = "@echo off\r\n"
		for _, setting := range settings {
			script += fmt.Sprintf("set \"%s=%s\"\r\n", setting[0], setting[1])
		}
		script += fmt.Sprintf("\"%s\" serve 2>>\"%s\"\r\n", server, log)
	} else {
		path = filepath.Join(home, "mm-mcp.sh")
		script = "#!/bin/sh\n"
		for _, setting := range settings {
			script += fmt.Sprintf("export %s='%s'\n", setting[0], setting[1])
		}
		script += fmt.Sprintf("exec '%s' serve 2>>'%s'\n", server, log)
	}
	check(t, os.WriteFile(path, []byte(script), 0o700))
	return path
}

func writeGooseConfig(t *testing.T, home, model, wrapper string, options clientOptions) {
	t.Helper()
	mode := "auto"
	if options.approve {
		mode = "approve"
	}
	config := fmt.Sprintf(`GOOSE_PROVIDER: openai
GOOSE_MODEL: %[1]s
GOOSE_MODE: %[2]s
OPENAI_HOST: %[3]s
GOOSE_TELEMETRY_ENABLED: false
GOOSE_DISABLE_KEYRING: true
active_provider: openai
providers:
  openai:
    enabled: true
    model: %[1]s
    configured: true
extensions:
  %[4]s:
    enabled: true
    type: stdio
    name: %[4]s
    description: mm-mcp
    cmd: %[5]s
    args: []
    timeout: 300
`, scriptedmodel.Name, mode, model, scriptedmodel.Server, wrapper)
	path := filepath.Join(home, "config", "config.yaml")
	check(t, os.MkdirAll(filepath.Dir(path), 0o755))
	check(t, os.WriteFile(path, []byte(config), 0o600))
}

func writeVSCodeSettings(t *testing.T, data, wrapper string) {
	t.Helper()
	user := filepath.Join(data, "User")
	check(t, os.MkdirAll(user, 0o755))
	settings := map[string]any{
		"chat.mcp.autostart":               "never",
		"chat.mcp.apps.enabled":            true,
		"chat.tools.autoApprove":           true,
		"security.workspace.trust.enabled": false,
		"telemetry.telemetryLevel":         "off",
		"update.mode":                      "none",
		"extensions.autoUpdate":            false,
		"workbench.startupEditor":          "none",
		"chat.mcp.discovery.enabled":       map[string]bool{"claude-desktop": false, "copilot": false, "windsurf": false, "cursor-global": false, "cursor-workspace": false},
	}
	servers := map[string]any{"servers": map[string]any{scriptedmodel.Server: map[string]any{"type": "stdio", "command": wrapper, "args": []string{}}}}
	for name, value := range map[string]any{"settings.json": settings, "mcp.json": servers} {
		content, err := json.MarshalIndent(value, "", "  ")
		check(t, err)
		check(t, os.WriteFile(filepath.Join(user, name), content, 0o600))
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	check(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// waitForDebugging waits until the client answers on its debugging port.
func (c *clientRun) waitForDebugging() {
	c.t.Helper()
	address := fmt.Sprintf("http://127.0.0.1:%d/json/version", c.port)
	eventually(c.t, 2*time.Minute, func() (bool, string) {
		response, err := http.Get(address) //nolint:gosec,noctx // the client the test started, on the loopback address
		if err != nil {
			return false, err.Error()
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK, response.Status
	})
}

// drive runs one driver command and answers with what it printed.
func (c *clientRun) drive(command string, args map[string]any) map[string]any {
	c.t.Helper()
	out, err := c.tryDrive(command, args)
	if err != nil {
		c.t.Fatalf("%s %s: %v", c.client, command, err)
	}
	return out
}

func (c *clientRun) tryDrive(command string, args map[string]any) (map[string]any, error) {
	all := map[string]any{"client": c.client, "port": c.port, "server": scriptedmodel.Server}
	for name, value := range args {
		all[name] = value
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		return nil, err
	}
	run := exec.Command("node", filepath.Join("clients", "driver.mjs"), command, string(encoded))
	var stderr strings.Builder
	run.Stderr = &stderr
	printed, err := run.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var out map[string]any
	if err := json.Unmarshal(printed, &out); err != nil {
		return nil, fmt.Errorf("the driver printed %q: %w", printed, err)
	}
	return out, nil
}

// send has the person say message, and waits for the model's last answer.
func (c *clientRun) send(message string, args map[string]any) {
	c.t.Helper()
	all := map[string]any{"message": message}
	for name, value := range args {
		all[name] = value
	}
	c.drive("send", all)
}

// script is the message that has the scripted model call a tool.
func script(tool string, arguments map[string]any) string {
	encoded, _ := json.Marshal(arguments)
	return "SCRIPT " + tool + " " + string(encoded)
}

// keep saves a screenshot, the logs and the model's requests under
// .tmp/clients-artifacts, which CI uploads when the job fails.
func (c *clientRun) keep() {
	dir := filepath.Join(clientsRoot, ".tmp", "clients-artifacts", strings.ReplaceAll(c.t.Name(), "/", "-"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.t.Logf("keeping the artifacts: %v", err)
		return
	}
	shot, _ := filepath.Abs(filepath.Join(dir, "window.png"))
	if _, err := c.tryDrive("screenshot", map[string]any{"path": shot, "timeout": 15000}); err != nil {
		c.t.Logf("taking a screenshot: %v", err)
	}
	entries, _ := os.ReadDir(c.logs)
	for _, entry := range entries {
		if content, err := os.ReadFile(filepath.Join(c.logs, entry.Name())); err == nil {
			_ = os.WriteFile(filepath.Join(dir, entry.Name()), content, 0o600)
		}
	}
	// What the client put in front of the model, request by request.
	if requests, err := json.MarshalIndent(c.model.Requests(), "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "model-requests.json"), requests, 0o600)
	}
	c.t.Logf("kept a screenshot, the logs and the model's requests in %s", dir)
}

// offered is every tool the client put in front of the model, by its bare
// name.
func (c *clientRun) offered() []string {
	var names []string
	for _, request := range c.model.Requests() {
		for _, name := range request.Tools {
			if bare := scriptedmodel.Bare(name); !slices.Contains(names, bare) {
				names = append(names, bare)
			}
		}
	}
	slices.Sort(names)
	return names
}

// A thread is shown as Mattermost lays it out, in every client, and its view
// loads the rest of the thread through the tool only views call, which the
// model is never offered. read_post shows nothing.
func TestClientsShowAThreadAndItsViewLoadsTheRest(t *testing.T) {
	for _, client := range testclients.Clients {
		t.Run(client, func(t *testing.T) {
			admin := admin(t)
			user, other := seedUser(t, admin), seedUser(t, admin)
			team := seedTeam(t, admin, user, other)
			channel := seedChannel(t, admin, team, user, other)
			tag := word(t)
			author := clientAs(t, other)
			root := postAs(t, author, channel.Id, "", "Who reviews the "+tag+" notes?")
			first := postAs(t, clientAs(t, user), channel.Id, root.Id, "I can take the first half, "+tag)
			postAs(t, author, channel.Id, root.Id, "Then I take the second half, "+tag)
			last := postAs(t, author, channel.Id, root.Id, "Both halves are done, "+tag)
			alone := postAs(t, author, channel.Id, "", "Read but never shown, "+tag)

			run := startClient(t, client, personalAccessToken(t, admin, user.Id).Token, clientOptions{})
			run.drive("ready", nil)

			run.send(script("show", map[string]any{"kind": "thread", "post_id": root.Id, "limit": 2}), nil)
			shown := run.drive("view", map[string]any{"text": root.Message})
			mustContain(t, "the view", fmt.Sprint(shown["text"]), first.Message, "2 more replies")
			if strings.Contains(fmt.Sprint(shown["text"]), last.Message) {
				t.Errorf("the view shows the reply past its first page: %s", shown["text"])
			}

			more := run.drive("press", map[string]any{"text": root.Message, "button": "2 more replies", "until": last.Message})
			mustContain(t, "the view after more", fmt.Sprint(more["text"]), first.Message, last.Message)

			run.send(script("read_post", map[string]any{"post_id": alone.Id}), nil)
			for _, text := range run.drive("views", nil)["texts"].([]any) {
				if strings.Contains(fmt.Sprint(text), alone.Message) {
					t.Errorf("read_post left a view: %s", text)
				}
			}

			offered := run.offered()
			if !slices.Contains(offered, "show") || !slices.Contains(offered, "read_post") || slices.Contains(offered, "view_action") {
				t.Errorf("%s offered its model %v; want show and read_post, never view_action", client, offered)
			}
		})
	}
}

// In Goose's approve mode every call a view makes is refused, so the view
// says it cannot load more here and offers the thread in Mattermost.
func TestClientsGooseInApproveModeRefusesAViewsCall(t *testing.T) {
	admin := admin(t)
	user := seedUser(t, admin)
	team := seedTeam(t, admin, user)
	channel := seedChannel(t, admin, team, user)
	tag := word(t)
	own := clientAs(t, user)
	root := postAs(t, own, channel.Id, "", "Notes for "+tag)
	postAs(t, own, channel.Id, root.Id, "First reply, "+tag)
	postAs(t, own, channel.Id, root.Id, "Second reply, "+tag)

	run := startClient(t, testclients.Goose, personalAccessToken(t, admin, user.Id).Token, clientOptions{approve: true})
	run.drive("ready", nil)
	run.send(script("show", map[string]any{"kind": "thread", "post_id": root.Id, "limit": 2}), map[string]any{"approve": true})
	refused := run.drive("press", map[string]any{"text": root.Message, "button": "1 more reply", "until": "This app does not load more of the thread here."})
	mustContain(t, "the refused view", fmt.Sprint(refused["text"]), "Open in Mattermost")
}

// What is unread is shown in every client, a channel opens in place, and,
// where writes are allowed, Mark read in the view marks it read in
// Mattermost, through the tool only views call that changes something.
func TestClientsShowWhatIsUnreadAndMarkAChannelReadFromTheView(t *testing.T) {
	for _, client := range testclients.Clients {
		t.Run(client, func(t *testing.T) {
			admin := admin(t)
			user, other := seedUser(t, admin), seedUser(t, admin)
			team := seedTeam(t, admin, user, other)
			channel := seedChannel(t, admin, team, user, other)
			own := clientAs(t, user)
			_, _, err := own.ViewChannel(t.Context(), user.Id, &model.ChannelView{ChannelId: channel.Id})
			check(t, err)
			tag := word(t)
			author := clientAs(t, other)
			postAs(t, author, channel.Id, "", "The "+tag+" build is red")
			newest := postAs(t, author, channel.Id, "", "and the "+tag+" deploy waits")

			run := startClient(t, client, personalAccessToken(t, admin, user.Id).Token, clientOptions{writes: true})
			run.drive("ready", nil)
			run.send(script("show", map[string]any{"kind": "unreads"}), nil)
			shown := run.drive("view", map[string]any{"text": channel.DisplayName})
			// The view writes a team's name in capitals, as Mattermost's sidebar does.
			mustContain(t, "the unread view", strings.ToLower(fmt.Sprint(shown["text"])), strings.ToLower(team.DisplayName), "2 unread", "mark read")

			opened := run.drive("press", map[string]any{"text": channel.DisplayName, "button": regexp.QuoteMeta(channel.DisplayName), "until": newest.Message})
			mustContain(t, "the channel opened in place", fmt.Sprint(opened["text"]), "New messages", newest.Message)

			run.drive("press", map[string]any{"text": channel.DisplayName, "button": "^Mark read$", "until": "Marked read"})
			unread, _, err := own.GetChannelUnread(t.Context(), channel.Id, user.Id)
			check(t, err)
			if unread.MsgCount != 0 {
				t.Errorf("Mark read in the view left %d messages unread in Mattermost", unread.MsgCount)
			}
		})
	}
}
