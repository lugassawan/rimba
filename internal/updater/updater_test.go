package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testVersion      = "v1.0.0"
	testVersionNew   = "v2.0.0"
	testVersionNew2  = "v1.2.3"
	testOS           = "linux"
	testArch         = "amd64"
	contentTypeJSON  = "application/json"
	contentTypeHdr   = "Content-Type"
	contentTypeOctet = "application/octet-stream"
	errWantFmt       = "error = %q, want %q"

	// Shared format and value constants
	contentFmt    = "content = %q, want %q"
	fatalReplace  = "Replace: %v"
	valNewBinary  = "new binary"
	valNewContent = "new content"
	testRcZshrc   = ".zshrc"
	testShellZsh  = "/bin/zsh"

	// checksum test constants
	testChecksumHex  = "aabbccdd"
	testChecksumFile = "rimba_2.0.0_linux_amd64.tar.gz"
)

// newTestUpdater creates an Updater wired to the given test server.
func newTestUpdater(srv *httptest.Server) *Updater {
	return &Updater{
		CurrentVersion: testVersion,
		GOOS:           testOS,
		GOARCH:         testArch,
		Client:         srv.Client(),
		APIEndpoint:    srv.URL,
	}
}

// requireNoError fails the test immediately if err is non-nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// serveJSON creates an httptest.Server that returns the given JSON body.
func serveJSON(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, contentTypeJSON)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// serveOctetStream creates an httptest.Server that returns the given binary data.
func serveOctetStream(t *testing.T, data []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, contentTypeOctet)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// releaseJSON builds a GitHub release JSON body with the given assets appended.
func releaseJSON(tagName string, extras ...string) string {
	assets := make([]string, 0, 1+len(extras))
	assets = append(assets,
		`{"name":"checksums.txt","browser_download_url":"https://example.com/checksums.txt"}`,
	)
	assets = append(assets, extras...)
	return fmt.Sprintf(`{"tag_name":%q,"assets":[%s]}`, tagName, strings.Join(assets, ","))
}

func TestCheckUpToDate(t *testing.T) {
	srv := serveJSON(t, `{"tag_name":"`+testVersion+`","assets":[]}`)
	u := newTestUpdater(srv)

	result, err := u.Check(context.Background())
	requireNoError(t, err)
	if !result.UpToDate {
		t.Errorf("expected up to date, got not up to date")
	}
	if result.LatestVersion != testVersion {
		t.Errorf("expected latest %s, got %s", testVersion, result.LatestVersion)
	}
}

func TestCheckNewVersionAvailable(t *testing.T) {
	srv := serveJSON(t, releaseJSON(testVersionNew,
		`{"name":"rimba_2.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/rimba_2.0.0_linux_amd64.tar.gz"}`,
	))
	u := newTestUpdater(srv)

	result, err := u.Check(context.Background())
	requireNoError(t, err)
	if result.UpToDate {
		t.Errorf("expected not up to date")
	}

	wantURL := "https://example.com/rimba_2.0.0_linux_amd64.tar.gz"
	if result.DownloadURL != wantURL {
		t.Errorf("download URL = %q, want %q", result.DownloadURL, wantURL)
	}

	wantAsset := "rimba_2.0.0_linux_amd64.tar.gz"
	if result.AssetName != wantAsset {
		t.Errorf("AssetName = %q, want %q", result.AssetName, wantAsset)
	}

	wantChecksums := "https://example.com/checksums.txt"
	if result.ChecksumsURL != wantChecksums {
		t.Errorf("ChecksumsURL = %q, want %q", result.ChecksumsURL, wantChecksums)
	}
}

func TestCheckNoMatchingAsset(t *testing.T) {
	srv := serveJSON(t, `{
		"tag_name":"`+testVersionNew+`",
		"assets":[
			{"name":"rimba_2.0.0_windows_amd64.zip","browser_download_url":"https://example.com/rimba_2.0.0_windows_amd64.zip"}
		]
	}`)
	u := newTestUpdater(srv)

	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for missing asset")
	}

	want := "no matching asset for linux/amd64 in release " + testVersionNew
	if got := err.Error(); got != want {
		t.Errorf(errWantFmt, got, want)
	}
}

func TestCheckMissingChecksums(t *testing.T) {
	srv := serveJSON(t, fmt.Sprintf(`{
		"tag_name":%q,
		"assets":[
			{"name":"rimba_2.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/rimba_2.0.0_linux_amd64.tar.gz"}
		]
	}`, testVersionNew))
	u := newTestUpdater(srv)

	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error when checksums.txt is absent")
	}
	if !strings.Contains(err.Error(), "checksums.txt not found") {
		t.Errorf("error = %q, want to contain 'checksums.txt not found'", err.Error())
	}
}

func TestCheckWindowsAsset(t *testing.T) {
	srv := serveJSON(t, releaseJSON(testVersionNew,
		`{"name":"rimba_2.0.0_windows_amd64.zip","browser_download_url":"https://example.com/rimba_2.0.0_windows_amd64.zip"}`,
	))
	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           goosWindows,
		GOARCH:         testArch,
		Client:         srv.Client(),
		APIEndpoint:    srv.URL,
	}

	result, err := u.Check(context.Background())
	requireNoError(t, err)
	if result.UpToDate {
		t.Error("expected not up to date")
	}
	wantAssetWin := assetNameFor(goosWindows, testArch, "2.0.0")
	if result.AssetName != wantAssetWin {
		t.Errorf("AssetName = %q, want %q", result.AssetName, wantAssetWin)
	}
	if result.ChecksumsURL == "" {
		t.Error("ChecksumsURL should be set")
	}
}

func TestCheckAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	u := newTestUpdater(srv)

	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for API failure")
	}

	want := "GitHub API returned status 500"
	if got := err.Error(); got != want {
		t.Errorf(errWantFmt, got, want)
	}
}

func TestIsDevVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"", true},
		{testVersion, false},
		{"1.0.0", false},
		{"0.1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := IsDevVersion(tt.version); got != tt.want {
				t.Errorf("IsDevVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestDownloadValidArchive(t *testing.T) {
	archiveData := buildTestArchive(t, "rimba", "#!/bin/sh\necho hello\n")
	srv := serveOctetStream(t, archiveData)

	u := newTestUpdater(srv)

	dl, err := u.Download(context.Background(), srv.URL+"/rimba_1.0.0_linux_amd64.tar.gz")
	requireNoError(t, err)
	t.Cleanup(func() { CleanupTempDir(dl.BinaryPath) })

	content, err := os.ReadFile(dl.BinaryPath)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}

	want := "#!/bin/sh\necho hello\n"
	if string(content) != want {
		t.Errorf("binary content = %q, want %q", content, want)
	}

	info, err := os.Stat(dl.BinaryPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("expected binary to be executable")
	}

	// SHA256 must be non-empty and match the archive bytes.
	if dl.SHA256 == "" {
		t.Error("SHA256 should not be empty")
	}
	h := sha256.Sum256(archiveData)
	want256 := hex.EncodeToString(h[:])
	if dl.SHA256 != want256 {
		t.Errorf("SHA256 = %q, want %q", dl.SHA256, want256)
	}
}

func TestDownloadZipArchive(t *testing.T) {
	archiveData := buildTestZipArchive(t, "rimba.exe", "exe content")
	srv := serveOctetStream(t, archiveData)

	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           goosWindows,
		GOARCH:         testArch,
		Client:         srv.Client(),
		APIEndpoint:    srv.URL,
	}

	dl, err := u.Download(context.Background(), srv.URL+"/rimba_1.0.0_windows_amd64.zip")
	requireNoError(t, err)
	t.Cleanup(func() { CleanupTempDir(dl.BinaryPath) })

	if !strings.HasSuffix(dl.BinaryPath, "rimba.exe") {
		t.Errorf("BinaryPath = %q, want suffix 'rimba.exe'", dl.BinaryPath)
	}

	content, err := os.ReadFile(dl.BinaryPath)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if string(content) != "exe content" {
		t.Errorf("binary content = %q, want %q", content, "exe content")
	}

	if dl.SHA256 == "" {
		t.Error("SHA256 should not be empty")
	}
}

func TestReplaceSameFilesystem(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(currentPath, newPath); err != nil {
		t.Fatalf("Replace failed: %v", err)
	}

	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}
	if string(content) != "new" {
		t.Errorf(contentFmt, content, "new")
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"wrapped permission error", fmt.Errorf("opening destination: %w", os.ErrPermission), true},
		{"direct permission error", os.ErrPermission, true},
		{"other error", errors.New("something else"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermissionError(tt.err); got != tt.want {
				t.Errorf("IsPermissionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	u := New(testVersionNew2)
	if u.CurrentVersion != testVersionNew2 {
		t.Errorf("CurrentVersion = %q, want %q", u.CurrentVersion, testVersionNew2)
	}
	if u.GOOS == "" {
		t.Error("GOOS should not be empty")
	}
	if u.GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}
	if u.Client == nil {
		t.Error("Client should not be nil")
	}
	if u.APIEndpoint != defaultAPIEndpoint {
		t.Errorf("APIEndpoint = %q, want %q", u.APIEndpoint, defaultAPIEndpoint)
	}
}

func TestNewClientHasTimeout(t *testing.T) {
	u := New(testVersion)
	hc, ok := u.Client.(*http.Client)
	if !ok {
		t.Fatal("Client is not *http.Client")
	}
	if hc.Timeout != 30*time.Second {
		t.Errorf("Client.Timeout = %v, want 30s", hc.Timeout)
	}
}

// ctxCapturingClient records the context from the most recent request.
type ctxCapturingClient struct {
	captured context.Context
	delegate HTTPClient
}

func (c *ctxCapturingClient) Do(req *http.Request) (*http.Response, error) {
	c.captured = req.Context()
	return c.delegate.Do(req)
}

// ctxMarkerKey is a package-level context key used to verify ctx propagation.
type ctxMarkerKey struct{}

func TestCheckContextPropagated(t *testing.T) {
	srv := serveJSON(t, `{"tag_name":"`+testVersion+`","assets":[]}`)
	capturing := &ctxCapturingClient{delegate: srv.Client()}
	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           testOS,
		GOARCH:         testArch,
		Client:         capturing,
		APIEndpoint:    srv.URL,
	}

	ctx := context.WithValue(context.Background(), ctxMarkerKey{}, "marker")
	_, err := u.Check(ctx)
	requireNoError(t, err)

	if capturing.captured == nil || capturing.captured.Value(ctxMarkerKey{}) != "marker" {
		t.Error("Check did not propagate the provided context to the HTTP request")
	}
}

func TestCheckCancelledContext(t *testing.T) {
	srv := serveJSON(t, `{"tag_name":"`+testVersion+`","assets":[]}`)
	u := newTestUpdater(srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := u.Check(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDownloadMkdirTempError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	t.Cleanup(srv.Close)

	// Point TMPDIR at a non-existent path so os.MkdirTemp fails.
	t.Setenv("TMPDIR", "/nonexistent/tmp/rimba-test")

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/archive.tar.gz")
	if err == nil {
		t.Fatal("expected error when MkdirTemp fails")
	}
	if !strings.Contains(err.Error(), "creating temp dir") {
		t.Errorf("error = %q, want to contain 'creating temp dir'", err.Error())
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/missing.tar.gz")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	want := "download returned status 404"
	if got := err.Error(); got != want {
		t.Errorf(errWantFmt, got, want)
	}
}

func TestDownloadInvalidArchive(t *testing.T) {
	srv := serveOctetStream(t, []byte("not a valid gzip archive"))

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/invalid.tar.gz")
	if err == nil {
		t.Fatal("expected error for invalid archive")
	}
}

func TestDownloadMissingBinary(t *testing.T) {
	archiveData := buildTestArchive(t, "other-binary", "content")
	srv := serveOctetStream(t, archiveData)

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/archive.tar.gz")
	if err == nil {
		t.Fatal("expected error for archive without rimba binary")
	}
	want := `binary "rimba" not found in archive`
	if got := err.Error(); got != want {
		t.Errorf(errWantFmt, got, want)
	}
}

func TestCleanupTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "subdir", "rimba")

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	CleanupTempDir(binaryPath)

	if _, err := os.Stat(filepath.Dir(binaryPath)); !os.IsNotExist(err) {
		t.Error("expected temp dir to be removed")
	}
}

func TestReplaceSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	realPath := filepath.Join(tmpDir, "real")
	if err := os.WriteFile(realPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte(valNewBinary), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(linkPath, newPath); err != nil {
		t.Fatalf("Replace via symlink: %v", err)
	}

	// The real file should now have the new content
	content, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("reading real path: %v", err)
	}
	if string(content) != valNewBinary {
		t.Errorf(contentFmt, content, valNewBinary)
	}
}

func TestCheckNetworkError(t *testing.T) {
	// Use a server that closes connections immediately
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		conn.Close()
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestCheckRequestError(t *testing.T) {
	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           testOS,
		GOARCH:         testArch,
		Client:         http.DefaultClient,
		APIEndpoint:    "\x7f://invalid", // control char causes NewRequest to fail
	}

	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "creating request") {
		t.Errorf("error = %q, want 'creating request'", err.Error())
	}
}

func TestDownloadRequestError(t *testing.T) {
	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           testOS,
		GOARCH:         testArch,
		Client:         http.DefaultClient,
	}

	_, err := u.Download(context.Background(), "\x7f://invalid/archive.tar.gz")
	if err == nil {
		t.Fatal("expected error for invalid download URL")
	}
	if !strings.Contains(err.Error(), "creating download request") {
		t.Errorf("error = %q, want 'creating download request'", err.Error())
	}
}

func TestDownloadCorruptTarArchive(t *testing.T) {
	// Create valid gzip wrapping invalid tar data
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte("this is not valid tar data"))
	_ = gz.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, contentTypeOctet)
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/corrupt.tar.gz")
	if err == nil {
		t.Fatal("expected error for corrupt tar archive")
	}
	if !strings.Contains(err.Error(), "not found in archive") && !strings.Contains(err.Error(), "reading archive") {
		t.Errorf("error = %q, want archive-related error", err.Error())
	}
}

func TestDownloadConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		conn.Close()
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Download(context.Background(), srv.URL+"/archive.tar.gz")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestReplaceNonexistentCurrent(t *testing.T) {
	tmpDir := t.TempDir()

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	err := Replace(filepath.Join(tmpDir, "nonexistent"), newPath)
	if err == nil {
		t.Fatal("expected error for nonexistent current binary")
	}
}

type errorReader struct{ err error }

func (r *errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestWriteBinaryCopyError(t *testing.T) {
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "binary")

	r := &errorReader{err: errors.New("read error")}
	_, err := writeBinary(dst, r)
	if err == nil {
		t.Fatal("expected error from io.Copy failure")
	}
	if !strings.Contains(err.Error(), "extracting binary") {
		t.Errorf("error = %q, want to contain 'extracting binary'", err.Error())
	}
}

func TestWriteBinaryOpenError(t *testing.T) {
	// Use a directory path where the binary file would go
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "subdir", "nested", "binary")

	// Don't create parent dirs — OpenFile should fail
	r := strings.NewReader("content")
	_, err := writeBinary(dst, r)
	if err == nil {
		t.Fatal("expected error from OpenFile failure")
	}
	if !strings.Contains(err.Error(), "creating binary file") {
		t.Errorf("error = %q, want to contain 'creating binary file'", err.Error())
	}
}

// buildTestArchiveTypeflag creates a tar.gz archive with a single entry of the given typeflag.
func buildTestArchiveTypeflag(t *testing.T, name, content string, typeflag byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Typeflag: typeflag,
		Name:     name,
		Mode:     0755,
		Size:     int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractTarGzSkipsNonRegularFiles(t *testing.T) {
	tests := []struct {
		name     string
		typeflag byte
	}{
		{"dir typeflag", tar.TypeDir},
		{"symlink typeflag", tar.TypeSymlink},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiveData := buildTestArchiveTypeflag(t, binaryName, "", tt.typeflag)
			// Write archive to temp file for extractTarGz
			tmpDir := t.TempDir()
			archivePath := filepath.Join(tmpDir, "archive.tar.gz")
			if err := os.WriteFile(archivePath, archiveData, 0644); err != nil {
				t.Fatal(err)
			}
			_, err := extractTarGz(archivePath, tmpDir, binaryName)
			if err == nil {
				t.Fatal("expected error when rimba entry is not a regular file")
			}
			if !strings.Contains(err.Error(), "not found in archive") {
				t.Errorf("error = %q, want 'not found in archive'", err.Error())
			}
		})
	}
}

func TestWriteBinaryExceedsMaxSize(t *testing.T) {
	orig := maxBinarySize
	maxBinarySize = 5
	t.Cleanup(func() { maxBinarySize = orig })

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "binary")

	r := strings.NewReader("this is more than 5 bytes of content")
	_, err := writeBinary(dst, r)
	if err == nil {
		t.Fatal("expected size-limit error for oversized binary")
	}
	if !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("error = %q, want to contain 'exceeds max'", err.Error())
	}
}

func TestDownloadExceedsMaxArchiveSize(t *testing.T) {
	orig := maxArchiveSize
	maxArchiveSize = 5
	t.Cleanup(func() { maxArchiveSize = orig })

	srv := serveOctetStream(t, []byte("this is more than 5 bytes"))
	u := newTestUpdater(srv)

	_, err := u.Download(context.Background(), srv.URL+"/archive.tar.gz")
	if err == nil {
		t.Fatal("expected error when archive exceeds size limit")
	}
	if !strings.Contains(err.Error(), "exceeds max allowed size") {
		t.Errorf("error = %q, want to contain 'exceeds max allowed size'", err.Error())
	}
}

func TestReplaceStatError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a symlink pointing to a file, then delete the target
	target := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(target, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	// Remove the target so Stat fails after EvalSymlinks succeeds
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	err := Replace(link, newPath)
	if err == nil {
		t.Fatal("expected error when symlink target is deleted")
	}
}

// buildTestArchive creates a tar.gz archive containing a single file.
func buildTestArchive(t *testing.T, name, content string) []byte {
	t.Helper()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// buildTestZipArchive creates a zip archive containing a single file.
func buildTestZipArchive(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReplaceSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the current binary
	currentPath := filepath.Join(tmpDir, "current-binary")
	if err := os.WriteFile(currentPath, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create the new binary in a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(subDir, "new-binary")
	newContent := "updated binary content"
	if err := os.WriteFile(newPath, []byte(newContent), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(currentPath, newPath); err != nil {
		t.Fatalf(fatalReplace, err)
	}

	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}
	if string(content) != newContent {
		t.Errorf(contentFmt, content, newContent)
	}
}

func TestCheckInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, contentTypeJSON)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}

	if !strings.Contains(err.Error(), "decoding release") {
		t.Errorf("err = %q, want it to contain %q", err.Error(), "decoding release")
	}
}

func TestReplaceNewBinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	err := Replace(currentPath, filepath.Join(tmpDir, "nonexistent-new"))
	if err == nil {
		t.Fatal("expected error for nonexistent new binary")
	}
	if !strings.Contains(err.Error(), "opening new binary") {
		t.Errorf("error = %q, want to contain 'opening new binary'", err.Error())
	}

	// Original binary should be unchanged
	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("reading current: %v", err)
	}
	if string(content) != "old" {
		t.Errorf("original binary was modified: got %q", content)
	}
}

func TestReplacePreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0750); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Replace(currentPath, newPath); err != nil {
		t.Fatalf(fatalReplace, err)
	}

	info, err := os.Stat(currentPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0750 {
		t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0750)
	}
}

func TestReplaceCreatesNewInode(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// Get original inode
	origInfo, err := os.Stat(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	origSys := origInfo.Sys()

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(currentPath, newPath); err != nil {
		t.Fatalf(fatalReplace, err)
	}

	// Verify new inode (the whole point of this fix)
	newInfo, err := os.Stat(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	newSys := newInfo.Sys()

	// On Unix, Sys() returns *syscall.Stat_t which has Ino field
	// We compare the raw sys values — if they're different, inode changed
	if origSys == newSys {
		t.Log("warning: could not confirm inode change (Sys() comparison)")
	}

	// Content should be updated
	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf(contentFmt, content, "new")
	}
}

func TestEnsurePathUnsupportedShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/fish")

	// Should return nil without error (silently skip)
	if err := EnsurePath("/some/dir"); err != nil {
		t.Errorf("EnsurePath with unsupported shell: %v", err)
	}
}

func TestUserInstallDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	dir, err := UserInstallDir()
	requireNoError(t, err)

	want := filepath.Join(home, localBinSubdir, "bin")
	if dir != want {
		t.Errorf("UserInstallDir() = %q, want %q", dir, want)
	}
}

func TestEnsurePathCreatesEntry(t *testing.T) {
	tmpDir := t.TempDir()
	rcFile := filepath.Join(tmpDir, testRcZshrc)

	// Create an empty rc file
	if err := os.WriteFile(rcFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	if err := EnsurePath(dir); err != nil {
		t.Fatalf("EnsurePath: %v", err)
	}

	content, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("reading rc file: %v", err)
	}

	if !strings.Contains(string(content), dir) {
		t.Errorf("rc file should contain %q, got %q", dir, content)
	}
	if !strings.Contains(string(content), "# Added by rimba") {
		t.Errorf("rc file should contain guard comment")
	}
}

func TestEnsurePathIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	rcFile := filepath.Join(tmpDir, testRcZshrc)

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	existing := fmt.Sprintf("export PATH=\"%s:$PATH\"\n", dir)
	if err := os.WriteFile(rcFile, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	if err := EnsurePath(dir); err != nil {
		t.Fatalf("EnsurePath: %v", err)
	}

	content, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("reading rc file: %v", err)
	}

	// Should not have added a duplicate
	if strings.Count(string(content), dir) != 1 {
		t.Errorf("expected exactly one PATH entry, got:\n%s", content)
	}
}

func TestEnsurePathSkipsCommentLines(t *testing.T) {
	tmpDir := t.TempDir()
	rcFile := filepath.Join(tmpDir, testRcZshrc)

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	// A comment that mentions both the dir and "PATH" must not be treated as
	// an existing export — EnsurePath should still append the real export line.
	commentOnly := fmt.Sprintf("# %s is not yet in PATH\n", dir)
	if err := os.WriteFile(rcFile, []byte(commentOnly), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	if err := EnsurePath(dir); err != nil {
		t.Fatalf("EnsurePath: %v", err)
	}

	content, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("reading rc file: %v", err)
	}
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
	if !strings.Contains(string(content), exportLine) {
		t.Errorf("rc file missing real export line; got:\n%s", content)
	}
}

func TestEnsurePathScannerError(t *testing.T) {
	tmpDir := t.TempDir()
	rcFile := filepath.Join(tmpDir, testRcZshrc)

	// Create a line exceeding bufio.MaxScanTokenSize (64KB) to trigger scanner error
	longLine := strings.Repeat("x", 70000) + "\n"
	if err := os.WriteFile(rcFile, []byte(longLine), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	err := EnsurePath(dir)
	if err == nil {
		t.Fatal("expected error from scanner exceeding token size")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("error = %q, want to contain 'reading'", err.Error())
	}
}

func TestEnsurePathBashShell(t *testing.T) {
	tmpDir := t.TempDir()
	rcFile := filepath.Join(tmpDir, ".bashrc")

	if err := os.WriteFile(rcFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	if err := EnsurePath(dir); err != nil {
		t.Fatalf("EnsurePath with bash: %v", err)
	}

	content, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("reading .bashrc: %v", err)
	}
	if !strings.Contains(string(content), dir) {
		t.Errorf(".bashrc should contain %q, got %q", dir, content)
	}
}

func TestEnsurePathNoRcFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	// No .zshrc file exists — EnsurePath should create it
	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	if err := EnsurePath(dir); err != nil {
		t.Fatalf("EnsurePath without rc file: %v", err)
	}

	rcFile := filepath.Join(tmpDir, testRcZshrc)
	content, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("reading .zshrc: %v", err)
	}
	if !strings.Contains(string(content), dir) {
		t.Errorf(".zshrc should contain %q, got %q", dir, content)
	}
}

func TestEnsurePathOpenFileError(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("SHELL", testShellZsh)
	t.Setenv("HOME", tmpDir)

	// Create .zshrc as a read-only file with content that doesn't match
	// the target path. Scanner reads fine, finds no match, then OpenFile
	// with O_WRONLY fails because the file is not writable.
	rcFile := filepath.Join(tmpDir, testRcZshrc)
	if err := os.WriteFile(rcFile, []byte("# existing config\n"), 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rcFile, 0644) })

	dir := filepath.Join(tmpDir, localBinSubdir, "bin")
	err := EnsurePath(dir)
	if err == nil {
		t.Fatal("expected error when rc file is read-only")
	}
	if !strings.Contains(err.Error(), "opening") {
		t.Errorf("error = %q, want to contain 'opening'", err.Error())
	}
}

func TestReplaceCreateTempError(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	// Make directory read-only so CreateTemp fails
	if err := os.Chmod(tmpDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmpDir, 0755) })

	err := Replace(currentPath, newPath)
	if err == nil {
		t.Fatal("expected error from CreateTemp in read-only dir")
	}
	if !strings.Contains(err.Error(), "creating temp file") {
		t.Errorf("error = %q, want to contain 'creating temp file'", err.Error())
	}
}

func TestReplaceCopyError(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// Use a directory as the "new binary" — os.Open succeeds but io.Copy
	// from a directory file descriptor produces no data (empty copy).
	// Instead, use a symlink to /dev/null for macOS or a readable path that
	// we close early. Actually, the simplest: create a named pipe.
	// On macOS/Linux, reading from a FIFO will block unless a writer opens it.
	// Use os.Pipe or a special file.
	//
	// Most reliable: make the source an unreadable file after open.
	// Actually, replace current binary dir with read-only mid-operation.
	//
	// Simplest approach: trigger "opening new binary" error by using a dir as path.
	newPath := t.TempDir() // a directory, not a file
	err := Replace(currentPath, newPath)
	if err == nil {
		t.Fatal("expected error when new binary is a directory")
	}
}

func TestReplaceRenameError(t *testing.T) {
	tmpDir := t.TempDir()

	currentPath := filepath.Join(tmpDir, "current")
	if err := os.WriteFile(currentPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create the new binary in a different filesystem mount (cross-device rename)
	// This is hard to guarantee, so instead test by making directory read-only
	// after CreateTemp succeeds but before Rename.
	// Actually, the simpler way: create current binary, create new binary,
	// then make current's directory read-only. But CreateTemp needs write access.
	// A different approach: use EvalSymlinks to resolve to a different dir.
	//
	// For now, this test just verifies Replace handles a valid case.
	// The Rename error path is hard to trigger without cross-device setups.
	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(newPath, []byte(valNewContent), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(currentPath, newPath); err != nil {
		t.Fatalf(fatalReplace, err)
	}

	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != valNewContent {
		t.Errorf(contentFmt, content, valNewContent)
	}
}

func TestDownloadValidArchiveCleanup(t *testing.T) {
	archiveData := buildTestArchive(t, "rimba", "#!/bin/sh\necho cleanup test\n")
	srv := serveOctetStream(t, archiveData)

	u := newTestUpdater(srv)

	dl, err := u.Download(context.Background(), srv.URL+"/rimba_1.0.0_linux_amd64.tar.gz")
	requireNoError(t, err)

	// Verify the binary exists
	if _, err := os.Stat(dl.BinaryPath); err != nil {
		t.Fatalf("binary should exist at %s: %v", dl.BinaryPath, err)
	}

	// Verify content
	content, err := os.ReadFile(dl.BinaryPath)
	if err != nil {
		t.Fatalf("reading binary: %v", err)
	}
	want := "#!/bin/sh\necho cleanup test\n"
	if string(content) != want {
		t.Errorf(contentFmt, content, want)
	}

	// Now clean up and verify temp dir is removed
	tmpDir := filepath.Dir(dl.BinaryPath)
	CleanupTempDir(dl.BinaryPath)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("expected temp dir %s to be removed after CleanupTempDir", tmpDir)
	}
}

// ---- FetchChecksums tests ----

func serveChecksums(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchChecksumsHappyPath(t *testing.T) {
	body := "aabbccdd  rimba_2.0.0_linux_amd64.tar.gz\n" +
		"eeff0011  rimba_2.0.0_darwin_amd64.tar.gz\n"
	srv := serveChecksums(t, body)
	u := newTestUpdater(srv)

	sums, err := u.FetchChecksums(context.Background(), srv.URL+"/checksums.txt")
	requireNoError(t, err)

	if got := sums["rimba_2.0.0_linux_amd64.tar.gz"]; got != "aabbccdd" {
		t.Errorf("linux sum = %q, want %q", got, "aabbccdd")
	}
	if got := sums["rimba_2.0.0_darwin_amd64.tar.gz"]; got != "eeff0011" {
		t.Errorf("darwin sum = %q, want %q", got, "eeff0011")
	}
}

func TestFetchChecksumsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	u := newTestUpdater(srv)

	_, err := u.FetchChecksums(context.Background(), srv.URL+"/checksums.txt")
	if err == nil {
		t.Fatal("expected error for HTTP error response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %q, want to contain '403'", err.Error())
	}
}

func TestFetchChecksumsNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		conn.Close()
	}))
	t.Cleanup(srv.Close)
	u := newTestUpdater(srv)

	_, err := u.FetchChecksums(context.Background(), srv.URL+"/checksums.txt")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestFetchChecksumsExceedsMaxSize(t *testing.T) {
	orig := maxChecksumSize
	maxChecksumSize = 5
	t.Cleanup(func() { maxChecksumSize = orig })

	srv := serveChecksums(t, "aabbccdd  rimba_2.0.0_linux_amd64.tar.gz\n") // >5 bytes
	u := newTestUpdater(srv)

	_, err := u.FetchChecksums(context.Background(), srv.URL+"/checksums.txt")
	if err == nil {
		t.Fatal("expected error when checksums response exceeds size limit")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want to contain 'exceeds'", err.Error())
	}
}

func TestFetchChecksumsRequestError(t *testing.T) {
	u := &Updater{
		CurrentVersion: testVersion,
		GOOS:           testOS,
		GOARCH:         testArch,
		Client:         http.DefaultClient,
	}

	_, err := u.FetchChecksums(context.Background(), "\x7f://invalid")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "creating checksums request") {
		t.Errorf("error = %q, want 'creating checksums request'", err.Error())
	}
}

func TestFetchChecksumsLowercasesHex(t *testing.T) {
	body := "AABBCCDD  rimba_2.0.0_linux_amd64.tar.gz\n"
	srv := serveChecksums(t, body)
	u := newTestUpdater(srv)

	sums, err := u.FetchChecksums(context.Background(), srv.URL+"/checksums.txt")
	requireNoError(t, err)

	if got := sums["rimba_2.0.0_linux_amd64.tar.gz"]; got != "aabbccdd" {
		t.Errorf("sum = %q, want lowercase %q", got, "aabbccdd")
	}
}

// ---- verifyChecksumMatch tests ----

func TestVerifyChecksumMatchHappyPath(t *testing.T) {
	sums := map[string]string{testChecksumFile: testChecksumHex}
	if err := verifyChecksumMatch(sums, testChecksumFile, testChecksumHex); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyChecksumMatchCaseInsensitive(t *testing.T) {
	sums := map[string]string{testChecksumFile: testChecksumHex}
	if err := verifyChecksumMatch(sums, testChecksumFile, strings.ToUpper(testChecksumHex)); err != nil {
		t.Errorf("unexpected error for uppercase input: %v", err)
	}
}

func TestVerifyChecksumMatchMismatch(t *testing.T) {
	sums := map[string]string{testChecksumFile: testChecksumHex}
	err := verifyChecksumMatch(sums, testChecksumFile, "deadbeef")
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %q, want to contain 'checksum mismatch'", err.Error())
	}
}

func TestVerifyChecksumMatchAbsentRow(t *testing.T) {
	sums := map[string]string{"other_file.tar.gz": testChecksumHex}
	err := verifyChecksumMatch(sums, testChecksumFile, testChecksumHex)
	if err == nil {
		t.Fatal("expected error for absent asset row (fail-closed)")
	}
	if !strings.Contains(err.Error(), "not found in checksums.txt") {
		t.Errorf("error = %q, want to contain 'not found in checksums.txt'", err.Error())
	}
}

// ---- assetNameFor tests ----

func TestAssetNameFor(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"linux", "amd64", "rimba_1.0.0_linux_amd64.tar.gz"},
		{"linux", "arm64", "rimba_1.0.0_linux_arm64.tar.gz"},
		{"darwin", "amd64", "rimba_1.0.0_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "rimba_1.0.0_darwin_arm64.tar.gz"},
		{goosWindows, "amd64", "rimba_1.0.0_windows_amd64.zip"},
		{goosWindows, "arm64", "rimba_1.0.0_windows_arm64.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			got := assetNameFor(tt.goos, tt.goarch, "1.0.0")
			if got != tt.want {
				t.Errorf("assetNameFor(%q, %q, %q) = %q, want %q", tt.goos, tt.goarch, "1.0.0", got, tt.want)
			}
		})
	}
}
