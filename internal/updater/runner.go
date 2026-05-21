package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/spinner"
)

const (
	retryUpdateHint = "retry: rimba update"
	reinstallHint   = "reinstall rimba: https://github.com/lugassawan/rimba#installation"
)

// Runner orchestrates the self-update flow with injectable seams for testability.
// Each unexported field maps to one stage of the update pipeline; NewRunner wires
// production defaults. ctx is checked once before the destructive install stage —
// os.Rename in Replace is atomic so a check inside Replace is pointless.
// Post-Replace pre-verify crash: a SIGINT/crash after Replace but before verify
// leaves a working new binary; recovery is `rimba version`.
type Runner struct {
	Version   string
	Out       io.Writer
	Spinner   *spinner.Spinner
	OnSuccess func()

	check          func() (*CheckResult, error)
	download       func(url string) (string, error)
	prepareBinary  func(path string) error
	executable     func() (string, error)
	evalSymlinks   func(path string) (string, error)
	replace        func(currentBinary, newBinary string) error
	userInstallDir func() (string, error)
	mkdirAll       func(path string, perm os.FileMode) error
	stat           func(path string) (os.FileInfo, error)
	readFile       func(path string) ([]byte, error)
	writeFile      func(path string, data []byte, perm os.FileMode) error
	ensurePath     func(dir string) error
	execCommand    func(ctx context.Context, name string, args ...string) ([]byte, error)
	cleanupTempDir func(binaryPath string)
}

// NewRunner creates a Runner with production defaults wired.
func NewRunner(version string) *Runner {
	u := New(version)
	r := &Runner{
		Version: version,
		Out:     os.Stdout,
		Spinner: spinner.New(spinner.Options{Writer: io.Discard}),
	}
	r.check = u.Check
	r.download = u.Download
	r.prepareBinary = PrepareBinary
	r.executable = os.Executable
	r.evalSymlinks = filepath.EvalSymlinks
	r.replace = Replace
	r.userInstallDir = UserInstallDir
	r.mkdirAll = os.MkdirAll
	r.stat = os.Stat
	r.readFile = os.ReadFile
	r.writeFile = os.WriteFile
	r.ensurePath = EnsurePath
	r.execCommand = defaultExecCommand
	r.cleanupTempDir = CleanupTempDir
	return r
}

func defaultExecCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, filepath.Clean(name), args...).Output() //nolint:gosec // path comes from os.Executable
}

// Run executes the full update pipeline: check → download → prepare → locate →
// install → verify. ctx is checked before the destructive install stage.
func (r *Runner) Run(ctx context.Context) error {
	r.Spinner.Start("Checking for updates...")
	result, err := r.check()
	if err != nil {
		return errhint.WithFix(
			fmt.Errorf("checking for updates: %w", err),
			"check network connectivity, or set GITHUB_TOKEN if rate limited",
		)
	}

	if result.UpToDate {
		r.Spinner.Stop()
		fmt.Fprintf(r.Out, "Already up to date (%s).\n", result.CurrentVersion)
		return nil
	}

	r.Spinner.Stop()
	fmt.Fprintf(r.Out, "New version available: %s → %s\n", result.CurrentVersion, result.LatestVersion)

	r.Spinner.Start("Downloading...")
	newBinary, err := r.download(result.DownloadURL)
	if err != nil {
		return errhint.WithFix(
			fmt.Errorf("downloading update: %w", err),
			"check network connectivity and retry: rimba update",
		)
	}
	defer r.cleanupTempDir(newBinary)

	if err := r.prepareBinary(newBinary); err != nil {
		return errhint.WithFix(fmt.Errorf("preparing binary: %w", err), retryUpdateHint)
	}

	currentBinary, err := r.locateCurrentBinary()
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	r.Spinner.Update("Installing...")
	installedBinary, err := r.install(currentBinary, newBinary)
	if err != nil {
		return err
	}

	r.Spinner.Stop()

	versionStr, err := r.verify(ctx, installedBinary)
	if err != nil {
		return err
	}

	fmt.Fprintf(r.Out, "Updated successfully: %s\n", versionStr)

	if r.OnSuccess != nil {
		r.OnSuccess()
	}

	if installedBinary != currentBinary {
		fmt.Fprintf(r.Out, "\nTo complete migration, remove the old binary:\n  sudo rm %s\n", currentBinary)
	}

	return nil
}

func (r *Runner) locateCurrentBinary() (string, error) {
	current, err := r.executable()
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("locating current binary: %w", err),
			reinstallHint,
		)
	}
	resolved, err := r.evalSymlinks(current)
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("resolving binary path: %w", err),
			reinstallHint,
		)
	}
	return resolved, nil
}

func (r *Runner) install(currentBinary, newBinary string) (string, error) {
	if err := r.replace(currentBinary, newBinary); err != nil {
		if !IsPermissionError(err) {
			return "", errhint.WithFix(fmt.Errorf("replacing binary: %w", err), retryUpdateHint)
		}
		return r.installToUserDir(currentBinary, newBinary)
	}
	return currentBinary, nil
}

func (r *Runner) installToUserDir(currentBinary, newBinary string) (string, error) {
	r.Spinner.Stop()
	userDir, err := r.userInstallDir()
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("getting user install dir: %w", err),
			"set HOME to your user home directory and retry: rimba update",
		)
	}
	fmt.Fprintf(r.Out, "Cannot write to %s. Installing to %s instead.\n",
		filepath.Dir(currentBinary), userDir)

	if err := r.mkdirAll(userDir, 0750); err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("creating install dir: %w", err),
			"check write permissions for ~/.local/bin and retry: rimba update",
		)
	}

	installedBinary := filepath.Join(userDir, "rimba")
	r.Spinner.Start("Installing...")

	if err := r.installBinaryAt(installedBinary, newBinary, userDir); err != nil {
		return "", err
	}

	if err := r.ensurePath(userDir); err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("updating PATH: %w", err),
			fmt.Sprintf("add %s to PATH manually: export PATH=\"%s:$PATH\"", userDir, userDir),
		)
	}

	return installedBinary, nil
}

func (r *Runner) installBinaryAt(installedBinary, newBinary, userDir string) error {
	if _, err := r.stat(installedBinary); os.IsNotExist(err) {
		src, readErr := r.readFile(newBinary)
		if readErr != nil {
			return errhint.WithFix(fmt.Errorf("reading new binary: %w", readErr), retryUpdateHint)
		}
		if writeErr := r.writeFile(installedBinary, src, 0755); writeErr != nil { //nolint:gosec // executable binary
			return errhint.WithFix(
				fmt.Errorf("writing binary: %w", writeErr),
				fmt.Sprintf("check write permissions for %s and retry: rimba update", userDir),
			)
		}
		return nil
	}
	if err := r.replace(installedBinary, newBinary); err != nil {
		return errhint.WithFix(
			fmt.Errorf("replacing binary: %w", err),
			fmt.Sprintf("check write permissions for %s and retry: rimba update", userDir),
		)
	}
	return nil
}

func (r *Runner) verify(ctx context.Context, installedBinary string) (string, error) {
	out, err := r.execCommand(ctx, installedBinary, "version")
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("verifying new binary: %w", err),
			fmt.Sprintf("the new binary at %s may be corrupt — retry: rimba update", installedBinary),
		)
	}
	return strings.TrimSpace(string(out)), nil
}
