package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/mingyuans/claude-profile-switch/internal/output"
	"github.com/mingyuans/claude-profile-switch/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	updateCheck   bool
	updateVersion string
	updateForce   bool
)

// newUpdateClient is swapped in tests to point at an httptest server.
var newUpdateClient = selfupdate.NewClient

// updateExecutable resolves the binary to replace; swapped in tests.
var updateExecutable = selfupdate.ExecutablePath

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Update ccs to the latest release",
	Long: `Download the latest ccs release from GitHub and replace the running binary.

The archive for your platform is verified against the release's checksums.txt
before anything on disk is touched, and the swap is an atomic rename, so an
interrupted update leaves the old binary intact.

  ccs update                  # update to the latest release
  ccs update --check          # only report whether an update is available
  ccs update --version v0.1.0 # install a specific release (also downgrades)

A binary built from source (version "dev" or a non-release git describe) is
not replaced unless you pass --force. Set GITHUB_TOKEN to avoid API rate
limits and CCS_REPO to update from a fork, same as install.sh.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		r := output.Default()
		client := newUpdateClient()

		tag := updateVersion
		if tag == "" {
			latest, err := client.LatestTag()
			if err != nil {
				return err
			}
			tag = latest
		}
		target, ok := selfupdate.ParseSemver(tag)
		if !ok {
			return fmt.Errorf("release tag %q is not a vMAJOR.MINOR.PATCH version", tag)
		}
		current, currentIsRelease := selfupdate.ParseSemver(version)

		r.Info("current: %s", r.Cyan(version))
		r.Info("target:  %s", r.Cyan(tag))

		upToDate := currentIsRelease && current.Compare(target) == 0 ||
			// "latest" never downgrades; an explicit --version may.
			currentIsRelease && updateVersion == "" && current.Compare(target) > 0

		if updateCheck {
			switch {
			case !currentIsRelease:
				r.Warning("running a source build; run 'ccs update --force' to switch to %s", tag)
			case upToDate:
				r.Success("already up to date")
			default:
				r.Info("update available, run 'ccs update'")
			}
			return nil
		}

		if !updateForce {
			if !currentIsRelease {
				return fmt.Errorf("current build %q is not a release build (built from source?); pass --force to replace it with %s", version, tag)
			}
			if upToDate {
				r.Success("already up to date")
				return nil
			}
		}

		exe, err := updateExecutable()
		if err != nil {
			return err
		}
		r.Info("downloading %s", client.AssetName())
		bin, err := client.Download(tag)
		if err != nil {
			return err
		}
		r.Success("checksum verified")
		if err := selfupdate.Replace(exe, bin); err != nil {
			if errors.Is(err, os.ErrPermission) {
				return fmt.Errorf("%w\n  rerun with sudo, or reinstall to a writable dir via install.sh --bin-dir", err)
			}
			return err
		}
		r.Success("updated %s to %s", exe, r.Green(tag))
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "only check whether a newer release exists")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "install this release tag instead of the latest (e.g. v0.1.0)")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "reinstall even if up to date or running a source build")
	rootCmd.AddCommand(updateCmd)
}
