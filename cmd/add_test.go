package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aftership/ccs-cli/internal/config"
)

// runAdd invokes the add command's RunE closure directly so tests stay
// independent of cobra's shared parser state.
func runAdd(t *testing.T, name string) error {
	t.Helper()
	addNoCreate = false
	addNoShare = true
	return addCmd.RunE(addCmd, []string{name})
}

// TestAdd_RejectedNameDoesNotCreateDirectory ensures validation runs
// before mkdir so a bad name doesn't leave an orphan directory behind.
func TestAdd_RejectedNameDoesNotCreateDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	if err := runAdd(t, ".bad-name"); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(tmp, ".claude-.bad-name")); !os.IsNotExist(statErr) {
		t.Errorf("orphan dir should not exist, stat err = %v", statErr)
	}
}

// TestAdd_AcceptsDottedNames covers the user-reported case `jimmy.yan`.
func TestAdd_AcceptsDottedNames(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	if err := runAdd(t, "jimmy.yan"); err != nil {
		t.Fatalf("add jimmy.yan: %v", err)
	}

	store, err := config.Load(filepath.Join(tmp, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("jimmy.yan"); err != nil {
		t.Errorf("profile not registered: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".claude-jimmy.yan")); err != nil {
		t.Errorf("expected dir created: %v", err)
	}
}
