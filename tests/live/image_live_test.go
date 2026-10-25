//go:build live && image

package live

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// The container image (ADR-023), built from image/Dockerfile around an mm-mcp
// built here for the machine Docker runs on, served over stdio as `docker run
// -i` serves it, against the live instance. It keeps to its own disk unless
// told otherwise, with a directory mounted (ADR-029). `task test:image` runs
// these; they carry the image tag as well as live, so `task test:live` never
// runs them.

// imageVersion is the version the image's mm-mcp is built to report, so a test
// knows the image runs the binary built here.
const imageVersion = "v0.0.0-image-test"

// builtImage is the image, built once for every test.
var builtImage = sync.OnceValues(func() (string, error) {
	arch, err := exec.Command("docker", "version", "--format", "{{.Server.Arch}}").Output()
	if err != nil {
		return "", fmt.Errorf("asking Docker which architecture it runs: %w", err)
	}
	context, err := os.MkdirTemp("", "mm-mcp-image-")
	if err != nil {
		return "", err
	}
	// Laid out as GoReleaser lays out dist/, which image/Dockerfile copies from.
	binary := filepath.Join(context, "mm-mcp_linux_"+strings.TrimSpace(string(arch))+"_test", "mm-mcp")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-X github.com/vriesdemichael/mm-mcp/internal/version.Version="+imageVersion, "-o", binary, "../../cmd/mm-mcp")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+strings.TrimSpace(string(arch)), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building mm-mcp: %w: %s", err, output)
	}
	// One tag per checkout, built again on each run, so runs leave no pile of
	// images behind and two checkouts never run each other's.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", err
	}
	tag := fmt.Sprintf("mm-mcp:live-image-%x", sha256.Sum256([]byte(root)))[:len("mm-mcp:live-image-")+12]
	if output, err := exec.Command("docker", "build", "--file", "../../image/Dockerfile", "--tag", tag, context).CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the image: %w: %s", err, output)
	}
	return tag, nil
})

// inMattermostsNetwork is how a container reaches the live instance: inside the
// network of the container that serves it, where Mattermost listens on its own
// port, or, for an instance no local container publishes, on this machine's.
func inMattermostsNetwork(t *testing.T) (network, address string) {
	t.Helper()
	live, err := url.Parse(liveURL)
	check(t, err)
	if live.Port() != "" {
		out, err := exec.Command("docker", "ps", "--quiet", "--filter", "publish="+live.Port()).Output()
		check(t, err)
		if ids := strings.Fields(string(out)); len(ids) == 1 {
			return "container:" + ids[0], "http://127.0.0.1:8065"
		}
	}
	return "host", liveURL
}

// imageSession is an MCP session with mm-mcp in the image, started as an MCP
// client starts a server's command, acting as the token's owner. env is passed
// on by name, so neither the token nor anything else lands in the command's
// arguments (ADR-019); args go to docker run before the image.
func imageSession(t *testing.T, token string, env map[string]string, args ...string) *mcp.ClientSession {
	t.Helper()
	image, err := builtImage()
	check(t, err)
	network, address := inMattermostsNetwork(t)
	passed := map[string]string{config.EnvURL: address, config.EnvToken: token}
	maps.Copy(passed, env)
	run := []string{"run", "--interactive", "--rm", "--network", network}
	command := exec.Command("docker")
	command.Env = os.Environ()
	for _, name := range slices.Sorted(maps.Keys(passed)) {
		run = append(run, "--env", name)
		command.Env = append(command.Env, name+"="+passed[name])
	}
	command.Args = append(append(append(command.Args, run...), args...), image)
	client := mcp.NewClient(&mcp.Implementation{Name: "image-test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: command}, nil)
	check(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listedTools(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	var names []string
	for tool, err := range session.Tools(t.Context(), nil) {
		check(t, err)
		names = append(names, tool.Name)
	}
	return names
}

func TestTheImageServesOverStdioAndReachesNoFileOfItsOwn(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)

	session := imageSession(t, personalAccessToken(t, admin, user.Id).Token, nil)

	if got := session.InitializeResult().ServerInfo.Version; got != imageVersion {
		t.Errorf("the image's server reports %q; want %q, the binary built for it", got, imageVersion)
	}
	listed := listedTools(t, session)
	if !slices.Contains(listed, "read_post") || slices.Contains(listed, "save_file") {
		t.Fatalf("the image lists %v; want the tools that read, and no save_file", listed)
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "get_me"})
	check(t, err)
	var me struct {
		Username string `json:"username"`
	}
	structured(t, result, &me)
	if result.IsError || me.Username != user.Username {
		t.Fatalf("get_me from the image answered %+v; want @%s", result, user.Username)
	}
}

// Set back to true, with a directory mounted and named, the image saves into
// it: the one way its disk is the person's.
func TestTheImageSavesIntoAMountedDirectoryWhenToldTo(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	_, id := attachAs(t, clientAs(t, other), channel.Id, "minutes.txt", []byte("decided: ship it\n"))
	mounted := t.TempDir()
	// As installation.md says: the container runs as the person, who owns the
	// directory and reads what is saved there, rather than as distroless's
	// nonroot user. Windows has no user ids, and Docker shares a directory
	// with the container whole.
	args := []string{"--volume", mounted + ":/downloads"}
	if uid := os.Getuid(); uid >= 0 {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, os.Getgid()))
	}
	session := imageSession(t, personalAccessToken(t, admin, user.Id).Token,
		map[string]string{config.EnvLocalFiles: "true", config.EnvDownloadDir: "/downloads"}, args...)

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "save_file", Arguments: map[string]any{"file_id": id}})
	check(t, err)

	var saved server.SavedFile
	structured(t, result, &saved)
	if result.IsError || saved.Path != "/downloads/minutes.txt" {
		t.Fatalf("save_file in the image answered %+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(mounted, "minutes.txt")); err != nil || string(got) != "decided: ship it\n" {
		t.Fatalf("the mounted directory holds %q, %v", got, err)
	}
}
