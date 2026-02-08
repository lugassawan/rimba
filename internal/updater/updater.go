package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
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

	req, err := http.NewRequest("GET", url, nil)
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
	req, err := http.NewRequest("GET", url, nil)
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
		os.RemoveAll(tmpDir)
		return "", err
	}

	return dst, nil
}

// extractBinary decompresses a tar.gz stream and extracts the rimba binary to dstDir.
func extractBinary(r io.Reader, dstDir string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("decompressing archive: %w", err)
	}
	defer gz.Close()

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
func writeBinary(dst string, r io.Reader) (string, error) {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return "", fmt.Errorf("creating binary file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("extracting binary: %w", err)
	}

	return dst, nil
}

// Replace swaps the current binary with a new one. It first attempts an atomic
// rename, falling back to a copy for cross-filesystem moves.
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

	// Try atomic rename first
	if err := os.Rename(newBinary, resolved); err != nil {
		// Fallback: copy for cross-filesystem moves
		return copyFile(newBinary, resolved, info.Mode())
	}

	return nil
}

// CleanupTempDir removes the parent directory of the given binary path.
func CleanupTempDir(binaryPath string) {
	os.RemoveAll(filepath.Dir(binaryPath))
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening new binary: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("opening destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}

	return nil
}
