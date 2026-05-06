// Package cmd wires cobra subcommands and is the only place that owns
// process-level concerns (stdout writer, exit code, store path resolution).
// Subcommand files (add.go, list.go, …) only describe what to do; rendering
// is delegated to internal/output and persistence to internal/config.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/spf13/cobra"
)

// defaultProfileName is the implicit profile auto-registered on first run so
// `ccs list` is never empty for a brand-new user — Claude Code itself uses
// ~/.claude when CLAUDE_CONFIG_DIR is unset.
const defaultProfileName = "default"

// version is overridden at build time via -ldflags "-X cmd.version=...".
var version = "dev"

// rootCmd is the top-level `ccs` command.
var rootCmd = &cobra.Command{
	Use:           "ccs",
	Short:         "Switch between Claude Code account profiles by toggling CLAUDE_CONFIG_DIR.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command, mapping any error to a non-zero exit code
// and rendering it through the standard output pipeline.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		r := output.Default()
		r.Error("%v", err)
		os.Exit(1)
	}
}

// loadStore is called by every subcommand. It resolves the on-disk path,
// reads it (missing file is fine), and returns a Store ready to mutate.
//
// On first run (the JSON file does not yet exist) it seeds a "default"
// profile pointing at ~/.claude and persists it, so subsequent commands —
// and especially `ccs list` — always show at least the implicit Claude Code
// profile. Once the file exists, no further auto-seeding happens, so a user
// who explicitly removes "default" stays removed.
func loadStore() (config.Store, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.Store{}, fmt.Errorf("resolve config path: %w", err)
	}
	firstRun := !fileExists(path)

	store, err := config.Load(path)
	if err != nil {
		return store, err
	}
	if !firstRun {
		return store, nil
	}
	if seedErr := seedDefaultProfile(&store); seedErr != nil {
		// Seeding is best-effort: a failure (e.g. unresolvable home dir)
		// must not block the command the user actually wants to run.
		return store, nil
	}
	if err := store.Save(); err != nil {
		return store, fmt.Errorf("persist seeded default profile: %w", err)
	}
	return store, nil
}

// seedDefaultProfile registers the canonical ~/.claude directory under the
// name "default" and marks it current — a fresh shell with no
// $CLAUDE_CONFIG_DIR override is in fact running on ~/.claude, so that's
// the profile we should reflect as active until the user switches.
func seedDefaultProfile(store *config.Store) error {
	dir, err := claudeDefaultDir()
	if err != nil {
		return err
	}
	if err := store.Add(config.Profile{Name: defaultProfileName, Dir: dir}); err != nil {
		return err
	}
	return store.SetCurrent(defaultProfileName)
}

// claudeDefaultDir returns the directory Claude Code uses when
// CLAUDE_CONFIG_DIR is unset (~/.claude).
func claudeDefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// defaultProfileDir is the directory ccs.add picks when the user passes a
// name but no path. We intentionally pick a sibling of the canonical
// `~/.claude` dir so files stay grouped under the user's home.
func defaultProfileDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, fmt.Sprintf(".claude-%s", name)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// On any other stat error treat the file as existing — we'd rather skip
	// seeding than risk overwriting a state we can't introspect.
	return true
}
