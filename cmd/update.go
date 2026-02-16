package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().Bool("force", false, "update even if running a development build")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:         "update",
	Short:       "Update rimba to the latest version",
	Long:        "Check for the latest release on GitHub and update the binary in place.",
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if updater.IsDevVersion(version) && !force {
			fmt.Fprintln(cmd.OutOrStdout(), "You are running a development build of rimba.")
			fmt.Fprintln(cmd.OutOrStdout(), "Use --force to update anyway.")
			return nil
		}

		u := updater.New(version)

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Checking for updates...")
		result, err := u.Check()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		if result.UpToDate {
			s.Stop()
			fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s).\n", result.CurrentVersion)
			return nil
		}

		s.Stop()
		fmt.Fprintf(cmd.OutOrStdout(), "New version available: %s → %s\n", result.CurrentVersion, result.LatestVersion)

		s.Start("Downloading...")
		newBinary, err := u.Download(result.DownloadURL)
		if err != nil {
			return fmt.Errorf("downloading update: %w", err)
		}
		defer updater.CleanupTempDir(newBinary)

		if err := updater.PrepareBinary(newBinary); err != nil {
			return fmt.Errorf("preparing binary: %w", err)
		}

		currentBinary, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locating current binary: %w", err)
		}
		currentBinary, err = filepath.EvalSymlinks(currentBinary)
		if err != nil {
			return fmt.Errorf("resolving binary path: %w", err)
		}

		s.Update("Installing...")
		installedBinary := currentBinary
		if err := updater.Replace(currentBinary, newBinary); err != nil {
			if !updater.IsPermissionError(err) {
				return fmt.Errorf("replacing binary: %w", err)
			}

			// Install to user-writable directory instead
			s.Stop()
			userDir, dirErr := updater.UserInstallDir()
			if dirErr != nil {
				return fmt.Errorf("getting user install dir: %w", dirErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cannot write to %s. Installing to %s instead.\n",
				filepath.Dir(currentBinary), userDir)

			if mkErr := os.MkdirAll(userDir, 0750); mkErr != nil {
				return fmt.Errorf("creating install dir: %w", mkErr)
			}

			installedBinary = filepath.Join(userDir, "rimba")
			s.Start("Installing...")
			if _, statErr := os.Stat(installedBinary); os.IsNotExist(statErr) {
				// First install to this directory — write directly
				src, readErr := os.ReadFile(newBinary)
				if readErr != nil {
					return fmt.Errorf("reading new binary: %w", readErr)
				}
				if writeErr := os.WriteFile(installedBinary, src, 0755); writeErr != nil { //nolint:gosec // executable binary
					return fmt.Errorf("writing binary: %w", writeErr)
				}
			} else if err := updater.Replace(installedBinary, newBinary); err != nil {
				return fmt.Errorf("replacing binary: %w", err)
			}

			if pathErr := updater.EnsurePath(userDir); pathErr != nil {
				return fmt.Errorf("updating PATH: %w", pathErr)
			}
		}

		s.Stop()

		// Verify the new binary works
		out, err := exec.Command(filepath.Clean(installedBinary), "version").Output() //nolint:gosec // path comes from os.Executable
		if err != nil {
			return fmt.Errorf("verifying new binary: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Updated successfully: %s\n", strings.TrimSpace(string(out)))

		// Print migration guidance if installed to a different location
		if installedBinary != currentBinary {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTo complete migration, remove the old binary:\n  sudo rm %s\n", currentBinary)
		}
		return nil
	},
}
