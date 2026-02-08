package updater

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const (
	testVersion     = "v1.0.0"
	testOS          = "linux"
	testArch        = "amd64"
	contentTypeJSON = "application/json"
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
		w.Header().Set("Content-Type", contentTypeJSON)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckUpToDate(t *testing.T) {
	srv := serveJSON(t, `{"tag_name":"v1.0.0","assets":[]}`)
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
		"tag_name":"v2.0.0",
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
		"tag_name":"v2.0.0",
		"assets":[
			{"name":"rimba_2.0.0_windows_amd64.zip","browser_download_url":"https://example.com/rimba_2.0.0_windows_amd64.zip"}
		]
	}`)
	u := newTestUpdater(srv)

	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error for missing asset")
	}

	want := "no matching asset for linux/amd64 in release v2.0.0"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
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
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestIsDevVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"", true},
		{"v1.0.0", false},
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archiveData)
	}))
	t.Cleanup(srv.Close)

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
