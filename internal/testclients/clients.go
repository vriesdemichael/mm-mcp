// Package testclients installs and finds the MCP clients the client tests
// drive (ADR-036): Goose's desktop app and VS Code, at the versions
// .github/tool-versions.env pins, unpacked under .tmp/clients/ in the
// checkout. `task test:clients` installs them; the tests only find them, and
// fail naming the task when they are missing (ADR-009).
//
// Each download is checked against the SHA-256 its publisher gives for it:
// GitHub's digest of a Goose release asset, and VS Code's update service's.
package testclients

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/vriesdemichael/mm-mcp/internal/network"
)

// A client the tests drive.
const (
	Goose  = "goose"
	VSCode = "vscode"
)

// Clients are the clients the tests drive, in the order they run.
var Clients = []string{Goose, VSCode}

// pins are the variables in .github/tool-versions.env that pin each client.
var pins = map[string]string{Goose: "GOOSE_VERSION", VSCode: "VSCODE_VERSION"}

// Platform is an operating system and architecture, as Go names them.
type Platform struct{ OS, Arch string }

// This is the platform the code runs on.
func This() Platform { return Platform{runtime.GOOS, runtime.GOARCH} }

func (p Platform) String() string { return p.OS + "/" + p.Arch }

// Pinned reads the version tool-versions.env pins for each client.
func Pinned(root string) (map[string]string, error) {
	path := filepath.Join(root, ".github", "tool-versions.env")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	found := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok || strings.HasPrefix(name, "#") {
			continue
		}
		for client, pin := range pins {
			if name == pin {
				found[client] = strings.TrimPrefix(value, "v")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for _, client := range Clients {
		if found[client] == "" {
			return nil, fmt.Errorf("%s pins no %s", path, pins[client])
		}
	}
	return found, nil
}

// Dir is where a client's version for a platform is unpacked.
func Dir(root, client, version string, platform Platform) string {
	return filepath.Join(root, ".tmp", "clients", fmt.Sprintf("%s-%s-%s-%s", client, version, platform.OS, platform.Arch))
}

// executables are where each client's program is, relative to where it is
// unpacked, by operating system.
var executables = map[string]map[string]string{
	Goose: {
		"linux":   "usr/lib/goose/Goose",
		"windows": "Goose.exe",
		"darwin":  "Goose.app/Contents/MacOS/Goose",
	},
	VSCode: {
		"linux":   "VSCode-linux-x64/code",
		"windows": "Code.exe",
		"darwin":  "Visual Studio Code.app/Contents/MacOS/Electron",
	},
}

// Executable is the installed program of a client's version, or an error that
// says how to install it.
func Executable(root, client, version string, platform Platform) (string, error) {
	relative, ok := executables[client][platform.OS]
	if !ok {
		return "", fmt.Errorf("the client tests do not drive %s on %s", client, platform)
	}
	dir := Dir(root, client, version, platform)
	if _, err := os.Stat(filepath.Join(dir, installedMark)); err != nil {
		return "", fmt.Errorf("%s %s is not installed in %s; install it with `task test:clients`, or `go run ./tools/clients install`", client, version, dir)
	}
	return findBelow(dir, relative)
}

// findBelow is relative under dir, or the one file of its name further down,
// since an archive may wrap its contents in a folder of its own.
func findBelow(dir, relative string) (string, error) {
	direct := filepath.Join(dir, filepath.FromSlash(relative))
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}
	var found []string
	suffix := string(filepath.Separator) + filepath.FromSlash(relative)
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, suffix) {
			found = append(found, path)
		}
		return nil
	})
	if len(found) != 1 {
		return "", fmt.Errorf("%s holds %d copies of %s", dir, len(found), relative)
	}
	return found[0], nil
}

// installedMark is the file a finished installation leaves, holding the
// download's checksum, so an interrupted one is never taken for finished.
const installedMark = ".installed"

// Download is what installing a client fetches.
type Download struct {
	URL    string
	SHA256 string
	// Kind is how the download is unpacked: deb, zip or tar.gz.
	Kind string
}

// Installer fetches the clients into a checkout.
type Installer struct {
	// Root is the checkout.
	Root string
	// GitHubToken, when set, raises GitHub's limit on API reads, which CI's
	// shared addresses exhaust.
	GitHubToken string
	// githubAPI and vscodeUpdate are where the publishers answer, when not
	// their own addresses: a test's server.
	githubAPI, vscodeUpdate string
}

func (i Installer) github() string {
	if i.githubAPI != "" {
		return i.githubAPI
	}
	return "https://api.github.com"
}

func (i Installer) vscode() string {
	if i.vscodeUpdate != "" {
		return i.vscodeUpdate
	}
	return "https://update.code.visualstudio.com"
}

// Source is where a client's version for a platform is published.
func (i Installer) Source(ctx context.Context, client, version string, platform Platform) (Download, error) {
	switch client {
	case Goose:
		return i.gooseSource(ctx, version, platform)
	case VSCode:
		return i.vscodeSource(ctx, version, platform)
	}
	return Download{}, fmt.Errorf("no client %q", client)
}

// gooseAssets are the names of Goose's desktop release assets, by platform.
var gooseAssets = map[Platform]struct{ name, kind string }{
	{"linux", "amd64"}:   {"goose_%s_amd64.deb", "deb"},
	{"linux", "arm64"}:   {"goose_%s_arm64.deb", "deb"},
	{"windows", "amd64"}: {"Goose-win32-x64.zip", "zip"},
	{"darwin", "arm64"}:  {"Goose-darwin-arm64.zip", "zip"},
	{"darwin", "amd64"}:  {"Goose-darwin-x64.zip", "zip"},
}

func (i Installer) gooseSource(ctx context.Context, version string, platform Platform) (Download, error) {
	asset, ok := gooseAssets[platform]
	if !ok {
		return Download{}, fmt.Errorf("no desktop app of Goose is published for %s", platform)
	}
	name := asset.name
	if strings.Contains(name, "%s") {
		name = fmt.Sprintf(name, version)
	}
	var release struct {
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := i.getJSON(ctx, i.github()+"/repos/block/goose/releases/tags/v"+version, &release); err != nil {
		return Download{}, fmt.Errorf("reading Goose %s's release: %w", version, err)
	}
	for _, a := range release.Assets {
		if a.Name == name {
			sum, ok := strings.CutPrefix(a.Digest, "sha256:")
			if !ok {
				return Download{}, fmt.Errorf("GitHub gives no SHA-256 for %s", name)
			}
			return Download{URL: a.URL, SHA256: sum, Kind: asset.kind}, nil
		}
	}
	return Download{}, fmt.Errorf("the release of Goose %s has no asset %s", version, name)
}

// vscodePlatforms are VS Code's names for its archives, by platform.
var vscodePlatforms = map[Platform]struct{ name, kind string }{
	{"linux", "amd64"}:   {"linux-x64", "tar.gz"},
	{"linux", "arm64"}:   {"linux-arm64", "tar.gz"},
	{"windows", "amd64"}: {"win32-x64-archive", "zip"},
	{"windows", "arm64"}: {"win32-arm64-archive", "zip"},
	{"darwin", "arm64"}:  {"darwin-arm64", "zip"},
	{"darwin", "amd64"}:  {"darwin", "zip"},
}

func (i Installer) vscodeSource(ctx context.Context, version string, platform Platform) (Download, error) {
	target, ok := vscodePlatforms[platform]
	if !ok {
		return Download{}, fmt.Errorf("VS Code publishes no archive for %s", platform)
	}
	var build struct {
		URL    string `json:"url"`
		SHA256 string `json:"sha256hash"`
	}
	if err := i.getJSON(ctx, fmt.Sprintf("%s/api/versions/%s/%s/stable", i.vscode(), version, target.name), &build); err != nil {
		return Download{}, fmt.Errorf("reading VS Code %s for %s: %w", version, target.name, err)
	}
	if build.URL == "" || build.SHA256 == "" {
		return Download{}, fmt.Errorf("VS Code's update service gives no archive and checksum for %s %s", version, target.name)
	}
	return Download{URL: build.URL, SHA256: build.SHA256, Kind: target.kind}, nil
}

// Newest is the newest release of a client, for the scheduled run against
// what people install today.
func (i Installer) Newest(ctx context.Context, client string) (string, error) {
	switch client {
	case Goose:
		var release struct {
			Tag string `json:"tag_name"`
		}
		if err := i.getJSON(ctx, i.github()+"/repos/block/goose/releases/latest", &release); err != nil {
			return "", err
		}
		return strings.TrimPrefix(release.Tag, "v"), nil
	case VSCode:
		var versions []string
		if err := i.getJSON(ctx, i.vscode()+"/api/releases/stable", &versions); err != nil {
			return "", err
		}
		if len(versions) == 0 {
			return "", errors.New("VS Code's update service lists no release")
		}
		return versions[0], nil
	}
	return "", fmt.Errorf("no client %q", client)
}

var httpClient = &http.Client{Transport: network.NewSafeTransport(), Timeout: 30 * time.Minute}

func (i Installer) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if i.GitHubToken != "" && strings.HasPrefix(url, i.github()+"/") {
		request.Header.Set("Authorization", "Bearer "+i.GitHubToken)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}
	return response, nil
}

func (i Installer) getJSON(ctx context.Context, url string, into any) error {
	response, err := i.get(ctx, url)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	return json.NewDecoder(response.Body).Decode(into)
}

// Install downloads a client's version for this platform, checks it, and
// unpacks it, unless it is installed already. It answers with the program.
func (i Installer) Install(ctx context.Context, client, version string) (string, error) {
	platform := This()
	dir := Dir(i.Root, client, version, platform)
	if program, err := Executable(i.Root, client, version, platform); err == nil {
		return program, nil
	}
	source, err := i.Source(ctx, client, version, platform)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	archive := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"."+source.Kind)
	if err := i.download(ctx, source, archive); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archive) }()
	if err := unpack(source.Kind, archive, dir); err != nil {
		return "", fmt.Errorf("unpacking %s: %w", archive, err)
	}
	if err := os.WriteFile(filepath.Join(dir, installedMark), []byte(source.SHA256+"\n"), 0o644); err != nil {
		return "", err
	}
	return Executable(i.Root, client, version, platform)
}

// download fetches a download to path, and refuses it unless its SHA-256 is
// the one its publisher gives.
func (i Installer) download(ctx context.Context, source Download, path string) error {
	response, err := i.get(ctx, source.URL)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, sum), response.Body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); !strings.EqualFold(got, source.SHA256) {
		return fmt.Errorf("%s has SHA-256 %s; its publisher gives %s", source.URL, got, source.SHA256)
	}
	return nil
}

func unpack(kind, archive, dir string) error {
	switch kind {
	case "zip":
		return unzip(archive, dir)
	case "tar.gz":
		return untar(archive, dir)
	case "deb":
		// A .deb's data is xz or zstd inside an ar archive; dpkg-deb, which
		// every Debian and Ubuntu has, reads either.
		output, err := exec.Command("dpkg-deb", "-x", archive, dir).CombinedOutput()
		if err != nil {
			return fmt.Errorf("dpkg-deb -x: %w: %s", err, output)
		}
		return nil
	}
	return fmt.Errorf("no way to unpack %q", kind)
}

// inside is name below dir, refusing a name that climbs out of it.
func inside(dir, name string) (string, error) {
	path := filepath.Join(dir, filepath.FromSlash(name))
	if path != dir && !strings.HasPrefix(path, dir+string(filepath.Separator)) {
		return "", fmt.Errorf("%s would unpack outside %s", name, dir)
	}
	return path, nil
}

func unzip(archive, dir string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, entry := range reader.File {
		path, err := inside(dir, entry.Name)
		if err != nil {
			return err
		}
		mode := entry.Mode()
		switch {
		case mode.IsDir():
			err = os.MkdirAll(path, 0o755)
		case mode&fs.ModeSymlink != 0:
			err = symlinkFrom(entry, path)
		default:
			err = writeFrom(entry.Open, path, mode.Perm())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func symlinkFrom(entry *zip.File, path string) error {
	opened, err := entry.Open()
	if err != nil {
		return err
	}
	target, err := io.ReadAll(opened)
	_ = opened.Close()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.Symlink(string(target), path)
}

func untar(archive, dir string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		path, err := inside(dir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(path, 0o755)
		case tar.TypeSymlink:
			if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
				err = os.Symlink(header.Linkname, path)
			}
		case tar.TypeReg:
			err = writeFrom(func() (io.ReadCloser, error) { return io.NopCloser(reader), nil }, path, header.FileInfo().Mode().Perm())
		}
		if err != nil {
			return err
		}
	}
}

func writeFrom(open func() (io.ReadCloser, error), path string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	source, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, source); err != nil { //nolint:gosec // a client archive from its publisher, checked against its checksum
		_ = file.Close()
		return err
	}
	return file.Close()
}

// Known reports whether name is a client the tests drive.
func Known(name string) bool { return slices.Contains(Clients, name) }

// Pin is the variable in tool-versions.env that pins a client.
func Pin(client string) string { return pins[client] }
