package updater

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

func TestCheckUpToDate(t *testing.T) {
	srv := serveJSON(t, `{"tag_name":"`+testVersion+`","assets":[]}`)
	u := newTestUpdater(srv)

	result, err := u.Check()
	requireNoError(t, err)
	if !result.UpToDate {
		t.Errorf("expected up to date, got not up to date")
	}
	if result.LatestVersion != testVersion {
		t.Errorf("expected latest %s, got %s", testVersion, result.LatestVersion)
	}
}

func TestCheckNewVersionAvailable(t *testing.T) {
	srv := serveJSON(t, `{
		"tag_name":"`+testVersionNew+`",
		"assets":[
			{"name":"rimba_2.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/rimba_2.0.0_linux_amd64.tar.gz"}
		]
	}`)
	u := newTestUpdater(srv)

	result, err := u.Check()
	requireNoError(t, err)
	if result.UpToDate {
		t.Errorf("expected not up to date")
	}

	wantURL := "https://example.com/rimba_2.0.0_linux_amd64.tar.gz"
	if result.DownloadURL != wantURL {
		t.Errorf("download URL = %q, want %q", result.DownloadURL, wantURL)
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

	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error for missing asset")
	}

	want := "no matching asset for linux/amd64 in release " + testVersionNew
	if got := err.Error(); got != want {
		t.Errorf(errWantFmt, got, want)
	}
}

func TestCheckAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	u := newTestUpdater(srv)

	_, err := u.Check()
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

	binaryPath, err := u.Download(srv.URL + "/rimba_1.0.0_linux_amd64.tar.gz")
	requireNoError(t, err)
	t.Cleanup(func() { CleanupTempDir(binaryPath) })

	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}

	want := "#!/bin/sh\necho hello\n"
	if string(content) != want {
		t.Errorf("binary content = %q, want %q", content, want)
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("expected binary to be executable")
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
		t.Errorf("content = %q, want %q", content, "new")
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
		{"other error", fmt.Errorf("something else"), false},
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

func TestReplaceElevatedFailsGracefully(t *testing.T) {
	err := ReplaceElevated("/nonexistent/path/binary", "/also/nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent paths")
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

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Download(srv.URL + "/missing.tar.gz")
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
	_, err := u.Download(srv.URL + "/invalid.tar.gz")
	if err == nil {
		t.Fatal("expected error for invalid archive")
	}
}

func TestDownloadMissingBinary(t *testing.T) {
	archiveData := buildTestArchive(t, "other-binary", "content")
	srv := serveOctetStream(t, archiveData)

	u := newTestUpdater(srv)
	_, err := u.Download(srv.URL + "/archive.tar.gz")
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
