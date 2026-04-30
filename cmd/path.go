package cmd

import (
	"github.com/aftership/ccs-cli/internal/config"
	"github.com/aftership/ccs-cli/internal/output"
	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:   "path <name>",
	Short: "Print the directory path of a profile (script-friendly, no decoration)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadStore()
		if err != nil {
			return err
		}
		profile, err := store.Get(args[0])
		if err != nil {
			return err
		}
		expanded, err := config.ExpandDir(profile.Dir)
		if err != nil {
			return err
		}
		output.Default().Println("%s", expanded)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pathCmd)
}
