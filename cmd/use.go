package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/aftership/ccs-cli/internal/config"
	"github.com/aftership/ccs-cli/internal/output"
	"github.com/spf13/cobra"
)

var useExport bool

var useCmd = &cobra.Command{
	Use:     "use <name>",
	Aliases: []string{"switch"},
	Short:   "Switch to a profile (live in current shell when sourced via `ccs init`)",
	Long: `Activate a profile by setting CLAUDE_CONFIG_DIR.

Without --export this prints a human-readable confirmation and persists the
profile name as 'current' in the on-disk store. To make the change take
effect in the current shell, install the shell integration once:

  eval "$(ccs-cli init zsh)"

After that, the wrapper function 'ccs' transparently calls the binary with
--export and evals the resulting 'export CLAUDE_CONFIG_DIR=...' line.

Use --export directly only if you are scripting:

  eval "$(ccs-cli use work --export)"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := loadStore()
		if err != nil {
			return err
		}
		profile, err := store.Get(name)
		if err != nil {
			return err
		}
		dir, err := config.ExpandDir(profile.Dir)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("ensure dir %s: %w", dir, err)
		}

		if err := store.SetCurrent(name); err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}

		if useExport {
			// Stdout is consumed by `eval`. Emit ONLY the export line — no
			// banners, no ANSI, no trailing chatter — and route any human
			// hint to stderr where shells ignore it.
			fmt.Fprintf(os.Stdout, "export CLAUDE_CONFIG_DIR=%s\n", shellQuote(dir))
			fmt.Fprintf(os.Stderr, "  %s switched to %q (%s)\n", output.IconSuccess, name, dir)
			return nil
		}

		r := output.Default()
		r.Success("switched to %s", r.Cyan(name))
		r.Info("CLAUDE_CONFIG_DIR -> %s", r.Dim(dir))
		r.Warning("this shell still has the old value; install shell integration:")
		r.Plain("    %s\n", r.Bold(`eval "$(ccs-cli init zsh)"`))
		return nil
	},
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes
// using the standard '\” trick. Profile dirs typically contain none of
// these, but the quoting keeps the output safe under `eval`.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func init() {
	useCmd.Flags().BoolVar(&useExport, "export", false, "print 'export CLAUDE_CONFIG_DIR=...' for `eval`")
	rootCmd.AddCommand(useCmd)
}
