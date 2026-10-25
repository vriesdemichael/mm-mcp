//go:build live

package live

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/cli"
	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/testsupport"
)

// mm-mcp serve checks the address and the credential before it serves.

type started struct {
	code   int
	stderr string
	served bool
	// tools are the tools the server it would serve lists.
	tools []string
}

func serveWith(t *testing.T, env map[string]string, args ...string) started {
	t.Helper()
	var stdout, stderr bytes.Buffer
	result := started{}
	result.code = cli.Run(t.Context(), append([]string{"serve"}, args...), cli.Deps{
		Getenv: testsupport.Env(env),
		Stdout: &stdout,
		Stderr: &stderr,
		Serve: func(ctx context.Context, served *mcp.Server, _ cli.ServeOptions) error {
			result.served = true
			serverSide, clientSide := mcp.NewInMemoryTransports()
			if _, err := served.Connect(ctx, serverSide, nil); err != nil {
				return err
			}
			session, err := mcp.NewClient(&mcp.Implementation{Name: "serve-test", Version: "1"}, nil).Connect(ctx, clientSide, nil)
			if err != nil {
				return err
			}
			defer func() { _ = session.Close() }()
			for tool, err := range session.Tools(ctx, nil) {
				if err != nil {
					return err
				}
				result.tools = append(result.tools, tool.Name)
			}
			return nil
		},
	})
	result.stderr = stderr.String()
	return result
}

func TestServeWithAWorkingTokenSaysWhoItIsConnectedAs(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)

	got := serveWith(t, map[string]string{config.EnvURL: liveURL, config.EnvToken: personalAccessToken(t, admin, user.Id).Token})

	if got.code != cli.ExitOK || !got.served || !strings.Contains(got.stderr, "connected to "+liveURL+" as @"+user.Username) {
		t.Fatalf("got %+v", got)
	}
}

func TestServeWithARefusedTokenStopsAtStartAndSaysWhatToDo(t *testing.T) {
	t.Parallel()

	got := serveWith(t, map[string]string{config.EnvURL: liveURL, config.EnvToken: "not-a-token-mattermost-issued"})

	if got.code != cli.ExitConfig || got.served || !strings.Contains(got.stderr, "refused the token in MM_TOKEN") || !strings.Contains(got.stderr, "mm-mcp login") {
		t.Fatalf("got %+v", got)
	}
}

func TestServeWithTheAPIAddressAsMMURLStillConnects(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)

	got := serveWith(t, map[string]string{config.EnvURL: liveURL + "/api/v4/", config.EnvToken: personalAccessToken(t, admin, user.Id).Token})

	if got.code != cli.ExitOK || !got.served || !strings.Contains(got.stderr, "as @"+user.Username) {
		t.Fatalf("got %+v", got)
	}
}

func TestServeAtAnAddressThatIsNotMattermostStopsAtStart(t *testing.T) {
	t.Parallel()

	got := serveWith(t, map[string]string{config.EnvURL: liveURL + "/not-mattermost", config.EnvToken: "any-token"})

	if got.code != cli.ExitConfig || got.served || !strings.Contains(got.stderr, "does not answer as a Mattermost server") {
		t.Fatalf("got %+v", got)
	}
}

// Over stdio the server offers this machine's files unless MM_MCP_LOCAL_FILES
// turns them off, as the container image does; over HTTP never (ADR-029).
func TestServeOffersThisMachinesFilesOnlyOverStdioAndUntilTurnedOff(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user := seedUser(t, admin)
	token := personalAccessToken(t, admin, user.Id).Token
	env := func(localFiles string) map[string]string {
		return map[string]string{config.EnvURL: liveURL, config.EnvToken: token, config.EnvLocalFiles: localFiles}
	}

	for _, run := range []struct {
		name       string
		localFiles string
		args       []string
		offered    bool
	}{
		{"over stdio", "", nil, true},
		{"over stdio, turned off", "false", nil, false},
		{"over HTTP", "", []string{"--transport", "http"}, false},
		{"over HTTP, asked for", "true", []string{"--transport", "http"}, false},
	} {
		got := serveWith(t, env(run.localFiles), run.args...)
		if got.code != cli.ExitOK || !got.served || !slices.Contains(got.tools, "read_post") || slices.Contains(got.tools, "save_file") != run.offered {
			t.Errorf("%s: got %+v; want save_file offered %v", run.name, got, run.offered)
		}
	}
}
