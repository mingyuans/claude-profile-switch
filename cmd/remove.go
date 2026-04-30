package cmd

import (
	"fmt"
	"os"

	"github.com/aftership/ccs-cli/internal/config"
	"github.com/aftership/ccs-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	rmPurge bool
	rmYes   bool
)

var removeCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove", "delete"},
	Short:   "Unregister a profile (use --purge to also delete its directory)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := loadStore()
		if err != nil {
			return err
		}
		r := output.Default()

		removed, err := store.Remove(name)
		if err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		r.Success("unregistered %s", r.Cyan(name))

		if !rmPurge {
			r.Info("directory left intact: %s", r.Dim(removed.Dir))
			return nil
		}
		dir, err := config.ExpandDir(removed.Dir)
		if err != nil {
			return err
		}
		if !rmYes {
			return fmt.Errorf("refusing to delete %s without --yes (rerun with --purge --yes)", dir)
		}
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove %s: %w", dir, err)
		}
		r.Success("deleted directory: %s", r.Dim(dir))
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&rmPurge, "purge", false, "also delete the profile directory from disk")
	removeCmd.Flags().BoolVar(&rmYes, "yes", false, "confirm destructive --purge (required)")
	rootCmd.AddCommand(removeCmd)
}
