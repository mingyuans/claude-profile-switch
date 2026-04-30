package cmd

import (
	"github.com/aftership/ccs-cli/internal/output"
	"github.com/aftership/ccs-cli/internal/shellfs"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [shell]",
	Short: "Print shell integration script (zsh, bash, fish)",
	Long: `Print a shell snippet that defines a 'ccs' function.

The function forwards most subcommands directly to the binary, but for
'use' / 'switch' it runs the binary with --export and evals the resulting
'export CLAUDE_CONFIG_DIR=...' line, so the active profile takes effect in
the current shell session.

Source it once from your shell rc file, e.g.:

  echo 'eval "$(ccs-cli init zsh)"' >> ~/.zshrc

Supported shells: zsh, bash, fish.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"zsh", "bash", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := shellfs.Script(args[0])
		if err != nil {
			return err
		}
		output.Default().Plain("%s", script)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
