package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultAPIEndpoint = "https://api.github.com"
	repoPath           = "/repos/lugassawan/rimba/releases/latest"
	binaryName         = "rimba"
	localBinSubdir     = ".local"
	checksumsFileName  = "checksums.txt"
	goosWindows        = "windows"
)

// allowedAssetHosts pins release-asset downloads to GitHub-controlled hosts.
// The GitHub API's browser_download_url is always https://github.com/...; the
// CDN redirect to objects.githubusercontent.com is followed at fetch time and
// never appears in the API response. objects.githubusercontent.com is included
// defensively for any future direct CDN URLs that GitHub may introduce.
var allowedAssetHosts = map[string]bool{
	"github.com":                    true,
	"objects.githubusercontent.com": true,
}

// maxBinarySize is the decompressed size limit for the extracted binary (100 MiB).
// Tests may override this to exercise the size-limit path.
var maxBinarySize int64 = 100 << 20

// maxArchiveSize is the download size limit for the archive (100 MiB).
// Tests may override this to exercise the size-limit path.
var maxArchiveSize int64 = 100 << 20

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
	AssetName      string
	ChecksumsURL   string
}

// DownloadResult holds the paths and digest produced by Download.
type DownloadResult struct {
	ArchivePath string
	BinaryPath  string
	SHA256      string
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
		Client:         &http.Client{Timeout: 30 * time.Second},
		APIEndpoint:    defaultAPIEndpoint,
	}
}

// IsDevVersion returns true if the version string indicates a development build.
func IsDevVersion(v string) bool {
	return v == "" || v == "dev"
}

// Check queries the GitHub API for the latest release and compares versions.
// It is fail-closed: if checksums.txt is absent from the release assets, it
// returns an error rather than allowing an unverified download.
func (u *Updater) Check(ctx context.Context) (*CheckResult, error) {
	url := u.APIEndpoint + repoPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		assetName := assetNameFor(u.GOOS, u.GOARCH, latest)
		downloadURL, checksumsURL := findReleaseAssets(assetName, release.Assets)
		if downloadURL == "" {
			return nil, fmt.Errorf("no matching asset for %s/%s in release %s", u.GOOS, u.GOARCH, release.TagName)
		}
		if checksumsURL == "" {
			return nil, fmt.Errorf("%s not found in release %s", checksumsFileName, release.TagName)
		}
		if err := validateAssetURL(downloadURL); err != nil {
			return nil, fmt.Errorf("untrusted download URL: %w", err)
		}
		if err := validateAssetURL(checksumsURL); err != nil {
			return nil, fmt.Errorf("untrusted checksums URL: %w", err)
		}
		result.DownloadURL = downloadURL
		result.AssetName = assetName
		result.ChecksumsURL = checksumsURL
	}

	return result, nil
}

// Download fetches the release archive from url, streams it to a temp file while
// computing its SHA-256, then extracts the platform binary. Returns a DownloadResult
// with paths to both the archive and the extracted binary.
func (u *Updater) Download(ctx context.Context, url string) (*DownloadResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "rimba-update-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	isWindows := u.GOOS == goosWindows
	archiveName := "archive.tar.gz"
	if isWindows {
		archiveName = "archive.zip"
	}
	archivePath := filepath.Join(tmpDir, archiveName)

	archiveFile, err := os.Create(archivePath) //nolint:gosec // path is under controlled temp dir
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("creating archive file: %w", err)
	}

	h := sha256.New()
	mw := io.MultiWriter(archiveFile, h)

	lr := io.LimitReader(resp.Body, maxArchiveSize+1)
	n, copyErr := io.Copy(mw, lr)
	_ = archiveFile.Close()

	if copyErr != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("downloading archive: %w", copyErr)
	}
	if n > maxArchiveSize {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("archive exceeds max allowed size of %d bytes", maxArchiveSize)
	}

	binaryPath, err := dispatchExtract(archivePath, tmpDir, isWindows)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	return &DownloadResult{
		ArchivePath: archivePath,
		BinaryPath:  binaryPath,
		SHA256:      hex.EncodeToString(h.Sum(nil)),
	}, nil
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

	// Install the new binary in place: atomic os.Rename on Unix, rename-aside on Windows.
	if err := swapBinary(tmpPath, resolved); err != nil {
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
			line := scanner.Text()
			if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, dir) && strings.Contains(line, "PATH") {
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

// assetNameFor builds the release archive filename for the given platform and version.
func assetNameFor(goos, goarch, version string) string {
	ext := ".tar.gz"
	if goos == goosWindows {
		ext = ".zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s%s", binaryName, version, goos, goarch, ext)
}

// findReleaseAssets scans release assets for the platform archive and checksums file.
// Returns empty strings for any asset not found.
// Returned URLs are validated by Check via validateAssetURL before use (issue #281).
func findReleaseAssets(assetName string, assets []Asset) (downloadURL, checksumsURL string) {
	for _, a := range assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
		}
		if a.Name == checksumsFileName {
			checksumsURL = a.BrowserDownloadURL
		}
	}
	return
}

// validateAssetURL rejects asset URLs that are not https or whose host is not a
// trusted GitHub host, defending against a tampered API response that redirects
// the binary and checksums.txt to an attacker-controlled server (issue #281).
func validateAssetURL(rawURL string) error {
	// Defensive: Check() pre-screens for empty, but callers outside Check may not.
	if rawURL == "" {
		return errors.New("asset URL must not be empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing asset URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("asset URL %q must use https, got scheme %q", rawURL, u.Scheme)
	}
	if !allowedAssetHosts[u.Hostname()] {
		return fmt.Errorf("asset URL %q host %q is not a trusted GitHub host", rawURL, u.Hostname())
	}
	return nil
}
