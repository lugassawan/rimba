package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/updater"
)

const (
	hdrContentType   = "Content-Type"
	mimeJSON         = "application/json"
	testVersionHint  = "v1.0.0"
	testVersionNew   = "v2.0.0"
	testVersionOther = "v3.0.0"
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
			"tag_name":"` + testVersionNew + `",
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
	if result.LatestVersion != testVersionNew {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, testVersionNew)
	}
}

func TestCheckUpdateHintUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(hdrContentType, mimeJSON)
		_, _ = w.Write([]byte(`{"tag_name":"` + testVersionHint + `","assets":[]}`))
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
			"tag_name":"` + testVersionNew + `",
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

func TestPrintUpdateHint(t *testing.T) {
	cmd, buf := newTestCmd()
	result := &updater.CheckResult{
		CurrentVersion: testVersionHint,
		LatestVersion:  testVersionNew,
	}
	printUpdateHint(cmd, result)

	out := buf.String()
	if !strings.Contains(out, "Update available") {
		t.Errorf("output missing 'Update available': %q", out)
	}
	if !strings.Contains(out, testVersionHint) {
		t.Errorf("output missing current version: %q", out)
	}
	if !strings.Contains(out, testVersionNew) {
		t.Errorf("output missing latest version: %q", out)
	}
}

func TestCollectHintClosed(t *testing.T) {
	ch := make(chan *updater.CheckResult)
	close(ch)
	result := collectHint(ch)
	if result != nil {
		t.Errorf("expected nil for closed channel, got %+v", result)
	}
}

func TestCollectHintValue(t *testing.T) {
	ch := make(chan *updater.CheckResult, 1)
	ch <- &updater.CheckResult{LatestVersion: testVersionOther}
	result := collectHint(ch)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LatestVersion != testVersionOther {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, testVersionOther)
	}
}
