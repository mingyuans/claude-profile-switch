package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// withIsolatedConfig redirects ccs to a fresh tempdir for both the JSON
// store path and HOME (so claudeDefaultDir resolves under tempdir too).
// Returns the resolved store path.
func withIsolatedConfig(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "profiles.json")
	t.Setenv("CCS_CONFIG_PATH", cfg)
	t.Setenv("HOME", tmp)
	return cfg
}

func TestLoadStore_FirstRunSeedsDefaultProfile(t *testing.T) {
	cfgPath := withIsolatedConfig(t)
	home, _ := os.UserHomeDir()

	store, err := loadStore()
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	got := store.List()
	if len(got) != 1 {
		t.Fatalf("expected 1 seeded profile, got %d", len(got))
	}
	if got[0].Name != defaultProfileName {
		t.Errorf("seeded profile name = %q, want %q", got[0].Name, defaultProfileName)
	}
	if got[0].Dir != filepath.Join(home, ".claude") {
		t.Errorf("seeded profile dir = %q, want ~/.claude", got[0].Dir)
	}
	if store.Current != defaultProfileName {
		t.Errorf("seeded store.Current = %q, want %q", store.Current, defaultProfileName)
	}

	// Seed should have been persisted so subsequent commands see it.
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("seed should have created config file: %v", err)
	}
}

func TestLoadStore_DoesNotReseedOnSubsequentRuns(t *testing.T) {
	withIsolatedConfig(t)

	// First run creates default.
	if _, err := loadStore(); err != nil {
		t.Fatal(err)
	}
	// Second run reads existing file without re-seeding.
	store, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(store.List()); got != 1 {
		t.Errorf("expected 1 profile on second load, got %d", got)
	}
}

func TestLoadStore_DoesNotResurrectAfterRemove(t *testing.T) {
	withIsolatedConfig(t)

	// Seed.
	store, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	// Remove default and persist.
	if _, err := store.Remove(defaultProfileName); err != nil {
		t.Fatalf("remove default: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload — file exists with empty profiles, must not re-seed.
	reloaded, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reloaded.List()); got != 0 {
		t.Errorf("expected 0 profiles after rm, got %d", got)
	}
}

func TestLoadStore_PreservesExistingProfilesOnFirstLoad(t *testing.T) {
	cfgPath := withIsolatedConfig(t)

	// Pre-create file with a custom profile (simulates a user who configured
	// ccs by hand). loadStore must NOT seed default in this case.
	hand := []byte(`{"current":"work","profiles":[{"name":"work","dir":"/tmp/w"}]}`)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, hand, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	got := store.List()
	if len(got) != 1 || got[0].Name != "work" {
		t.Errorf("expected only the pre-existing 'work' profile, got %+v", got)
	}
}
