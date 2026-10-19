package testclients

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A publisher, served locally: GitHub's releases of Goose and VS Code's
// update service, with one VS Code archive for this platform.
type publisher struct {
	*httptest.Server
	archive []byte
	// token is the Authorization header GitHub was last sent.
	token string
}

func newPublisher(t *testing.T) *publisher {
	t.Helper()
	p := &publisher{archive: vscodeArchive(t)}
	sum := sha256.Sum256(p.archive)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/block/goose/releases/tags/v1.55.0", func(w http.ResponseWriter, r *http.Request) {
		p.token = r.Header.Get("Authorization")
		writeJSONTo(w, map[string]any{"assets": []map[string]string{
			{"name": "goose_1.55.0_amd64.deb", "browser_download_url": p.URL + "/goose.deb", "digest": "sha256:" + strings.Repeat("a", 64)},
			{"name": "Goose-win32-x64.zip", "browser_download_url": p.URL + "/goose.zip", "digest": ""},
		}})
	})
	mux.HandleFunc("GET /repos/block/goose/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONTo(w, map[string]string{"tag_name": "v1.56.0"})
	})
	mux.HandleFunc("GET /api/releases/stable", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONTo(w, []string{"1.143.0", "1.142.0"})
	})
	mux.HandleFunc("GET /api/versions/1.142.0/{target}/stable", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONTo(w, map[string]string{"url": p.URL + "/vscode-archive", "sha256hash": hex.EncodeToString(sum[:])})
	})
	mux.HandleFunc("GET /vscode-archive", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(p.archive) })
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Close)
	return p
}

func (p *publisher) installer(root string) Installer {
	return Installer{Root: root, GitHubToken: "a-token", githubAPI: p.URL, vscodeUpdate: p.URL}
}

func writeJSONTo(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// vscodeArchive is a VS Code archive for this platform, holding its program
// where VS Code's does.
func vscodeArchive(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	program := executables[VSCode][runtime.GOOS]
	if vscodePlatforms[This()].kind == "tar.gz" {
		gz := gzip.NewWriter(&out)
		tw := tar.NewWriter(gz)
		_ = tw.WriteHeader(&tar.Header{Name: program, Typeflag: tar.TypeReg, Mode: 0o755, Size: 7})
		_, _ = tw.Write([]byte("program"))
		_ = tw.Close()
		_ = gz.Close()
		return out.Bytes()
	}
	archive := zip.NewWriter(&out)
	entry, _ := archive.Create(program)
	_, _ = entry.Write([]byte("program"))
	_ = archive.Close()
	return out.Bytes()
}

func TestGoosesReleaseNamesItsAssetAndItsChecksum(t *testing.T) {
	t.Parallel()
	p := newPublisher(t)
	installer := p.installer(t.TempDir())
	linux, err := installer.Source(t.Context(), Goose, "1.55.0", Platform{"linux", "amd64"})
	if err != nil || linux.Kind != "deb" || linux.URL != p.URL+"/goose.deb" || linux.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("the Linux asset: %+v, %v", linux, err)
	}
	if p.token != "Bearer a-token" {
		t.Errorf("GitHub was sent %q", p.token)
	}
	if _, err := installer.Source(t.Context(), Goose, "1.55.0", Platform{"windows", "amd64"}); err == nil || !strings.Contains(err.Error(), "no SHA-256") {
		t.Errorf("an asset without a checksum: %v", err)
	}
	if _, err := installer.Source(t.Context(), Goose, "1.55.0", Platform{"darwin", "arm64"}); err == nil || !strings.Contains(err.Error(), "has no asset") {
		t.Errorf("an asset the release lacks: %v", err)
	}
	if _, err := installer.Source(t.Context(), Goose, "1.55.0", Platform{"plan9", "amd64"}); err == nil {
		t.Error("a platform Goose has no app for")
	}
	if _, err := installer.Source(t.Context(), Goose, "9.9.9", Platform{"linux", "amd64"}); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a release that does not exist: %v", err)
	}
	if _, err := installer.Source(t.Context(), "other", "1", Platform{"linux", "amd64"}); err == nil {
		t.Error("a client the tests do not drive")
	}
}

func TestVSCodesUpdateServiceNamesItsArchiveAndItsChecksum(t *testing.T) {
	t.Parallel()
	p := newPublisher(t)
	installer := p.installer(t.TempDir())
	source, err := installer.Source(t.Context(), VSCode, "1.142.0", Platform{"windows", "amd64"})
	if err != nil || source.Kind != "zip" || source.URL != p.URL+"/vscode-archive" || len(source.SHA256) != 64 {
		t.Fatalf("the Windows archive: %+v, %v", source, err)
	}
	if _, err := installer.Source(t.Context(), VSCode, "1.0.0", Platform{"linux", "amd64"}); err == nil {
		t.Error("a version the service does not have")
	}
	if _, err := installer.Source(t.Context(), VSCode, "1.142.0", Platform{"plan9", "amd64"}); err == nil {
		t.Error("a platform VS Code has no archive for")
	}
}

func TestTheNewestReleaseIsEachPublishersNewest(t *testing.T) {
	t.Parallel()
	p := newPublisher(t)
	installer := p.installer(t.TempDir())
	for client, want := range map[string]string{Goose: "1.56.0", VSCode: "1.143.0"} {
		if got, err := installer.Newest(t.Context(), client); err != nil || got != want {
			t.Errorf("%s's newest: %q, %v; want %s", client, got, err, want)
		}
	}
	if _, err := installer.Newest(t.Context(), "other"); err == nil {
		t.Error("a client the tests do not drive")
	}
}

// Installing downloads the archive, checks it, unpacks it and finds the
// program; once installed, it is found without asking the publisher again.
func TestAClientIsInstalledOnceAndFoundThereafter(t *testing.T) {
	t.Parallel()
	p := newPublisher(t)
	root := t.TempDir()
	installer := p.installer(root)
	program, err := installer.Install(t.Context(), VSCode, "1.142.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(Dir(root, VSCode, "1.142.0", This()), filepath.FromSlash(executables[VSCode][runtime.GOOS])); program != want {
		t.Errorf("installed %s; want %s", program, want)
	}
	p.Close()
	if again, err := installer.Install(t.Context(), VSCode, "1.142.0"); err != nil || again != program {
		t.Errorf("installing again asked the publisher: %q, %v", again, err)
	}
}

// An install whose download is not what its publisher gives leaves nothing
// that is taken for installed.
func TestAFailedInstallLeavesNothingInstalled(t *testing.T) {
	t.Parallel()
	p := newPublisher(t)
	root := t.TempDir()
	p.archive = []byte("not the archive the checksum is of")
	installer := p.installer(root)
	if _, err := installer.Install(t.Context(), VSCode, "1.142.0"); err == nil || !strings.Contains(err.Error(), "publisher gives") {
		t.Fatalf("a download that fails its checksum: %v", err)
	}
	if _, err := Executable(root, VSCode, "1.142.0", This()); err == nil {
		t.Error("a failed install was taken for installed")
	}
	if _, err := installer.Install(t.Context(), VSCode, "1.0.0"); err == nil {
		t.Error("a version the publisher does not have was installed")
	}
	p.Close()
	if _, err := installer.Newest(t.Context(), Goose); err == nil {
		t.Error("a publisher that does not answer")
	}
}
