package cmd

import (
	"os"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/mingyuans/claude-profile-switch/internal/rcfile"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List registered profiles",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadStore()
		if err != nil {
			return err
		}
		r := output.Default()

		profiles := store.List()
		if len(profiles) == 0 {
			r.Info("No profiles registered yet. Try: ccs add work")
			return nil
		}

		liveDir := resolveLiveDir()
		rows := make([][]string, 0, len(profiles))
		for _, p := range profiles {
			status := profileStatus(p, store.Current, liveDir)
			rows = append(rows, []string{
				r.Cyan(p.Name),
				paintStatus(r, status),
				r.Dim(p.Dir),
			})
		}
		r.Table([]string{"NAME", "ACTIVE", "PATH"}, rows)
		return nil
	},
}

// resolveLiveDir reports the CLAUDE_CONFIG_DIR a *fresh* shell would see —
// not whatever this shell happens to have exported. We read it from the
// managed block in the user's rc file so `ccs list` matches what `claude`
// would actually pick up when launched from a brand-new terminal.
//
// Returns "" if the rc file or block is missing, or the shell is not
// auto-detectable: in those cases no profile is considered live, which
// is the correct signal — nothing has been persisted.
func resolveLiveDir() string {
	shellName, err := resolveShellName()
	if err != nil {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	rcPath, err := rcfile.Path(shellName, home)
	if err != nil {
		return ""
	}
	dir, ok, err := rcfile.ReadExportedDir(rcPath)
	if err != nil || !ok {
		return ""
	}
	return dir
}

// profileStatus returns the value rendered in the ACTIVE column.
//
//   - "live"    — p.Dir matches the CLAUDE_CONFIG_DIR persisted in the
//     user's rc file; a freshly opened shell will run on this profile.
//   - "current" — p was the target of the most recent `ccs use`, but the
//     rc file doesn't currently encode it (no integration installed yet,
//     --no-rc was used, manual edit, etc.).
//   - ""        — neither.
//
// "live" wins over "current" when both apply, since the rc file is the
// stronger signal.
func profileStatus(p config.Profile, currentName, liveDir string) string {
	if liveDir != "" {
		if expanded, err := config.ExpandDir(p.Dir); err == nil && expanded == liveDir {
			return "live"
		}
	}
	if p.Name == currentName {
		return "current"
	}
	return ""
}

// paintStatus colours the ACTIVE-column value: "live" stands out in green
// (matches the Success icon family) and "current" in yellow (matches the
// Warning family — same "heads-up, not the live one" hue used elsewhere).
// An empty status stays empty so blank cells don't pick up stray escapes.
func paintStatus(r *output.Renderer, status string) string {
	switch status {
	case "live":
		return r.Green(r.Bold(status))
	case "current":
		return r.Yellow(status)
	default:
		return status
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
