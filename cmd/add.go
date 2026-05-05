package cmd

import (
	"fmt"
	"os"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/spf13/cobra"
)

var (
	addNoCreate bool
	addNoShare  bool
)

var addCmd = &cobra.Command{
	Use:   "add <name> [path]",
	Short: "Register a new profile (creates the directory if it does not exist)",
	Long: `Register a new profile.

If [path] is omitted, defaults to ~/.claude-<name>. The target directory
is created automatically (use --no-create to skip directory creation).
The profile is *not* activated by this command — run 'ccs use <name>'
afterwards.

By default, shareable items from ~/.claude (CLAUDE.md, agents/, commands/,
skills/, output-styles/, keybindings.json, hooks/, plugins/, settings.json)
are symlinked into the new profile so user-level extensions and preferences
are reused across accounts. Credentials, sessions, projects/, todos/ and
settings.local.json stay isolated. Pass --no-share to skip the symlink step
entirely.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := loadStore()
		if err != nil {
			return err
		}
		r := output.Default()
		r.Header("Adding profile %s", r.Cyan(name))

		// Validate name *before* touching the filesystem so a rejected
		// name doesn't leave behind an orphaned directory.
		if err := config.ValidateName(name); err != nil {
			return err
		}

		dir, err := resolveAddDir(name, args)
		if err != nil {
			return err
		}

		if !addNoCreate {
			if err := ensureDir(dir, r); err != nil {
				return err
			}
		} else {
			r.Skip("dir creation skipped (--no-create): %s", r.Dim(dir))
		}

		if err := store.Add(config.Profile{Name: name, Dir: dir}); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		r.Success("registered %s -> %s", r.Cyan(name), r.Dim(dir))

		if !addNoShare {
			source, err := claudeDefaultDir()
			if err != nil {
				r.Warning("share skipped: %v", err)
			} else {
				results, err := linkSharedItems(source, dir)
				if err != nil {
					// Linker fatal failure — surface but don't roll back
					// the registration; the user can re-link by hand.
					r.Warning("share aborted: %v", err)
				}
				renderShareResults(r, results)
			}
		} else {
			r.Skip("share skipped (--no-share)")
		}

		r.Info("activate with: %s", r.Bold("ccs use "+name))
		return nil
	},
}

func resolveAddDir(name string, args []string) (string, error) {
	if len(args) == 2 {
		return config.ExpandDir(args[1])
	}
	defaultDir, err := defaultProfileDir(name)
	if err != nil {
		return "", err
	}
	return config.ExpandDir(defaultDir)
}

func ensureDir(dir string, r *output.Renderer) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		r.Info("dir already exists: %s", r.Dim(dir))
		return nil
	case err == nil && !info.IsDir():
		return fmt.Errorf("path exists but is not a directory: %s", dir)
	case os.IsNotExist(err):
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("create dir %s: %w", dir, mkErr)
		}
		r.Success("created dir: %s", r.Dim(dir))
		return nil
	default:
		return fmt.Errorf("stat %s: %w", dir, err)
	}
}

func init() {
	addCmd.Flags().BoolVar(&addNoCreate, "no-create", false, "do not create the profile directory")
	addCmd.Flags().BoolVar(&addNoShare, "no-share", false, "do not symlink shareable items (CLAUDE.md, agents/, commands/, skills/, ...) from ~/.claude")
	rootCmd.AddCommand(addCmd)
}
