package updater

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultAPIEndpoint = "https://api.github.com"
	repoPath           = "/repos/lugassawan/rimba/releases/latest"
	binaryName         = "rimba"
	localBinSubdir     = ".local"
)

// HTTPClient abstracts HTTP requests for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Release represents a GitHub release API response.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset from the GitHub API.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckResult holds the result of a version check.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpToDate       bool
	DownloadURL    string
}

// Updater checks for and applies updates from GitHub releases.
type Updater struct {
	CurrentVersion string
	GOOS           string
	GOARCH         string
	Client         HTTPClient
	APIEndpoint    string
}

// New creates an Updater with production defaults.
func New(currentVersion string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Client:         &http.Client{},
		APIEndpoint:    defaultAPIEndpoint,
	}
}

// IsDevVersion returns true if the version string indicates a development build.
func IsDevVersion(v string) bool {
	return v == "" || v == "dev"
}

// Check queries the GitHub API for the latest release and compares versions.
func (u *Updater) Check() (*CheckResult, error) {
	url := u.APIEndpoint + repoPath

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(u.CurrentVersion, "v")

	result := &CheckResult{
		CurrentVersion: u.CurrentVersion,
		LatestVersion:  release.TagName,
		UpToDate:       current == latest,
	}

	if !result.UpToDate {
		assetName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, latest, u.GOOS, u.GOARCH)
		for _, a := range release.Assets {
			if a.Name == assetName {
				result.DownloadURL = a.BrowserDownloadURL
				break
			}
		}
		if result.DownloadURL == "" {
			return nil, fmt.Errorf("no matching asset for %s/%s in release %s", u.GOOS, u.GOARCH, release.TagName)
		}
	}

	return result, nil
}

// Download fetches a tar.gz archive from the given URL and extracts the binary
// to a temporary directory. Returns the path to the extracted binary.
func (u *Updater) Download(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating download request: %w", err)
	}

	resp, err := u.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "rimba-update-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	dst, err := extractBinary(resp.Body, tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}

	return dst, nil
}

// Replace atomically swaps the current binary with a new one via
// temp-file-then-rename in the destination directory.
func Replace(currentBinary, newBinary string) error {
	// Resolve symlinks to get the real path
	resolved, err := filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	// Preserve permissions from the original binary
	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}

	// Create temp file in the same directory as the destination.
	// Same filesystem guarantees os.Rename will succeed.
	dir := filepath.Dir(resolved)
	tmp, err := os.CreateTemp(dir, ".rimba-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up temp file on any error (no-op if rename succeeded)
		_ = os.Remove(tmpPath)
	}()

	// Copy new binary content into the temp file
	src, err := os.Open(filepath.Clean(newBinary))
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("opening new binary: %w", err)
	}
	if _, err := io.Copy(tmp, src); err != nil {
		_ = src.Close()
		_ = tmp.Close()
		return fmt.Errorf("copying binary: %w", err)
	}
	_ = src.Close()

	// Set permissions from original binary
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename: new inode replaces old one
	if err := os.Rename(tmpPath, resolved); err != nil { //nolint:gosec // resolved is the current binary path, not user input
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// CleanupTempDir removes the parent directory of the given binary path.
func CleanupTempDir(binaryPath string) {
	_ = os.RemoveAll(filepath.Dir(binaryPath))
}

// IsPermissionError returns true if the error chain contains os.ErrPermission.
func IsPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission)
}

// UserInstallDir returns ~/.local/bin as the user-writable install directory.
func UserInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, localBinSubdir, "bin"), nil
}

// EnsurePath adds dir to the user's shell PATH config if not already present.
// Detects shell from $SHELL, appends export with guard comment, idempotent.
func EnsurePath(dir string) error {
	shell := filepath.Base(os.Getenv("SHELL"))
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	var rcFile string
	switch shell {
	case "zsh":
		rcFile = filepath.Join(home, ".zshrc")
	case "bash":
		rcFile = filepath.Join(home, ".bashrc")
	default:
		// Unsupported shell — skip silently
		return nil
	}

	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
	guard := "# Added by rimba"

	// Check if the export line already exists
	if f, err := os.Open(rcFile); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), dir) && strings.Contains(scanner.Text(), "PATH") {
				_ = f.Close()
				return nil // already configured
			}
		}
		scanErr := scanner.Err()
		_ = f.Close()
		if scanErr != nil {
			return fmt.Errorf("reading %s: %w", rcFile, scanErr)
		}
	}

	// Append export line with guard comment
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gosec // user shell config
	if err != nil {
		return fmt.Errorf("opening %s: %w", rcFile, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintf(f, "\n%s\n%s\n", guard, exportLine); err != nil {
		return fmt.Errorf("writing to %s: %w", rcFile, err)
	}

	return nil
}

// extractBinary decompresses a tar.gz stream and extracts the rimba binary to dstDir.
func extractBinary(r io.Reader, dstDir string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("decompressing archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading archive: %w", err)
		}

		if hdr.Name == binaryName {
			return writeBinary(filepath.Join(dstDir, binaryName), tr)
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

// writeBinary writes the tar entry contents to dst with executable permissions.
func writeBinary(dst string, r io.Reader) (_ string, retErr error) {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755) //nolint:gosec // executable binary requires 0755
	if err != nil {
		return "", fmt.Errorf("creating binary file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}

	return dst, nil
}
