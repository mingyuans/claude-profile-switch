package cmd

import (
	"fmt"
	"os"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/spf13/cobra"
)

var registerShare bool

var registerCmd = &cobra.Command{
	Use:     "register <name> <path>",
	Aliases: []string{"import"},
	Short:   "Register an existing Claude profile directory under a name",
	Long: `Register an existing Claude profile directory so ccs can manage it.

Unlike 'ccs add', this command does NOT create a new directory and does NOT
symlink shareable items by default — it assumes <path> is already a working
Claude config directory (e.g. another ~/.claude-* you set up by hand). The
path must exist and be a directory.

Pass --share to opt in to symlinking shareable items (CLAUDE.md, agents/,
commands/, skills/, output-styles/, keybindings.json, hooks/, plugins/,
settings.json) from ~/.claude into the registered directory.

The profile is *not* activated by this command — run 'ccs use <name>'
afterwards.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := loadStore()
		if err != nil {
			return err
		}
		r := output.Default()
		r.Header("Registering profile %s", r.Cyan(name))

		if err := config.ValidateName(name); err != nil {
			return err
		}

		dir, err := config.ExpandDir(args[1])
		if err != nil {
			return err
		}

		if err := requireExistingDir(dir); err != nil {
			return err
		}

		if err := store.Add(config.Profile{Name: name, Dir: dir}); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		r.Success("registered %s -> %s", r.Cyan(name), r.Dim(dir))

		if registerShare {
			source, err := claudeDefaultDir()
			if err != nil {
				r.Warning("share skipped: %v", err)
			} else {
				results, err := linkSharedItems(source, dir)
				if err != nil {
					r.Warning("share aborted: %v", err)
				}
				renderShareResults(r, results)
			}
		}

		r.Info("activate with: %s", r.Bold("ccs use "+name))
		return nil
	},
}

// requireExistingDir refuses to register a path that does not yet exist or
// points at a non-directory: this command is for *importing* existing
// profiles, so a missing path almost certainly means the user typed the
// wrong thing rather than wanting us to create one (that's what `add` is
// for).
func requireExistingDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s (use 'ccs add' to create a new profile dir)", dir)
		}
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", dir)
	}
	return nil
}

func init() {
	registerCmd.Flags().BoolVar(&registerShare, "share", false, "symlink shareable items (CLAUDE.md, agents/, commands/, skills/, ...) from ~/.claude")
	rootCmd.AddCommand(registerCmd)
}
