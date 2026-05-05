package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mingyuans/claude-profile-switch/internal/config"
	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/mingyuans/claude-profile-switch/internal/rcfile"
	"github.com/spf13/cobra"
)

var (
	useExport bool
	useNoRc   bool
)

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

		rcPath, rcErr := persistExportToRc(dir)

		if useExport {
			// Stdout is consumed by `eval`. Emit ONLY the export line — no
			// banners, no ANSI, no trailing chatter — and route any human
			// hint to stderr where shells ignore it.
			fmt.Fprintf(os.Stdout, "export CLAUDE_CONFIG_DIR=%s\n", shellQuote(dir))
			fmt.Fprintf(os.Stderr, "  %s switched to %q (%s)\n", output.IconSuccess, name, dir)
			reportRcOutcome(os.Stderr, rcPath, rcErr)
			return nil
		}

		r := output.Default()
		r.Success("switched to %s", r.Cyan(name))
		r.Info("CLAUDE_CONFIG_DIR -> %s", r.Dim(dir))
		switch {
		case rcErr != nil:
			r.Warning("rc file not updated: %v", rcErr)
		case rcPath != "":
			r.Info("persisted to %s (managed block)", r.Dim(rcPath))
		}
		r.Warning("this shell still has the old value; install shell integration once:")
		r.Plain("    %s\n", r.Bold(`eval "$(ccs-cli init zsh)"`))
		return nil
	},
}

// persistExportToRc writes (or rewrites) the managed export block in the
// user's shell rc file so a fresh terminal automatically picks up the new
// CLAUDE_CONFIG_DIR. Returns the rc path that was updated and any error.
//
// Returns ("", nil) when the user opted out via --no-rc or
// CCS_NO_RC=1, so callers can distinguish "skipped on purpose" from
// "tried but failed".
func persistExportToRc(dir string) (string, error) {
	if useNoRc || os.Getenv("CCS_NO_RC") == "1" {
		return "", nil
	}
	shellName, err := resolveShellName()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	rcPath, err := rcfile.Path(shellName, home)
	if err != nil {
		return "", err
	}
	line, err := rcfile.ExportLine(shellName, dir)
	if err != nil {
		return "", err
	}
	if err := rcfile.Update(rcPath, line); err != nil {
		return rcPath, err
	}
	return rcPath, nil
}

// resolveShellName picks the shell to target for rc file persistence.
// CCS_SHELL wins (useful for tests / non-interactive scripts), otherwise
// $SHELL is parsed.
func resolveShellName() (string, error) {
	if override := os.Getenv("CCS_SHELL"); override != "" {
		return rcfile.DetectShell(override)
	}
	return rcfile.DetectShell(os.Getenv("SHELL"))
}

// reportRcOutcome writes a one-line stderr hint describing what happened
// to the rc file, in the --export path where stdout is reserved for eval.
func reportRcOutcome(w *os.File, path string, err error) {
	switch {
	case errors.Is(err, rcfile.ErrUnsupportedShell):
		fmt.Fprintf(w, "  %s rc file not updated (unsupported shell; set CCS_SHELL=zsh|bash|fish)\n", output.IconSkip)
	case err != nil:
		fmt.Fprintf(w, "  %s rc file not updated: %v\n", output.IconWarning, err)
	case path != "":
		fmt.Fprintf(w, "  %s persisted to %s\n", output.IconSuccess, path)
	}
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes
// using the standard '\” trick. Profile dirs typically contain none of
// these, but the quoting keeps the output safe under `eval`.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func init() {
	useCmd.Flags().BoolVar(&useExport, "export", false, "print 'export CLAUDE_CONFIG_DIR=...' for `eval`")
	useCmd.Flags().BoolVar(&useNoRc, "no-rc", false, "do not write the export to your shell rc file")
	rootCmd.AddCommand(useCmd)
}
