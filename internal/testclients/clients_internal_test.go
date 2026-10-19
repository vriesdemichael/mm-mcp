package testclients

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThePinsAreReadFromToolVersions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, filepath.Join(root, ".github", "tool-versions.env"), "# a comment\nGOOSE_VERSION=1.55.0\nOTHER=1\nVSCODE_VERSION=v1.142.0\n")
	pinned, err := Pinned(root)
	if err != nil || pinned[Goose] != "1.55.0" || pinned[VSCode] != "1.142.0" {
		t.Fatalf("read %v, %v", pinned, err)
	}
	write(t, filepath.Join(root, ".github", "tool-versions.env"), "GOOSE_VERSION=1.55.0\n")
	if _, err := Pinned(root); err == nil || !strings.Contains(err.Error(), "VSCODE_VERSION") {
		t.Errorf("a missing pin: %v", err)
	}
}

// The repository's own file pins both.
func TestTheRepositoryPinsEveryClient(t *testing.T) {
	t.Parallel()
	if _, err := Pinned(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
}

func TestAClientIsFoundOnlyOnceItsInstallationFinished(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	linux := Platform{"linux", "amd64"}
	dir := Dir(root, VSCode, "1.142.0", linux)
	write(t, filepath.Join(dir, "VSCode-linux-x64", "code"), "")
	if _, err := Executable(root, VSCode, "1.142.0", linux); err == nil || !strings.Contains(err.Error(), "task test:clients") {
		t.Errorf("an unfinished installation was found: %v", err)
	}
	write(t, filepath.Join(dir, installedMark), "sum\n")
	if program, err := Executable(root, VSCode, "1.142.0", linux); err != nil || program != filepath.Join(dir, "VSCode-linux-x64", "code") {
		t.Errorf("found %q, %v", program, err)
	}
	// A folder the archive wraps its contents in.
	windows := Platform{"windows", "amd64"}
	wrapped := Dir(root, Goose, "1.55.0", windows)
	write(t, filepath.Join(wrapped, "Goose-win32-x64", "Goose.exe"), "")
	write(t, filepath.Join(wrapped, installedMark), "sum\n")
	if program, err := Executable(root, Goose, "1.55.0", windows); err != nil || program != filepath.Join(wrapped, "Goose-win32-x64", "Goose.exe") {
		t.Errorf("found %q, %v", program, err)
	}
	if _, err := Executable(root, Goose, "1.55.0", Platform{"plan9", "amd64"}); err == nil {
		t.Error("a platform no client runs on")
	}
}

func TestArchivesUnpackInsideTheirFolderOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var zipped bytes.Buffer
	archive := zip.NewWriter(&zipped)
	entry, _ := archive.Create("app/Code.exe")
	_, _ = entry.Write([]byte("program"))
	_ = archive.Close()
	zipPath := filepath.Join(dir, "a.zip")
	write(t, zipPath, zipped.String())
	if err := unpack("zip", zipPath, filepath.Join(dir, "zip")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "zip", "app", "Code.exe")); string(got) != "program" {
		t.Errorf("unzipped %q", got)
	}

	var tarred bytes.Buffer
	gz := gzip.NewWriter(&tarred)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "VSCode-linux-x64/", Typeflag: tar.TypeDir, Mode: 0o755})
	_ = tw.WriteHeader(&tar.Header{Name: "VSCode-linux-x64/code", Typeflag: tar.TypeReg, Mode: 0o755, Size: 7})
	_, _ = tw.Write([]byte("program"))
	_ = tw.Close()
	_ = gz.Close()
	tarPath := filepath.Join(dir, "a.tar.gz")
	write(t, tarPath, tarred.String())
	if err := unpack("tar.gz", tarPath, filepath.Join(dir, "tar")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "tar", "VSCode-linux-x64", "code")); string(got) != "program" {
		t.Errorf("untarred %q", got)
	}

	var climbing bytes.Buffer
	evil := zip.NewWriter(&climbing)
	entry, _ = evil.Create("../escaped")
	_, _ = entry.Write([]byte("x"))
	_ = evil.Close()
	evilPath := filepath.Join(dir, "evil.zip")
	write(t, evilPath, climbing.String())
	if err := unpack("zip", evilPath, filepath.Join(dir, "evil")); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("an entry that climbs out: %v", err)
	}
	if err := unpack("rar", evilPath, dir); err == nil {
		t.Error("an unknown kind was unpacked")
	}
}

// A download whose checksum is not its publisher's is refused.
func TestADownloadIsCheckedAgainstItsPublishersChecksum(t *testing.T) {
	t.Parallel()
	served := []byte("the client")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(served) }))
	defer server.Close()
	sum := sha256.Sum256(served)
	path := filepath.Join(t.TempDir(), "client.zip")
	if err := (Installer{}).download(t.Context(), Download{URL: server.URL, SHA256: hex.EncodeToString(sum[:])}, path); err != nil {
		t.Fatal(err)
	}
	if err := (Installer{}).download(t.Context(), Download{URL: server.URL, SHA256: strings.Repeat("0", 64)}, path); err == nil || !strings.Contains(err.Error(), "publisher gives") {
		t.Errorf("a wrong checksum: %v", err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
