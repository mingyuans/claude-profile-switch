package cmd

import (
	"os"

	"github.com/aftership/ccs-cli/internal/output"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active profile (live $CLAUDE_CONFIG_DIR + last switch)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadStore()
		if err != nil {
			return err
		}
		r := output.Default()

		liveDir := os.Getenv("CLAUDE_CONFIG_DIR")
		if liveDir == "" {
			r.Warning("CLAUDE_CONFIG_DIR is not set in this shell")
		} else {
			r.Info("CLAUDE_CONFIG_DIR=%s", r.Dim(liveDir))
		}
		if store.Current == "" {
			r.Skip("no profile recorded as last `ccs use`")
		} else {
			r.Info("last switched profile: %s", r.Cyan(store.Current))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(currentCmd)
}
