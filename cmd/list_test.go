package cmd

import (
	"path/filepath"
	"testing"

	"github.com/mingyuans/claude-profile-switch/internal/rcfile"
)

// liveDirForTest exercises the same code path list.go uses to decide
// the "live" profile, so tests don't have to reach into private helpers
// or render the table to assert behaviour.
func liveDirForTest(t *testing.T) string {
	t.Helper()
	return resolveLiveDir()
}

func TestList_LiveDir_ReadsFromRcBlock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")
	// Pollute the env var so we'd notice if the resolver fell back to it.
	t.Setenv("CLAUDE_CONFIG_DIR", "/from/env/should-not-be-used")

	want := filepath.Join(tmp, ".claude-work")
	line, err := rcfile.ExportLine("zsh", want)
	if err != nil {
		t.Fatal(err)
	}
	if err := rcfile.Update(filepath.Join(tmp, ".zshrc"), line); err != nil {
		t.Fatal(err)
	}

	got := liveDirForTest(t)
	if got != want {
		t.Errorf("got %q, want %q (must come from .zshrc, not env)", got, want)
	}
}

func TestList_LiveDir_EmptyWhenRcMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("CCS_SHELL", "zsh")
	t.Setenv("CLAUDE_CONFIG_DIR", "/some/dir") // must be ignored

	if got := liveDirForTest(t); got != "" {
		t.Errorf("got %q, want empty (rc missing)", got)
	}
}

func TestList_LiveDir_EmptyWhenShellUnsupported(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CCS_SHELL", "/bin/dash")

	if got := liveDirForTest(t); got != "" {
		t.Errorf("got %q, want empty (shell unsupported)", got)
	}
}
