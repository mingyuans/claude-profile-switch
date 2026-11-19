package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mingyuans/claude-profile-switch/internal/selfupdate"
)

// setupUpdate points `ccs update` at a fake release server publishing tag
// with binary content, and at a fake installed binary. Returns its path.
func setupUpdate(t *testing.T, current, tag string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	payload := []byte("binary " + tag)
	_ = tw.WriteHeader(&tar.Header{Name: "ccs", Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(payload)
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	asset := fmt.Sprintf("ccs-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":%q}`, tag)
		case "/o/r/releases/download/" + tag + "/" + asset:
			w.Write(archive)
		case "/o/r/releases/download/" + tag + "/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	exe := filepath.Join(t.TempDir(), "ccs")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	origClient, origExe, origVersion := newUpdateClient, updateExecutable, version
	t.Cleanup(func() {
		newUpdateClient, updateExecutable, version = origClient, origExe, origVersion
		updateCheck, updateVersion, updateForce = false, "", false
	})
	newUpdateClient = func() *selfupdate.Client {
		return &selfupdate.Client{HTTP: srv.Client(), Repo: "o/r", APIBase: srv.URL, DownloadURL: srv.URL, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	}
	updateExecutable = func() (string, error) { return exe, nil }
	version = current
	updateCheck, updateVersion, updateForce = false, "", false
	return exe
}

func installed(t *testing.T, exe string) string {
	t.Helper()
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUpdate_InstallsNewerRelease(t *testing.T) {
	exe := setupUpdate(t, "v0.1.0", "v0.2.0")
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "binary v0.2.0" {
		t.Errorf("installed = %q", got)
	}
}

func TestUpdate_UpToDateIsNoop(t *testing.T) {
	exe := setupUpdate(t, "v0.2.0", "v0.2.0")
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "old" {
		t.Errorf("binary should be untouched, got %q", got)
	}
}

func TestUpdate_NeverDowngradesToLatest(t *testing.T) {
	exe := setupUpdate(t, "v0.3.0", "v0.2.0")
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "old" {
		t.Errorf("binary should be untouched, got %q", got)
	}
}

func TestUpdate_ExplicitVersionCanDowngrade(t *testing.T) {
	exe := setupUpdate(t, "v0.3.0", "v0.2.0")
	updateVersion = "v0.2.0"
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "binary v0.2.0" {
		t.Errorf("installed = %q", got)
	}
}

func TestUpdate_CheckDoesNotInstall(t *testing.T) {
	exe := setupUpdate(t, "v0.1.0", "v0.2.0")
	updateCheck = true
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "old" {
		t.Errorf("--check must not install, got %q", got)
	}
}

func TestUpdate_SourceBuildRequiresForce(t *testing.T) {
	exe := setupUpdate(t, "dev", "v0.2.0")
	err := updateCmd.RunE(updateCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force hint, got %v", err)
	}
	if got := installed(t, exe); got != "old" {
		t.Errorf("binary should be untouched, got %q", got)
	}

	updateForce = true
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := installed(t, exe); got != "binary v0.2.0" {
		t.Errorf("installed = %q", got)
	}
}
