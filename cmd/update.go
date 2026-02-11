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
		if err := updater.Replace(currentBinary, newBinary); err != nil {
			if !updater.IsPermissionError(err) {
				return fmt.Errorf("replacing binary: %w", err)
			}
			s.Stop()
			fmt.Fprintln(cmd.OutOrStdout(), "Elevated permissions required. Retrying with sudo...")
			s.Start("Installing with sudo...")
			if err := updater.ReplaceElevated(currentBinary, newBinary); err != nil {
				return fmt.Errorf("replacing binary with sudo: %w", err)
			}
		}

		s.Stop()

		// Verify the new binary works
		out, err := exec.Command(filepath.Clean(currentBinary), "version").Output() //nolint:gosec // path comes from os.Executable
		if err != nil {
			return fmt.Errorf("verifying new binary: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Updated successfully: %s\n", strings.TrimSpace(string(out)))
		return nil
	},
}
