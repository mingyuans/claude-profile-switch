// Package selfupdate replaces the running ccs binary with a release
// downloaded from GitHub. It mirrors install.sh: same repo, same asset
// naming (ccs-<os>-<arch>.tar.gz) and verifies the archive against the
// release's checksums.txt before touching the installed binary.
package selfupdate

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultRepo is the GitHub slug releases are fetched from; CCS_REPO
// overrides it, matching install.sh.
const DefaultRepo = "mingyuans/claude-profile-switch"

// binaryName is the file expected inside each release tarball.
const binaryName = "ccs"

// Client talks to the GitHub API / download host. Base URLs are fields so
// tests can point them at an httptest server.
type Client struct {
	HTTP        *http.Client
	Repo        string
	APIBase     string // e.g. https://api.github.com
	DownloadURL string // e.g. https://github.com
	Token       string // optional, sent as Bearer to the API to lift rate limits
	GOOS        string
	GOARCH      string
}

// NewClient returns a Client configured from the environment
// (CCS_REPO, GITHUB_TOKEN) for the current platform.
func NewClient() *Client {
	repo := os.Getenv("CCS_REPO")
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{
		HTTP:        &http.Client{Timeout: 60 * time.Second},
		Repo:        repo,
		APIBase:     "https://api.github.com",
		DownloadURL: "https://github.com",
		Token:       os.Getenv("GITHUB_TOKEN"),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
	}
}

// AssetName is the tarball name for the client's platform.
func (c *Client) AssetName() string {
	return fmt.Sprintf("%s-%s-%s.tar.gz", binaryName, c.GOOS, c.GOARCH)
}

// LatestTag returns the tag_name of the repo's latest (non-prerelease) release.
func (c *Client) LatestTag() (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.APIBase, c.Repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query latest release: %s returned %s", url, resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	if body.TagName == "" {
		return "", errors.New("latest release has no tag_name")
	}
	return body.TagName, nil
}

// Download fetches the platform tarball for tag, verifies it against the
// release's checksums.txt and returns the extracted ccs binary bytes.
func (c *Client) Download(tag string) ([]byte, error) {
	asset := c.AssetName()
	archive, err := c.get(c.releaseURL(tag, asset))
	if err != nil {
		return nil, err
	}
	sums, err := c.get(c.releaseURL(tag, "checksums.txt"))
	if err != nil {
		return nil, err
	}
	if err := verifyChecksum(sums, asset, archive); err != nil {
		return nil, err
	}
	return extractBinary(archive)
}

func (c *Client) releaseURL(tag, file string) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/%s", c.DownloadURL, c.Repo, tag, file)
}

func (c *Client) get(url string) ([]byte, error) {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return data, nil
}

// verifyChecksum checks data against the `<sha256>  <name>` line for name
// in a shasum-style checksums file.
func verifyChecksum(sums []byte, name string, data []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, fields[0]) {
			return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, fields[0])
		}
		return nil
	}
	return fmt.Errorf("checksums.txt has no entry for %s", name)
}

// extractBinary returns the contents of the top-level "ccs" file in a
// .tar.gz archive.
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("archive did not contain a %q binary", binaryName)
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Clean(hdr.Name) == binaryName {
			return io.ReadAll(tr)
		}
	}
}

// Replace atomically swaps the file at target for bin: it writes a temp
// file in the same directory (so rename stays on one filesystem), keeps
// the original permission bits, then renames it over target.
func Replace(target string, bin []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".ccs-update-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("replace %s: %w", target, err)
	}
	return nil
}

// ExecutablePath resolves the path of the running binary, following
// symlinks so we replace the real file rather than a link to it.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running binary: %w", err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", exe, err)
	}
	return real, nil
}

// Semver is a parsed vMAJOR.MINOR.PATCH version; any suffix such as
// "-rc1" or git-describe's "-3-gabc123-dirty" is ignored.
type Semver [3]int

// ParseSemver parses "v1.2.3" (leading "v" optional, suffix ignored).
func ParseSemver(s string) (Semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Semver{}, false
	}
	var v Semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Semver{}, false
		}
		v[i] = n
	}
	return v, true
}

// Compare returns -1, 0 or 1 as a is older than, equal to, or newer than b.
func (a Semver) Compare(b Semver) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}
