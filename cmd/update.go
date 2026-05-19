package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

const retryUpdateHint = "retry: rimba update"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rimba to the latest version",
	Example: `  rimba update
  rimba update --force`,
	Long: `Check for the latest release on GitHub and update the binary in place.

If the current binary cannot be replaced due to file permissions, rimba installs the
new version to ~/.local/bin instead and prints the path.

After a successful update, rimba prints a one-line tip if agent files are installed at
user level (run rimba init -g to refresh) or in this repo (run rimba init --agents to
refresh). Set RIMBA_QUIET=1 to suppress the tip.`,
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
			return errhint.WithFix(
				fmt.Errorf("checking for updates: %w", err),
				"check network connectivity, or set GITHUB_TOKEN if rate limited",
			)
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
			return errhint.WithFix(
				fmt.Errorf("downloading update: %w", err),
				"check network connectivity and retry: rimba update",
			)
		}
		defer updater.CleanupTempDir(newBinary)

		if err := updater.PrepareBinary(newBinary); err != nil {
			return errhint.WithFix(fmt.Errorf("preparing binary: %w", err), retryUpdateHint)
		}

		currentBinary, err := os.Executable()
		if err != nil {
			return errhint.WithFix(
				fmt.Errorf("locating current binary: %w", err),
				"reinstall rimba: https://github.com/lugassawan/rimba#installation",
			)
		}
		currentBinary, err = filepath.EvalSymlinks(currentBinary)
		if err != nil {
			return errhint.WithFix(
				fmt.Errorf("resolving binary path: %w", err),
				"reinstall rimba: https://github.com/lugassawan/rimba#installation",
			)
		}

		s.Update("Installing...")
		installedBinary := currentBinary
		if err := updater.Replace(currentBinary, newBinary); err != nil {
			if !updater.IsPermissionError(err) {
				return errhint.WithFix(fmt.Errorf("replacing binary: %w", err), retryUpdateHint)
			}

			// Install to user-writable directory instead
			s.Stop()
			userDir, dirErr := updater.UserInstallDir()
			if dirErr != nil {
				return errhint.WithFix(
					fmt.Errorf("getting user install dir: %w", dirErr),
					"set HOME to your user home directory and retry: rimba update",
				)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cannot write to %s. Installing to %s instead.\n",
				filepath.Dir(currentBinary), userDir)

			if mkErr := os.MkdirAll(userDir, 0750); mkErr != nil {
				return errhint.WithFix(
					fmt.Errorf("creating install dir: %w", mkErr),
					"check write permissions for ~/.local/bin and retry: rimba update",
				)
			}

			installedBinary = filepath.Join(userDir, "rimba")
			s.Start("Installing...")
			if _, statErr := os.Stat(installedBinary); os.IsNotExist(statErr) {
				// First install to this directory — write directly
				src, readErr := os.ReadFile(newBinary)
				if readErr != nil {
					return errhint.WithFix(fmt.Errorf("reading new binary: %w", readErr), retryUpdateHint)
				}
				if writeErr := os.WriteFile(installedBinary, src, 0755); writeErr != nil { //nolint:gosec // executable binary
					return errhint.WithFix(
						fmt.Errorf("writing binary: %w", writeErr),
						fmt.Sprintf("check write permissions for %s and retry: rimba update", userDir),
					)
				}
			} else if err := updater.Replace(installedBinary, newBinary); err != nil {
				return errhint.WithFix(
					fmt.Errorf("replacing binary: %w", err),
					fmt.Sprintf("check write permissions for %s and retry: rimba update", userDir),
				)
			}

			if pathErr := updater.EnsurePath(userDir); pathErr != nil {
				return errhint.WithFix(
					fmt.Errorf("updating PATH: %w", pathErr),
					fmt.Sprintf("add %s to PATH manually: export PATH=\"%s:$PATH\"", userDir, userDir),
				)
			}
		}

		s.Stop()

		// Verify the new binary works
		out, err := exec.Command(filepath.Clean(installedBinary), "version").Output() //nolint:gosec // path comes from os.Executable
		if err != nil {
			return errhint.WithFix(
				fmt.Errorf("verifying new binary: %w", err),
				fmt.Sprintf("the new binary at %s may be corrupt — retry: rimba update", installedBinary),
			)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Updated successfully: %s\n", strings.TrimSpace(string(out)))

		home, repoRoot := resolvePostUpdateTipPaths()
		printAgentRefreshTips(cmd, home, repoRoot)

		// Print migration guidance if installed to a different location
		if installedBinary != currentBinary {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTo complete migration, remove the old binary:\n  sudo rm %s\n", currentBinary)
		}
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("force", false, "update even if running a development build")
	rootCmd.AddCommand(updateCmd)
}
