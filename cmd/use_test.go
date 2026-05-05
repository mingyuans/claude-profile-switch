package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/rcfile"
)

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/Users/foo/.claude-work", `'/Users/foo/.claude-work'`},
		{"/tmp/with space", `'/tmp/with space'`},
		{"/tmp/it's", `'/tmp/it'\''s'`},
		{"", `''`},
	}
	for _, c := range cases {
		got := shellQuote(c.in)
		if got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// runUse invokes the use command's RunE closure directly. Resets package
// vars between calls so flags don't leak across tests.
func runUse(t *testing.T, name string, noRc bool) error {
	t.Helper()
	useExport = false
	useNoRc = noRc
	return useCmd.RunE(useCmd, []string{name})
}

// seedProfile registers profile `name` in the test store so `ccs use` can
// find it. Returns the resolved profile dir.
func seedProfile(t *testing.T, name string) string {
	t.Helper()
	store, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := defaultProfileDir(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add(config.Profile{Name: name, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUse_PersistsExportToRcFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")

	dir := seedProfile(t, "work")

	if err := runUse(t, "work", false); err != nil {
		t.Fatalf("use work: %v", err)
	}

	rcPath := filepath.Join(tmp, ".zshrc")
	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, rcfile.BeginMarker) {
		t.Errorf("missing begin marker in .zshrc:\n%s", got)
	}
	if !strings.Contains(got, "export CLAUDE_CONFIG_DIR='"+dir+"'") {
		t.Errorf("missing export line for %q in .zshrc:\n%s", dir, got)
	}
}

func TestUse_RewritesBlockOnSecondSwitch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")

	fooDir := seedProfile(t, "foo")
	barDir := seedProfile(t, "bar")

	if err := runUse(t, "foo", false); err != nil {
		t.Fatal(err)
	}
	if err := runUse(t, "bar", false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	got := string(data)
	if strings.Contains(got, fooDir) {
		t.Errorf("old profile dir %q still in .zshrc:\n%s", fooDir, got)
	}
	if !strings.Contains(got, barDir) {
		t.Errorf("new profile dir %q missing from .zshrc:\n%s", barDir, got)
	}
	if n := strings.Count(got, rcfile.BeginMarker); n != 1 {
		t.Errorf("expected exactly 1 managed block, got %d:\n%s", n, got)
	}
}

func TestUse_RespectsNoRcFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")

	seedProfile(t, "work")
	if err := runUse(t, "work", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".zshrc")); !os.IsNotExist(err) {
		t.Errorf("expected .zshrc not to exist, stat err = %v", err)
	}
}

func TestUse_RespectsCcsNoRcEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")
	t.Setenv("CCS_NO_RC", "1")

	seedProfile(t, "work")
	if err := runUse(t, "work", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".zshrc")); !os.IsNotExist(err) {
		t.Errorf("expected .zshrc not to exist, stat err = %v", err)
	}
}

func TestUse_UnsupportedShellDoesNotFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)
	t.Setenv("CCS_SHELL", "/bin/dash")

	seedProfile(t, "work")

	if err := runUse(t, "work", false); err != nil {
		t.Errorf("use should not fail when shell is unsupported, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".zshrc")); !os.IsNotExist(err) {
		t.Errorf("expected no rc file written for unsupported shell")
	}
}
