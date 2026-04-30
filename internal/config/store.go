// Package config persists ccs-cli profile registrations as JSON. The on-disk
// schema is intentionally tiny — name + filesystem directory per profile —
// because CLAUDE_CONFIG_DIR fully describes a Claude Code account.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	configDirName  = "ccs"
	configFileName = "profiles.json"
	defaultPerm    = 0o755
	filePerm       = 0o644
)

// Profile maps a human-readable name to the directory CLAUDE_CONFIG_DIR
// should be set to.
type Profile struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// Store is the on-disk document. Current is the last `ccs use` selection;
// callers compare it with the live $CLAUDE_CONFIG_DIR to decide what
// "current" means in different contexts.
type Store struct {
	Current  string    `json:"current"`
	Profiles []Profile `json:"profiles"`

	path string
}

// ErrNotFound is returned by Get/Remove when the named profile is missing.
var ErrNotFound = errors.New("profile not found")

// ErrAlreadyExists is returned by Add when the name is already registered.
var ErrAlreadyExists = errors.New("profile already exists")

// DefaultPath resolves the JSON store path, honouring XDG_CONFIG_HOME and
// falling back to $HOME/.config/ccs/profiles.json. The returned path is
// guaranteed absolute but may not yet exist on disk.
func DefaultPath() (string, error) {
	if base := os.Getenv("CCS_CONFIG_PATH"); base != "" {
		return base, nil
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, configDirName, configFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", configDirName, configFileName), nil
}

// Load reads the store at path. A missing file is *not* an error; an empty
// Store is returned so first-run Add succeeds.
func Load(path string) (Store, error) {
	s := Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", path, err)
	}
	s.path = path
	return s, nil
}

// LoadDefault is sugar for Load(DefaultPath()).
func LoadDefault() (Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Load(p)
}

// Path returns the absolute filesystem path the store is bound to.
func (s Store) Path() string { return s.path }

// Save serialises the store atomically: write to a sibling tmp file, then
// rename onto the target. A crash mid-write leaves the previous file intact.
func (s Store) Save() error {
	if s.path == "" {
		return errors.New("store has no path; load via Load() first")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), defaultPerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	out := struct {
		Current  string    `json:"current"`
		Profiles []Profile `json:"profiles"`
	}{
		Current:  s.Current,
		Profiles: s.sortedProfiles(),
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, s.path, err)
	}
	return nil
}

// Add registers p; returns ErrAlreadyExists if the name is taken.
func (s *Store) Add(p Profile) error {
	if err := validateProfile(p); err != nil {
		return err
	}
	if _, ok := s.find(p.Name); ok {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, p.Name)
	}
	s.Profiles = append(s.Profiles, p)
	return nil
}

// Remove deletes the profile with the given name. Returns ErrNotFound if
// no such profile is registered. If the removed profile was Current, Current
// is cleared.
func (s *Store) Remove(name string) (Profile, error) {
	idx, ok := s.find(name)
	if !ok {
		return Profile{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	removed := s.Profiles[idx]
	s.Profiles = append(s.Profiles[:idx], s.Profiles[idx+1:]...)
	if s.Current == name {
		s.Current = ""
	}
	return removed, nil
}

// Get fetches a profile by name.
func (s Store) Get(name string) (Profile, error) {
	idx, ok := s.find(name)
	if !ok {
		return Profile{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return s.Profiles[idx], nil
}

// List returns profiles sorted by name (stable for table rendering).
func (s Store) List() []Profile {
	return s.sortedProfiles()
}

// SetCurrent records name as the active profile. Returns ErrNotFound if
// the profile is not registered.
func (s *Store) SetCurrent(name string) error {
	if _, ok := s.find(name); !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	s.Current = name
	return nil
}

func (s Store) find(name string) (int, bool) {
	for i, p := range s.Profiles {
		if p.Name == name {
			return i, true
		}
	}
	return 0, false
}

func (s Store) sortedProfiles() []Profile {
	out := make([]Profile, len(s.Profiles))
	copy(out, s.Profiles)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// validateProfile applies the rules a name/dir must satisfy. Names are
// restricted to a small character class so they can be used unquoted in
// shell commands and as path segments — see ValidateName for the exact
// grammar.
func validateProfile(p Profile) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if strings.TrimSpace(p.Dir) == "" {
		return errors.New("profile directory is required")
	}
	if !filepath.IsAbs(p.Dir) {
		return fmt.Errorf("profile directory must be absolute: %s", p.Dir)
	}
	return nil
}

// ValidateName reports whether name is a syntactically valid profile name.
// The grammar:
//
//   - allowed characters: letters, digits, '.', '-', '_'
//   - must start with a letter, digit, or '_' (no leading '.' or '-' so the
//     name doesn't collide with shell flags or hidden-file conventions)
//   - must not end with '.' or '-' (avoids awkward path segments)
//   - must not contain ".." (defence-in-depth against path-traversal abuse
//     even though Dir is a separate field)
//
// Returns nil if the name is valid; otherwise an error describing the rule.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("profile name is required")
	}
	if !isSafeName(name) {
		return fmt.Errorf("invalid profile name %q: use letters, digits, '.', '-' or '_'; must start with letter/digit/'_', no leading/trailing '.' or '-', no '..'", name)
	}
	return nil
}

func isSafeName(s string) bool {
	if s == "" || strings.Contains(s, "..") {
		return false
	}
	runes := []rune(s)
	if !isNameStart(runes[0]) {
		return false
	}
	if last := runes[len(runes)-1]; last == '.' || last == '-' {
		return false
	}
	for _, r := range runes {
		if !isNameRune(r) {
			return false
		}
	}
	return true
}

func isNameStart(r rune) bool { return isAlnum(r) || r == '_' }
func isNameRune(r rune) bool  { return isAlnum(r) || r == '.' || r == '-' || r == '_' }
func isAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// ExpandDir resolves ~ and environment variables so callers can keep raw
// strings in the JSON file but feed an absolute path to os.MkdirAll/etc.
func ExpandDir(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("empty path")
	}
	expanded := os.ExpandEnv(dir)
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve abs path: %w", err)
	}
	return abs, nil
}
