package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTempStore(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load(empty): %v", err)
	}
	return s
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	s := newTempStore(t)
	if got := len(s.Profiles); got != 0 {
		t.Errorf("want 0 profiles, got %d", got)
	}
	if s.Path() == "" {
		t.Errorf("Path should be populated even on miss")
	}
}

func TestAdd_RejectsDuplicateAndInvalid(t *testing.T) {
	s := newTempStore(t)
	if err := s.Add(Profile{Name: "work", Dir: "/tmp/work"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := s.Add(Profile{Name: "work", Dir: "/tmp/work2"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("want ErrAlreadyExists, got %v", err)
	}

	cases := []Profile{
		{Name: "", Dir: "/tmp/x"},
		{Name: "bad name", Dir: "/tmp/x"},
		{Name: "ok", Dir: ""},
		{Name: "ok", Dir: "relative/dir"},
	}
	for _, c := range cases {
		if err := s.Add(c); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{
		"work",
		"jimmy.yan",
		"team-a",
		"work_v2",
		"alice.bob_2024",
		"_internal",
		"2-personal",
		"a",
	}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) returned %v, want nil", name, err)
		}
	}

	invalid := map[string]string{
		"":          "empty",
		"   ":       "whitespace",
		"bad name":  "contains space",
		".hidden":   "leading dot",
		"-flag":     "leading dash",
		"name.":     "trailing dot",
		"name-":     "trailing dash",
		"a..b":      "contains '..'",
		"foo/bar":   "path separator",
		"foo:bar":   "colon",
		"naïve":     "non-ascii",
		"work\tabc": "tab",
	}
	for name, why := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) returned nil, want error (%s)", name, why)
		}
	}
}

func TestRemove_ClearsCurrentWhenMatching(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add(Profile{Name: "work", Dir: "/tmp/work"})
	_ = s.Add(Profile{Name: "personal", Dir: "/tmp/p"})
	if err := s.SetCurrent("work"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Remove("work"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if s.Current != "" {
		t.Errorf("Current should be cleared, got %q", s.Current)
	}

	if _, err := s.Remove("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for missing, got %v", err)
	}
}

func TestSetCurrent_RequiresRegistered(t *testing.T) {
	s := newTempStore(t)
	if err := s.SetCurrent("ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
	_ = s.Add(Profile{Name: "work", Dir: "/tmp/work"})
	if err := s.SetCurrent("work"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	if s.Current != "work" {
		t.Errorf("Current = %q, want work", s.Current)
	}
}

func TestList_ReturnsSorted(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add(Profile{Name: "zeta", Dir: "/tmp/z"})
	_ = s.Add(Profile{Name: "alpha", Dir: "/tmp/a"})
	_ = s.Add(Profile{Name: "beta", Dir: "/tmp/b"})

	got := s.List()
	want := []string{"alpha", "beta", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, p := range got {
		if p.Name != want[i] {
			t.Errorf("[%d] = %s, want %s", i, p.Name, want[i])
		}
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add(Profile{Name: "work", Dir: "/tmp/work"})
	_ = s.Add(Profile{Name: "personal", Dir: "/tmp/p"})
	_ = s.SetCurrent("work")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(s.Path())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Current != "work" {
		t.Errorf("reloaded.Current = %q, want work", reloaded.Current)
	}
	if len(reloaded.Profiles) != 2 {
		t.Fatalf("reloaded profiles len = %d, want 2", len(reloaded.Profiles))
	}
}

func TestSave_AtomicTmpThenRename(t *testing.T) {
	s := newTempStore(t)
	_ = s.Add(Profile{Name: "x", Dir: "/tmp/x"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(s.Path() + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".tmp file should be cleaned up, stat err = %v", err)
	}
	if _, err := os.Stat(s.Path()); err != nil {
		t.Errorf("primary file missing: %v", err)
	}
}

func TestDefaultPath_HonoursCCSConfigPath(t *testing.T) {
	t.Setenv("CCS_CONFIG_PATH", "/tmp/override.json")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/override.json" {
		t.Errorf("DefaultPath = %s, want override", got)
	}
}

func TestDefaultPath_HonoursXDGHome(t *testing.T) {
	t.Setenv("CCS_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/xdg/ccs/profiles.json" {
		t.Errorf("DefaultPath = %s, want xdg-derived path", got)
	}
}

func TestExpandDir_HandlesTildeAndEnv(t *testing.T) {
	home, _ := os.UserHomeDir()
	got, err := ExpandDir("~/foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "foo") {
		t.Errorf("ExpandDir(~/foo) = %s, want %s", got, filepath.Join(home, "foo"))
	}

	t.Setenv("CCS_TEST_VAR", "/tmp/envvar")
	got, err = ExpandDir("$CCS_TEST_VAR/sub")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/envvar/sub" {
		t.Errorf("ExpandDir($CCS_TEST_VAR/sub) = %s", got)
	}
}
