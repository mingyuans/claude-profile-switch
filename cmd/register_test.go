package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mingyuans/claude-profile-switch/internal/config"
)

// runRegister invokes the register command's RunE closure directly so tests
// stay independent of cobra's shared parser state.
func runRegister(t *testing.T, name, path string) error {
	t.Helper()
	registerShare = false
	return registerCmd.RunE(registerCmd, []string{name, path})
}

func TestRegister_RegistersExistingDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	existing := filepath.Join(tmp, ".claude-existing")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runRegister(t, "existing", existing); err != nil {
		t.Fatalf("register: %v", err)
	}

	store, err := config.Load(filepath.Join(tmp, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("existing")
	if err != nil {
		t.Fatalf("profile not registered: %v", err)
	}
	if got.Dir != existing {
		t.Errorf("dir = %q, want %q", got.Dir, existing)
	}
}

// TestRegister_RejectsMissingPath ensures the command refuses to register a
// non-existent directory — that's the whole point of having a separate
// command from `add`.
func TestRegister_RejectsMissingPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	missing := filepath.Join(tmp, "does-not-exist")
	err := runRegister(t, "ghost", missing)
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should mention 'does not exist'", err)
	}

	store, err := config.Load(filepath.Join(tmp, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("ghost"); err == nil {
		t.Error("profile should not be registered when path is missing")
	}
}

func TestRegister_RejectsFilePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	file := filepath.Join(tmp, "regular.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runRegister(t, "filey", file)
	if err == nil {
		t.Fatal("expected error for non-directory path, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q should mention 'not a directory'", err)
	}
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CCS_CONFIG_PATH", filepath.Join(tmp, "profiles.json"))
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".claude-dup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runRegister(t, "dup", dir); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := runRegister(t, "dup", dir); err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
}
