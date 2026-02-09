package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/updater"
)

const (
	hdrContentType  = "Content-Type"
	mimeJSON        = "application/json"
	testVersionHint = "v1.0.0"
)

// overrideNewUpdater temporarily replaces newUpdater to point at the test server.
func overrideNewUpdater(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := newUpdater
	newUpdater = func(version string) *updater.Updater {
		return &updater.Updater{
			CurrentVersion: version,
			GOOS:           "linux",
			GOARCH:         "amd64",
			Client:         srv.Client(),
			APIEndpoint:    srv.URL,
		}
	}
	t.Cleanup(func() { newUpdater = orig })
}

func TestCheckUpdateHintNewVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(hdrContentType, mimeJSON)
		_, _ = w.Write([]byte(`{
			"tag_name":"v2.0.0",
			"assets":[
				{"name":"rimba_2.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/download"}
			]
		}`))
	}))
	t.Cleanup(srv.Close)
	overrideNewUpdater(t, srv)

	ch := checkUpdateHint(testVersionHint, 2*time.Second)
	result := collectHint(ch)
	if result == nil {
		t.Fatal("expected non-nil result for available update")
	}
	if result.LatestVersion != "v2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "v2.0.0")
	}
}

func TestCheckUpdateHintUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(hdrContentType, mimeJSON)
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	overrideNewUpdater(t, srv)

	ch := checkUpdateHint(testVersionHint, 2*time.Second)
	result := collectHint(ch)
	if result != nil {
		t.Errorf("expected nil result for up-to-date version, got %+v", result)
	}
}

func TestCheckUpdateHintDevVersion(t *testing.T) {
	// Should not make any HTTP calls for dev versions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP request for dev version")
	}))
	t.Cleanup(srv.Close)
	overrideNewUpdater(t, srv)

	ch := checkUpdateHint("dev", 2*time.Second)
	result := collectHint(ch)
	if result != nil {
		t.Errorf("expected nil result for dev version, got %+v", result)
	}
}

func TestCheckUpdateHintTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set(hdrContentType, mimeJSON)
		_, _ = w.Write([]byte(`{
			"tag_name":"v2.0.0",
			"assets":[
				{"name":"rimba_2.0.0_linux_amd64.tar.gz","browser_download_url":"https://example.com/download"}
			]
		}`))
	}))
	t.Cleanup(srv.Close)
	overrideNewUpdater(t, srv)

	ch := checkUpdateHint(testVersionHint, 50*time.Millisecond)
	result := collectHint(ch)
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
}
