package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/server"
)

func template(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAWindowsBundleNamesItsExeAndOnlyItsPlatform(t *testing.T) {
	t.Parallel()
	raw, err := Manifest(template(t, "mcpb/manifest.json"), "v0.3.1", "windows")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
		Server  struct {
			EntryPoint string `json:"entry_point"`
			MCPConfig  struct {
				Command string            `json:"command"`
				Env     map[string]string `json:"env"`
			} `json:"mcp_config"`
		} `json:"server"`
		Compatibility struct {
			Platforms []string `json:"platforms"`
		} `json:"compatibility"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "0.3.1" || manifest.Server.EntryPoint != "server/mm-mcp.exe" ||
		manifest.Server.MCPConfig.Command != "${__dirname}/server/mm-mcp.exe" ||
		len(manifest.Compatibility.Platforms) != 1 || manifest.Compatibility.Platforms[0] != "win32" {
		t.Fatalf("got %+v", manifest)
	}
	if manifest.Server.MCPConfig.Env["MM_TOKEN"] != "${user_config.mattermost_token}" {
		t.Fatalf("the token no longer reaches the server: %v", manifest.Server.MCPConfig.Env)
	}
}

func TestTheBundleListsEveryToolTheServerHas(t *testing.T) {
	t.Parallel()
	raw, err := Manifest(template(t, "mcpb/manifest.json"), "v0.3.1", "linux")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
		ToolsGenerated bool `json:"tools_generated"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Tools) != len(server.AllSpecs()) || !manifest.ToolsGenerated {
		t.Fatalf("the bundle lists %d tools of %d", len(manifest.Tools), len(server.AllSpecs()))
	}
	for _, tool := range manifest.Tools {
		if tool.Name == "" || !strings.HasSuffix(tool.Description, ".") || strings.Contains(tool.Description, ". ") {
			t.Errorf("a tool reads %+v", tool)
		}
	}
}

func TestAnUnknownPlatformOrVersionIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := Manifest(template(t, "mcpb/manifest.json"), "v0.3.1", "plan9"); err == nil {
		t.Error("plan9 was accepted")
	}
	if _, err := Manifest(template(t, "mcpb/manifest.json"), "0.3.1", "linux"); err == nil {
		t.Error("a version without its v was accepted")
	}
}

const image = "ghcr.io/vriesdemichael/mm-mcp"

func TestTheRegistryEntryListsEveryBundleByAddressAndChecksumAndTheImageByTag(t *testing.T) {
	t.Parallel()
	raw, err := ServerJSON(template(t, "server.json"), template(t, "mcpb/manifest.json"), "v0.3.1", "https://example.com/download/v0.3.1/", image,
		map[string]string{"mm-mcp_0.3.1_linux_amd64.mcpb": "aa", "mm-mcp_0.3.1_darwin_arm64.mcpb": "bb"})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version  string           `json:"version"`
		Packages []map[string]any `json:"packages"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != "0.3.1" || len(document.Packages) != 3 ||
		document.Packages[0]["identifier"] != "https://example.com/download/v0.3.1/mm-mcp_0.3.1_darwin_arm64.mcpb" ||
		document.Packages[0]["fileSha256"] != "bb" || document.Packages[0]["version"] != "0.3.1" || document.Packages[1]["registryType"] != "mcpb" {
		t.Fatalf("got %+v", document)
	}
	// The registry refuses an image package with a version, a checksum or a
	// registry address of its own: the tag is its version.
	oci := document.Packages[2]
	if oci["registryType"] != "oci" || oci["identifier"] != image+":0.3.1" || oci["transport"].(map[string]any)["type"] != "stdio" {
		t.Fatalf("got %+v", oci)
	}
	for _, refused := range []string{"version", "fileSha256", "registryBaseUrl"} {
		if _, ok := oci[refused]; ok {
			t.Errorf("the image's package has %s: %+v", refused, oci)
		}
	}
	if _, err := ServerJSON(template(t, "server.json"), template(t, "mcpb/manifest.json"), "v0.3.1", "https://example.com", image, nil); err == nil {
		t.Fatal("an entry with no bundles was written")
	}
	for _, wrong := range []string{"", image + ":0.3.1", image + "@sha256:aa"} {
		if _, err := ServerJSON(template(t, "server.json"), template(t, "mcpb/manifest.json"), "v0.3.1", "https://example.com", wrong,
			map[string]string{"mm-mcp_0.3.1_linux_amd64.mcpb": "aa"}); err == nil {
			t.Errorf("an entry with the image %q was written", wrong)
		}
	}
}

func TestEveryRegistryPackageDeclaresTheVariablesItsServerCanUse(t *testing.T) {
	t.Parallel()
	raw, err := ServerJSON(template(t, "server.json"), template(t, "mcpb/manifest.json"), "v0.3.1", "https://example.com", image,
		map[string]string{"mm-mcp_0.3.1_linux_amd64.mcpb": "aa", "mm-mcp_0.3.1_windows_arm64.mcpb": "bb"})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Packages []struct {
			RegistryType         string           `json:"registryType"`
			EnvironmentVariables []map[string]any `json:"environmentVariables"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Packages) != 3 {
		t.Fatalf("got %d packages", len(document.Packages))
	}
	want := map[string]map[string]any{
		"MM_URL":                    {"isRequired": true, "isSecret": false, "format": "string"},
		"MM_TOKEN":                  {"isRequired": false, "isSecret": true, "format": "string"},
		"MM_MCP_ALLOW_WRITES":       {"isRequired": false, "isSecret": false, "format": "boolean", "default": "false"},
		"MM_MCP_ELICITATION_OPT_IN": {"isRequired": false, "isSecret": false, "format": "boolean", "default": "false"},
		"MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE": {"isRequired": false, "isSecret": false, "format": "boolean", "default": "false"},
		"MM_MCP_MARK_AI_GENERATED":                      {"isRequired": false, "isSecret": false, "format": "boolean", "default": "true"},
		"MM_MCP_POST_FROM_VIEWS":                        {"isRequired": false, "isSecret": false, "format": "boolean", "default": "true"},
		"MM_MCP_LOCAL_FILES":                            {"isRequired": false, "isSecret": false, "format": "boolean", "default": "true"},
		"MM_MCP_DOWNLOAD_DIR":                           {"isRequired": false, "isSecret": false, "format": "filepath"},
		"MM_MCP_CA_FILE":                                {"isRequired": false, "isSecret": false, "format": "filepath"},
		"MM_MCP_RESOURCE_MAX_MB":                        {"isRequired": false, "isSecret": false, "format": "number", "default": "10"},
	}
	// The image's disk is not the person's: no path of theirs, and no reaching
	// it (ADR-029).
	notInTheImage := []string{"MM_MCP_LOCAL_FILES", "MM_MCP_DOWNLOAD_DIR", "MM_MCP_CA_FILE"}
	for _, pkg := range document.Packages {
		count := len(want)
		if pkg.RegistryType == "oci" {
			count -= len(notInTheImage)
		}
		if len(pkg.EnvironmentVariables) != count {
			t.Fatalf("%s: got %v, want %d variables", pkg.RegistryType, pkg.EnvironmentVariables, count)
		}
		for _, variable := range pkg.EnvironmentVariables {
			name, _ := variable["name"].(string)
			expected, ok := want[name]
			if !ok {
				t.Errorf("%s is declared, and the bundle does not set it", name)
				continue
			}
			if pkg.RegistryType == "oci" && slices.Contains(notInTheImage, name) {
				t.Errorf("the image's package declares %s, which names or reaches the person's disk", name)
			}
			if description, _ := variable["description"].(string); description == "" {
				t.Errorf("%s has no description", name)
			}
			for key, value := range expected {
				if variable[key] != value {
					t.Errorf("%s: %s is %v, want %v", name, key, variable[key], value)
				}
			}
			if _, ok := expected["default"]; !ok && variable["default"] != nil {
				t.Errorf("%s has the default %v, and the bundle gives none", name, variable["default"])
			}
		}
	}
}

func TestAVariableTheBundleDoesNotAskForIsRefused(t *testing.T) {
	t.Parallel()
	for _, manifest := range []string{
		`{"server":{"mcp_config":{"env":{"MM_URL":"https://chat.example.com"}}},"user_config":{}}`,
		`{"server":{"mcp_config":{"env":{"MM_URL":"${user_config.url}"}}},"user_config":{}}`,
		`{"server":{"mcp_config":{"env":{"MM_URL":"${user_config.url}"}}},"user_config":{"url":{"type":"colour"}}}`,
		`{"server":{"mcp_config":{}},"user_config":{}}`,
	} {
		if variables, err := EnvironmentVariables([]byte(manifest)); err == nil {
			t.Errorf("%s was read as %+v", manifest, variables)
		}
	}
}

func TestEveryBinaryGoReleaserBuiltBecomesABundleForItsPlatform(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	linux := filepath.Join(dir, "mm-mcp")
	windows := filepath.Join(dir, "mm-mcp.exe")
	for _, path := range []string{linux, windows} {
		if err := os.WriteFile(path, []byte("binary"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	artifacts := `[
	  {"name":"mm-mcp.exe","path":"` + filepath.ToSlash(windows) + `","goos":"windows","goarch":"arm64","type":"Binary"},
	  {"name":"mm-mcp_0.3.1_linux_amd64.tar.gz","path":"x","goos":"linux","goarch":"amd64","type":"Archive"},
	  {"name":"mm-mcp","path":"` + filepath.ToSlash(linux) + `","goos":"linux","goarch":"amd64","type":"Binary"}
	]`
	binaries, err := Binaries([]byte(artifacts))
	if err != nil || len(binaries) != 2 || binaries[0].GOOS != "linux" {
		t.Fatalf("got %+v, %v", binaries, err)
	}
	for _, binary := range binaries {
		path, err := Bundle(template(t, "mcpb/manifest.json"), "v0.3.1", binary, dir)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, file := range reader.File {
			names = append(names, file.Name)
			if strings.HasPrefix(file.Name, "server/") && file.Mode()&0o100 == 0 {
				t.Errorf("%s: %s is not executable", path, file.Name)
			}
		}
		_ = reader.Close()
		want := []string{"manifest.json", "server/" + filepath.Base(binary.Path)}
		if !slices.Equal(names, want) {
			t.Errorf("%s holds %v, want %v", filepath.Base(path), names, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mm-mcp_0.3.1_windows_arm64.mcpb")); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactsWithoutBinariesAreRefused(t *testing.T) {
	t.Parallel()
	if _, err := Binaries([]byte(`[{"type":"Archive"}]`)); err == nil {
		t.Fatal("artifacts with no binaries were accepted")
	}
}

// The registry lists the image only when its label names this server, and the
// image's package leaves out MM_MCP_LOCAL_FILES because the image sets it off.
func TestTheImageNamesTheServerAndKeepsToItsOwnDisk(t *testing.T) {
	t.Parallel()
	var entry struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(template(t, "server.json"), &entry); err != nil || entry.Name == "" {
		t.Fatalf("server.json: %v, %+v", err, entry)
	}
	dockerfile := string(template(t, "image/Dockerfile"))
	if !strings.Contains(dockerfile, `io.modelcontextprotocol.server.name="`+entry.Name+`"`) {
		t.Errorf("image/Dockerfile does not label the image as %s", entry.Name)
	}
	if !slices.Contains(strings.Split(dockerfile, "\n"), "ENV "+config.EnvLocalFiles+"=false") {
		t.Errorf("image/Dockerfile does not set %s=false", config.EnvLocalFiles)
	}
}
