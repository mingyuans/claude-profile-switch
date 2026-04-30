package cmd

import (
	"os"

	"github.com/aftership/ccs-cli/internal/config"
	"github.com/aftership/ccs-cli/internal/output"
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

		liveDir := os.Getenv("CLAUDE_CONFIG_DIR")
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

// profileStatus returns the value rendered in the ACTIVE column.
//
//   - "live"    — p.Dir matches the current shell's $CLAUDE_CONFIG_DIR, i.e.
//     this is the profile actually in effect right now.
//   - "current" — p was the target of the most recent `ccs use`, but the
//     shell environment doesn't currently match (no integration installed,
//     new terminal, manual override, etc.).
//   - ""        — neither.
//
// "live" wins over "current" when both apply, since live shell state is the
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
