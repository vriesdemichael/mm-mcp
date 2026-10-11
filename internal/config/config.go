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
	EnvURL                             = "MM_URL"
	EnvToken                           = "MM_TOKEN"
	EnvAllowWrites                     = "MM_MCP_ALLOW_WRITES"
	EnvAskBeforeWrites                 = "MM_MCP_ASK_BEFORE_WRITES"
	EnvForceHumanInTheLoopInClaudeCode = "MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE"
	EnvDownloadDir                     = "MM_MCP_DOWNLOAD_DIR"
	EnvMarkAIGenerated                 = "MM_MCP_MARK_AI_GENERATED"
	EnvBlockExternalNetwork            = "MM_MCP_BLOCK_EXTERNAL_NETWORK"
	EnvCAFile                          = "MM_MCP_CA_FILE"
	EnvResourceDownloadMaxMB           = "MM_MCP_RESOURCE_DOWNLOAD_MAX_MB"
	EnvResourceMaxMB                   = "MM_MCP_RESOURCE_MAX_MB"
)

// What a read of a file's resource is bounded by when nothing else is set
// (ADR-029), in megabytes of 1,048,576 bytes. A resource goes to the MCP
// client, not straight into the model's context, so it is bounded by what a
// client takes in one message rather than by what a model reads at once.
const (
	// DefaultResourceDownloadMaxMB is the largest file a resource read
	// downloads and converts, which is held in memory while it is.
	DefaultResourceDownloadMaxMB = 200
	// DefaultResourceMaxMB is the most one resource holds as it is sent:
	// Claude Code takes a message of 16 MB over SSE.
	DefaultResourceMaxMB = 16
	// maxMB is the most either may be set to: what one read holds in memory.
	maxMB = 2048
)

// EnvironmentVariables is every variable mm-mcp reads. The unit-test seal
// empties each of them, and a governance test fails when the source names one
// that is not listed here (ADR-006).
var EnvironmentVariables = []string{
	EnvURL, EnvToken, EnvAllowWrites, EnvAskBeforeWrites, EnvForceHumanInTheLoopInClaudeCode, EnvDownloadDir, EnvMarkAIGenerated,
	EnvBlockExternalNetwork, EnvCAFile, EnvResourceDownloadMaxMB, EnvResourceMaxMB,
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
	// SkipAsking says mm-mcp does not ask before a write others see, and
	// leaves it to the MCP client's own approval of the tool call:
	// MM_MCP_ASK_BEFORE_WRITES=false (ADR-033). Its zero value asks.
	SkipAsking bool
	// ForceHumanInTheLoopInClaudeCode marks each tool that would ask so that the MCP client
	// asks a person on every call, whatever its permission rules allow:
	// MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE=true, only with SkipAsking (ADR-033).
	ForceHumanInTheLoopInClaudeCode bool
	// MarkAIGenerated marks every post and edit the model writes as written
	// with AI, as Mattermost shows it. On unless turned off.
	MarkAIGenerated bool
	// DownloadDir is where save_file writes; empty means the person's
	// Downloads directory.
	DownloadDir string
	// CAFile is a PEM file of certificate authorities to trust beside the
	// system's, for a server whose certificate an organisation signed itself.
	CAFile string
	// ResourceDownloadMaxMB is the largest file a resource read downloads, and
	// ResourceMaxMB the most a resource holds as sent; zero is the default.
	// ResourceDownloadBytes and ResourceBytes read them.
	ResourceDownloadMaxMB int
	ResourceMaxMB         int
	// Local says the server runs on the person's own machine, for the one client
	// that started it over stdio. It comes from how the server is served, not
	// from the environment, and only a local server offers the tools that read
	// or write this machine's files.
	Local bool
}

// ResourceDownloadBytes is the largest file a resource read downloads.
func (c Config) ResourceDownloadBytes() int64 {
	return megabytes(c.ResourceDownloadMaxMB, DefaultResourceDownloadMaxMB)
}

// ResourceBytes is the most one resource holds as it is sent.
func (c Config) ResourceBytes() int64 {
	return megabytes(c.ResourceMaxMB, DefaultResourceMaxMB)
}

func megabytes(set, fallback int) int64 {
	if set <= 0 {
		set = fallback
	}
	return int64(set) << 20
}

// String leaves the token out, so a Config can be logged or printed in an error.
func (c Config) String() string {
	return fmt.Sprintf("Config{URL: %q, AllowWrites: %t, SkipAsking: %t, ForceHumanInTheLoopInClaudeCode: %t, MarkAIGenerated: %t, DownloadDir: %q, CAFile: %q, ResourceDownloadMaxMB: %d, ResourceMaxMB: %d, Local: %t}",
		c.URL, c.AllowWrites, c.SkipAsking, c.ForceHumanInTheLoopInClaudeCode, c.MarkAIGenerated, c.DownloadDir, c.CAFile, c.ResourceDownloadMaxMB, c.ResourceMaxMB, c.Local)
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
	ask := true
	if raw := lookup(EnvAskBeforeWrites); strings.TrimSpace(raw) != "" {
		if ask, err = ParseBool(EnvAskBeforeWrites, raw); err != nil {
			return Config{}, err
		}
	}
	force, err := ParseBool(EnvForceHumanInTheLoopInClaudeCode, lookup(EnvForceHumanInTheLoopInClaudeCode))
	if err != nil {
		return Config{}, err
	}
	if force && ask {
		// mm-mcp's question and the client's prompt would both come for one write.
		return Config{}, errorf("%s=true needs %s=false. While mm-mcp asks you before each write itself, a client that "+
			"prompts as well would ask twice; %s is for when your MCP client does the asking.",
			EnvForceHumanInTheLoopInClaudeCode, EnvAskBeforeWrites, EnvForceHumanInTheLoopInClaudeCode)
	}
	markAI := true
	if raw := lookup(EnvMarkAIGenerated); strings.TrimSpace(raw) != "" {
		if markAI, err = ParseBool(EnvMarkAIGenerated, raw); err != nil {
			return Config{}, err
		}
	}
	downloadDir := strings.TrimSpace(lookup(EnvDownloadDir))
	if downloadDir != "" && !filepath.IsAbs(downloadDir) {
		// A relative directory would be the MCP client's working directory,
		// which nobody chose and few could name.
		return Config{}, errorf("%s must be a full path, not %q.", EnvDownloadDir, downloadDir)
	}
	downloadMax, err := parseMB(EnvResourceDownloadMaxMB, lookup(EnvResourceDownloadMaxMB), DefaultResourceDownloadMaxMB)
	if err != nil {
		return Config{}, err
	}
	resourceMax, err := parseMB(EnvResourceMaxMB, lookup(EnvResourceMaxMB), DefaultResourceMaxMB)
	if err != nil {
		return Config{}, err
	}
	return Config{
		URL: address, Token: token, TokenStored: fromStore, Stored: stored, AllowWrites: allowWrites, SkipAsking: !ask, ForceHumanInTheLoopInClaudeCode: force, MarkAIGenerated: markAI,
		DownloadDir:           downloadDir,
		CAFile:                strings.TrimSpace(lookup(EnvCAFile)),
		ResourceDownloadMaxMB: downloadMax,
		ResourceMaxMB:         resourceMax,
	}, nil
}

// parseMB reads a size in megabytes: a whole number from 1 to maxMB, or the
// fallback when it is not set. A bundle's number setting may arrive written as
// a decimal, 16.0, which is whole all the same.
func parseMB(name, raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value != math.Trunc(value) || value < 1 || value > maxMB {
		return 0, errorf("%s must be a whole number of megabytes from 1 to %d, not %q.", name, maxMB, raw)
	}
	return int(value), nil
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
