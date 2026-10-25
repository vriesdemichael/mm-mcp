// Package config reads what the server is configured with from its environment.
//
// The environment is the only source today. No flag carries a credential
// (ADR-019): a flag is visible in the process list and in the MCP client's
// argument array, where the env block is not.
package config

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// The variables mm-mcp reads.
const (
	EnvURL              = "MM_URL"
	EnvToken            = "MM_TOKEN"
	EnvAllowWrites      = "MM_MCP_ALLOW_WRITES"
	EnvElicitationOptIn = "MM_MCP_ELICITATION_OPT_IN"
	// EnvAskBeforeWrites sets nothing; it is read only to stop a server
	// started with it, naming EnvElicitationOptIn (ADR-033).
	EnvAskBeforeWrites                 = "MM_MCP_ASK_BEFORE_WRITES"
	EnvForceHumanInTheLoopInClaudeCode = "MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE"
	EnvDownloadDir                     = "MM_MCP_DOWNLOAD_DIR"
	EnvLocalFiles                      = "MM_MCP_LOCAL_FILES"
	EnvMarkAIGenerated                 = "MM_MCP_MARK_AI_GENERATED"
	EnvPostFromViews                   = "MM_MCP_POST_FROM_VIEWS"
	EnvBlockExternalNetwork            = "MM_MCP_BLOCK_EXTERNAL_NETWORK"
	EnvCAFile                          = "MM_MCP_CA_FILE"
	EnvResourceMaxMB                   = "MM_MCP_RESOURCE_MAX_MB"
)

// DefaultResourceMaxMB is the most one resource's message holds when nothing
// else is set, in megabytes of 1,048,576 bytes (ADR-029). MCP sends a resource
// whole, in one message, and a client refuses a message past its own limit by
// dropping the request or the whole session: the TypeScript SDK's stdio
// transport takes 10 MiB, Go's 16 MiB. Refused here first, the file can be
// read another way.
const DefaultResourceMaxMB = 10

// maxMB is the largest cap on a resource's message, in megabytes: past it,
// the bytes no longer fit an int64.
const maxMB = math.MaxInt64 >> 20

// EnvironmentVariables is every variable mm-mcp reads. The unit-test seal
// empties each of them, and a governance test fails when the source names one
// that is not listed here (ADR-006).
var EnvironmentVariables = []string{
	EnvURL, EnvToken, EnvAllowWrites, EnvElicitationOptIn, EnvAskBeforeWrites, EnvForceHumanInTheLoopInClaudeCode, EnvDownloadDir, EnvLocalFiles,
	EnvMarkAIGenerated, EnvPostFromViews, EnvBlockExternalNetwork, EnvCAFile, EnvResourceMaxMB,
}

// Config is a server's configuration.
type Config struct {
	URL   string
	Token string
	// TokenStored says the token is the one `mm-mcp login` stored, MM_TOKEN
	// being unset.
	TokenStored bool
	// Stored is the credential store the token was looked for in, or nil. The
	// diagnose tool reads it to find a login MM_TOKEN hides.
	Stored      Stored
	AllowWrites bool
	// Elicit says mm-mcp also asks the person before a write others see,
	// through MCP elicitation, as a convenience in a client that shows the
	// question: MM_MCP_ELICITATION_OPT_IN=true. Its zero value asks nothing,
	// and the MCP client's own approval of each call decides (ADR-033).
	Elicit bool
	// ForceHumanInTheLoopInClaudeCode marks each tool that would ask so that
	// Claude Code asks a person on every call, whatever its permission rules
	// allow: MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE=true, only without
	// Elicit (ADR-033).
	ForceHumanInTheLoopInClaudeCode bool
	// MarkAIGenerated marks every post and edit the model writes as written
	// with AI, as Mattermost shows it. On unless turned off.
	MarkAIGenerated bool
	// PostFromViews lets the person post a proposal from its view, where the
	// client renders it (ADR-035). On unless turned off: off, the draft waits in
	// Mattermost for them to send.
	PostFromViews bool
	// DownloadDir is where save_file writes; empty means the person's
	// Downloads directory.
	DownloadDir string
	// LocalFiles lets a server over stdio reach the files of the machine it
	// runs on. On unless turned off, as the container image turns it off: a
	// container's disk is not the person's (ADR-029). It never reaches them
	// over HTTP.
	LocalFiles bool
	// CAFile is a PEM file of certificate authorities to trust beside the
	// system's, for a server whose certificate an organisation signed itself.
	CAFile string
	// ResourceMaxMB is the most a resource's message holds, in megabytes;
	// zero is DefaultResourceMaxMB, and less than zero caps nothing.
	// ResourceBytes reads it.
	ResourceMaxMB int64
	// Local says the server runs on the person's own machine, for the one client
	// that started it over stdio, and may reach its files. It comes from how the
	// server is served, which the environment can only narrow, through
	// LocalFiles, and only a local server offers the tools that read or write
	// this machine's files.
	Local bool
}

// ResourceBytes is the most one resource's message holds as it is sent, or 0
// when nothing bounds it.
func (c Config) ResourceBytes() int64 {
	switch {
	case c.ResourceMaxMB == 0:
		return DefaultResourceMaxMB << 20
	case c.ResourceMaxMB < 0:
		return 0
	}
	return c.ResourceMaxMB << 20
}

// String leaves the token out, so a Config can be logged or printed in an error.
func (c Config) String() string {
	return fmt.Sprintf("Config{URL: %q, AllowWrites: %t, Elicit: %t, ForceHumanInTheLoopInClaudeCode: %t, MarkAIGenerated: %t, PostFromViews: %t, DownloadDir: %q, LocalFiles: %t, CAFile: %q, ResourceMaxMB: %d, Local: %t}",
		c.URL, c.AllowWrites, c.Elicit, c.ForceHumanInTheLoopInClaudeCode, c.MarkAIGenerated, c.PostFromViews, c.DownloadDir, c.LocalFiles, c.CAFile, c.ResourceMaxMB, c.Local)
}

// GoString is String, so %#v leaves the token out too.
func (c Config) GoString() string { return c.String() }

// Error says what in the environment keeps the server from starting.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func errorf(format string, args ...any) error { return &Error{msg: fmt.Sprintf(format, args...)} }

// FromEnv reads a Config through lookup, which returns "" for a variable that is not set.
func FromEnv(lookup func(string) string) (Config, error) {
	return Load(lookup, nil)
}

// Stored finds the token `mm-mcp login` stored for the server at an address,
// and says whether there is one.
type Stored func(address string) (token string, found bool, err error)

// Load is FromEnv, with the token stored for MM_URL used when MM_TOKEN is not
// set (ADR-019). stored may be nil, and then MM_TOKEN is required.
func Load(lookup func(string) string, stored Stored) (Config, error) {
	lookup = expanded(lookup)
	address, err := parseURL(lookup(EnvURL))
	if err != nil {
		return Config{}, err
	}
	token := strings.TrimSpace(lookup(EnvToken))
	fromStore := false
	if token == "" && stored != nil {
		found := false
		token, found, err = stored(address)
		switch {
		case err != nil:
			return Config{}, errorf("%s is not set, and the token `mm-mcp login` stores could not be read: %v.", EnvToken, err)
		case found:
			fromStore = true
		}
	}
	if token == "" {
		return Config{}, errorf("%s is not set, and no login is stored for %s. Run `mm-mcp login --url %s` once, or put a personal access token, a bot token or a session token in the MCP client's env block for this server.", EnvToken, address, address)
	}
	allowWrites, err := ParseBool(EnvAllowWrites, lookup(EnvAllowWrites))
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(lookup(EnvAskBeforeWrites)) != "" {
		// Starting anyway would leave whoever set it believing it decides who asks.
		return Config{}, errorf("%s sets nothing: mm-mcp asks before a write only with %s=true, a convenience in a client that "+
			"shows the question, not a safeguard, and otherwise your MCP client's own approval of each call decides, while "+
			"mm-mcp checks what it can itself. Remove %s.",
			EnvAskBeforeWrites, EnvElicitationOptIn, EnvAskBeforeWrites)
	}
	elicit, err := ParseBool(EnvElicitationOptIn, lookup(EnvElicitationOptIn))
	if err != nil {
		return Config{}, err
	}
	force, err := ParseBool(EnvForceHumanInTheLoopInClaudeCode, lookup(EnvForceHumanInTheLoopInClaudeCode))
	if err != nil {
		return Config{}, err
	}
	if force && elicit {
		// mm-mcp's question and the client's prompt would both come for one write.
		return Config{}, errorf("%s=true and %s=true would ask twice for one write: mm-mcp's question, and Claude Code's "+
			"prompt as well. Keep one.", EnvForceHumanInTheLoopInClaudeCode, EnvElicitationOptIn)
	}
	markAI := true
	if raw := lookup(EnvMarkAIGenerated); strings.TrimSpace(raw) != "" {
		if markAI, err = ParseBool(EnvMarkAIGenerated, raw); err != nil {
			return Config{}, err
		}
	}
	postFromViews := true
	if raw := lookup(EnvPostFromViews); strings.TrimSpace(raw) != "" {
		if postFromViews, err = ParseBool(EnvPostFromViews, raw); err != nil {
			return Config{}, err
		}
	}
	localFiles := true
	if raw := lookup(EnvLocalFiles); strings.TrimSpace(raw) != "" {
		if localFiles, err = ParseBool(EnvLocalFiles, raw); err != nil {
			return Config{}, err
		}
	}
	downloadDir := strings.TrimSpace(lookup(EnvDownloadDir))
	if downloadDir != "" && !filepath.IsAbs(downloadDir) {
		// A relative directory would be the MCP client's working directory,
		// which nobody chose and few could name.
		return Config{}, errorf("%s must be a full path, not %q.", EnvDownloadDir, downloadDir)
	}
	resourceMax, err := parseMB(EnvResourceMaxMB, lookup(EnvResourceMaxMB))
	if err != nil {
		return Config{}, err
	}
	switch raw := strings.TrimSpace(lookup(EnvResourceMaxMB)); {
	case raw == "":
		resourceMax = DefaultResourceMaxMB
	case resourceMax == 0:
		// Set to 0: no limit, which the zero value does not mean.
		resourceMax = -1
	}
	return Config{
		URL: address, Token: token, TokenStored: fromStore, Stored: stored, AllowWrites: allowWrites, Elicit: elicit, ForceHumanInTheLoopInClaudeCode: force, MarkAIGenerated: markAI,
		PostFromViews: postFromViews,
		DownloadDir:   downloadDir,
		LocalFiles:    localFiles,
		CAFile:        strings.TrimSpace(lookup(EnvCAFile)),
		ResourceMaxMB: resourceMax,
	}, nil
}

// parseMB reads a cap in megabytes: a whole number, or 0 or nothing for none.
// A bundle's number setting may arrive written as a decimal, 16.0, which is
// whole all the same.
func parseMB(name, raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value != math.Trunc(value) || value < 0 || value > maxMB {
		return 0, errorf("%s must be a whole number of megabytes, or 0 for no limit, not %q.", name, raw)
	}
	return int64(value), nil
}

// ParseBool reads the usual spellings of true and false; empty is false.
func ParseBool(name, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "", "0", "false", "no", "off":
		return false, nil
	default:
		return false, errorf("%s must be true or false, not %q.", name, raw)
	}
}

// ParseURL reads an MM_URL value: an http or https address, without a query
// or the API's own /api/v4.
func ParseURL(raw string) (string, error) { return parseURL(raw) }

func parseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errorf("%s is not set. Set it to the Mattermost server's address.", EnvURL)
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errorf("%s must be an http or https address, not %q.", EnvURL, raw)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(raw, "#") || strings.Contains(raw, "?") {
		return "", errorf("%s must not carry a query or a fragment: %q.", EnvURL, raw)
	}
	// The API's address is a common answer to "what is the server's address",
	// and Mattermost's client adds /api/v4 itself.
	address := strings.TrimRight(raw, "/")
	if strings.HasSuffix(strings.ToLower(address), apiPath) {
		address = strings.TrimRight(address[:len(address)-len(apiPath)], "/")
	}
	return address, nil
}

// apiPath is where Mattermost serves its REST API, below its address.
const apiPath = "/api/v4"

// expanded reads a variable an MCP client left as a placeholder, such as
// ${user_config.mm_token} for a setting the person left empty, as unset.
func expanded(lookup func(string) string) func(string) string {
	return func(name string) string {
		value := lookup(name)
		if Placeholder(value) {
			return ""
		}
		return value
	}
}

// Placeholder reports whether value is a placeholder an MCP client left
// unexpanded, which mm-mcp reads as unset.
func Placeholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}")
}
