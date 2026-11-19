package selfupdate

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
	"strings"
	"testing"
)

// MakeTarball builds a .tar.gz holding a single file name with content.
func makeTarball(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sha(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// fakeRelease serves /repos/<repo>/releases/latest and the download URLs.
func fakeRelease(t *testing.T, tag string, files map[string][]byte) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/releases/latest" {
			fmt.Fprintf(w, `{"tag_name":%q}`, tag)
			return
		}
		prefix := "/o/r/releases/download/" + tag + "/"
		if name, ok := strings.CutPrefix(r.URL.Path, prefix); ok {
			if data, ok := files[name]; ok {
				w.Write(data)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client(), Repo: "o/r", APIBase: srv.URL, DownloadURL: srv.URL, GOOS: "linux", GOARCH: "amd64"}
}

func TestLatestTag(t *testing.T) {
	c := fakeRelease(t, "v1.2.3", nil)
	tag, err := c.LatestTag()
	if err != nil || tag != "v1.2.3" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
}

func TestDownload_VerifiesAndExtracts(t *testing.T) {
	archive := makeTarball(t, "ccs", []byte("new-binary"))
	c := fakeRelease(t, "v1.0.0", map[string][]byte{
		"ccs-linux-amd64.tar.gz": archive,
		"checksums.txt":          []byte(sha([]byte("other")) + "  ccs-darwin-arm64.tar.gz\n" + sha(archive) + "  ccs-linux-amd64.tar.gz\n"),
	})
	bin, err := c.Download("v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(bin) != "new-binary" {
		t.Errorf("binary = %q", bin)
	}
}

func TestDownload_ChecksumMismatch(t *testing.T) {
	archive := makeTarball(t, "ccs", []byte("new-binary"))
	c := fakeRelease(t, "v1.0.0", map[string][]byte{
		"ccs-linux-amd64.tar.gz": archive,
		"checksums.txt":          []byte(sha([]byte("tampered")) + "  ccs-linux-amd64.tar.gz\n"),
	})
	if _, err := c.Download("v1.0.0"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestDownload_MissingChecksumEntry(t *testing.T) {
	archive := makeTarball(t, "ccs", []byte("x"))
	c := fakeRelease(t, "v1.0.0", map[string][]byte{
		"ccs-linux-amd64.tar.gz": archive,
		"checksums.txt":          []byte(sha(archive) + "  ccs-darwin-arm64.tar.gz\n"),
	})
	if _, err := c.Download("v1.0.0"); err == nil || !strings.Contains(err.Error(), "no entry") {
		t.Fatalf("expected missing entry error, got %v", err)
	}
}

func TestDownload_ArchiveWithoutBinary(t *testing.T) {
	archive := makeTarball(t, "README", []byte("x"))
	c := fakeRelease(t, "v1.0.0", map[string][]byte{
		"ccs-linux-amd64.tar.gz": archive,
		"checksums.txt":          []byte(sha(archive) + "  ccs-linux-amd64.tar.gz\n"),
	})
	if _, err := c.Download("v1.0.0"); err == nil || !strings.Contains(err.Error(), "did not contain") {
		t.Fatalf("expected missing binary error, got %v", err)
	}
}

func TestReplace_KeepsModeAndSwapsContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ccs")
	if err := os.WriteFile(target, []byte("old"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := Replace(target, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("content = %q", got)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %v, want 0750", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestParseSemverAndCompare(t *testing.T) {
	cases := []struct {
		in string
		ok bool
		v  Semver
	}{
		{"v0.1.2", true, Semver{0, 1, 2}},
		{"1.10.0", true, Semver{1, 10, 0}},
		{"v0.1.2-3-gabc123-dirty", true, Semver{0, 1, 2}},
		{"dev", false, Semver{}},
		{"dc48034", false, Semver{}},
		{"v1.2", false, Semver{}},
	}
	for _, c := range cases {
		v, ok := ParseSemver(c.in)
		if ok != c.ok || v != c.v {
			t.Errorf("ParseSemver(%q) = %v, %v; want %v, %v", c.in, v, ok, c.v, c.ok)
		}
	}
	if (Semver{0, 1, 9}).Compare(Semver{0, 1, 10}) != -1 {
		t.Error("0.1.9 should be older than 0.1.10")
	}
	if (Semver{1, 0, 0}).Compare(Semver{0, 9, 9}) != 1 {
		t.Error("1.0.0 should be newer than 0.9.9")
	}
}
