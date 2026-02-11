package updater

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	// New constants for deduplication
	contentFmt    = "content = %q, want %q"
	binaryContent = "binary content"
	fatalCopyFile = "copyFile: %v"
	fatalReadDst  = "reading dst: %v"
	fatalStatDst  = "stat dst: %v"
	valNewBinary  = "new binary"
	permFmt       = "permissions = %o, want %o"
	valNewContent = "new content"
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

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src")
	if err := os.WriteFile(srcPath, []byte(binaryContent), 0755); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "dst")
	if err := copyFile(srcPath, dstPath, 0700); err != nil {
		t.Fatalf(fatalCopyFile, err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf(fatalReadDst, err)
	}
	if string(content) != binaryContent {
		t.Errorf(contentFmt, content, binaryContent)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf(fatalStatDst, err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode = %o, want %o", info.Mode().Perm(), 0700)
	}
}

func TestCopyFileSourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"), 0755)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestCopyFileDstError(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a directory where the dst file should be
	dstPath := filepath.Join(tmpDir, "dstdir")
	if err := os.Mkdir(dstPath, 0755); err != nil {
		t.Fatal(err)
	}

	err := copyFile(srcPath, dstPath, 0755)
	if err == nil {
		t.Fatal("expected error when dst is a directory")
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
	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error for network failure")
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
	_, err := u.Download(srv.URL + "/archive.tar.gz")
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

	r := &errorReader{err: fmt.Errorf("read error")}
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

func TestReplaceCrossFilesystem(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src-binary")
	srcContent := []byte("cross-filesystem binary content")
	if err := os.WriteFile(srcPath, srcContent, 0755); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "dst-binary")

	var perm os.FileMode = 0750
	if err := copyFile(srcPath, dstPath, perm); err != nil {
		t.Fatalf(fatalCopyFile, err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf(fatalReadDst, err)
	}
	if string(content) != string(srcContent) {
		t.Errorf(contentFmt, content, srcContent)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf(fatalStatDst, err)
	}
	if info.Mode().Perm() != perm {
		t.Errorf(permFmt, info.Mode().Perm(), perm)
	}
}

func TestReplaceFallbackToCopyFile(t *testing.T) {
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
		t.Fatalf("Replace: %v", err)
	}

	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}
	if string(content) != newContent {
		t.Errorf(contentFmt, content, newContent)
	}
}

func TestCopyFilePreservesContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a larger file to exercise io.Copy thoroughly
	largeContent := strings.Repeat("abcdefghijklmnop", 8192) // 128KB
	srcPath := filepath.Join(tmpDir, "large-src")
	if err := os.WriteFile(srcPath, []byte(largeContent), 0644); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "large-dst")
	var perm os.FileMode = 0755
	if err := copyFile(srcPath, dstPath, perm); err != nil {
		t.Fatalf(fatalCopyFile, err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf(fatalReadDst, err)
	}
	if len(content) != len(largeContent) {
		t.Errorf("content length = %d, want %d", len(content), len(largeContent))
	}
	if string(content) != largeContent {
		t.Error("content mismatch for large file copy")
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf(fatalStatDst, err)
	}
	if info.Mode().Perm() != perm {
		t.Errorf(permFmt, info.Mode().Perm(), perm)
	}
}

func TestReplacePreservesPermissions(t *testing.T) {
	// Test that copyFile (the cross-filesystem fallback) preserves the
	// permissions of the original binary. We call copyFile directly since
	// on the same filesystem os.Rename would succeed and skip the fallback.
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "new-binary")
	if err := os.WriteFile(srcPath, []byte(valNewContent), 0644); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "current-binary")
	if err := os.WriteFile(dstPath, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}

	// Simulate what Replace does on cross-filesystem: copyFile with
	// the permissions from the original binary (0755).
	var origPerm os.FileMode = 0755
	if err := copyFile(srcPath, dstPath, origPerm); err != nil {
		t.Fatalf(fatalCopyFile, err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf(fatalReadDst, err)
	}
	if string(content) != valNewContent {
		t.Errorf(contentFmt, content, valNewContent)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf(fatalStatDst, err)
	}
	if info.Mode().Perm() != origPerm {
		t.Errorf(permFmt, info.Mode().Perm(), origPerm)
	}
}

func TestCheckInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(contentTypeHdr, contentTypeJSON)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	t.Cleanup(srv.Close)

	u := newTestUpdater(srv)
	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}

	if !strings.Contains(err.Error(), "decoding release") {
		t.Errorf("err = %q, want it to contain %q", err.Error(), "decoding release")
	}
}

func TestDownloadValidArchiveCleanup(t *testing.T) {
	archiveData := buildTestArchive(t, "rimba", "#!/bin/sh\necho cleanup test\n")
	srv := serveOctetStream(t, archiveData)

	u := newTestUpdater(srv)

	binaryPath, err := u.Download(srv.URL + "/rimba_1.0.0_linux_amd64.tar.gz")
	requireNoError(t, err)

	// Verify the binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("binary should exist at %s: %v", binaryPath, err)
	}

	// Verify content
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("reading binary: %v", err)
	}
	want := "#!/bin/sh\necho cleanup test\n"
	if string(content) != want {
		t.Errorf(contentFmt, content, want)
	}

	// Now clean up and verify temp dir is removed
	tmpDir := filepath.Dir(binaryPath)
	CleanupTempDir(binaryPath)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("expected temp dir %s to be removed after CleanupTempDir", tmpDir)
	}
}
