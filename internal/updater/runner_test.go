package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/spinner"
)

const (
	testUserDir       = "/home/user/.local/bin"
	testCurrentBinary = "/usr/local/bin/rimba"
)

// fakeDownloadResult is the DownloadResult returned by the default test download seam.
var fakeDownloadResult = &DownloadResult{
	ArchivePath: "/tmp/rimba-test/archive.tar.gz",
	BinaryPath:  "/tmp/rimba-test/rimba",
	SHA256:      "aabbcc",
}

// newTestRunner returns a Runner wired with fake seams that all succeed, plus a
// buffer capturing stdout. Tests override individual seams to exercise one failure path.
func newTestRunner(t *testing.T) (*Runner, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	r := &Runner{
		Version: "1.0.0",
		Out:     &buf,
		Spinner: spinner.New(spinner.Options{Writer: io.Discard}),
	}
	r.check = func(_ context.Context) (*CheckResult, error) {
		return &CheckResult{
			CurrentVersion: "1.0.0",
			LatestVersion:  "v1.1.0",
			DownloadURL:    "http://fake/download",
			AssetName:      "rimba_1.1.0_linux_amd64.tar.gz",
			ChecksumsURL:   "http://fake/checksums.txt",
		}, nil
	}
	r.download = func(_ context.Context, url string) (*DownloadResult, error) {
		return fakeDownloadResult, nil
	}
	r.verifyChecksum = func(_ context.Context, _ *CheckResult, _ *DownloadResult) error { return nil }
	r.prepareBinary = func(path string) error { return nil }
	r.executable = func() (string, error) { return testCurrentBinary, nil }
	r.evalSymlinks = func(path string) (string, error) { return path, nil }
	r.replace = func(_, _ string) error { return nil }
	r.userInstallDir = func() (string, error) { return testUserDir, nil }
	r.mkdirAll = func(path string, perm os.FileMode) error { return nil }
	r.stat = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	r.readFile = func(path string) ([]byte, error) { return []byte("fake"), nil }
	r.writeFile = func(path string, data []byte, perm os.FileMode) error { return nil }
	r.ensurePath = func(dir string) error { return nil }
	r.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("rimba version 1.1.0"), nil
	}
	r.cleanupTempDir = func(binaryPath string) {}
	return r, &buf
}

// requireErrContains fails if err is nil or does not contain every fragment.
func requireErrContains(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	got := err.Error()
	for _, frag := range fragments {
		if !strings.Contains(got, frag) {
			t.Errorf("error = %q, want to contain %q", got, frag)
		}
	}
}

// testPermErr returns a wrapped os.ErrPermission to route the replace fallback path.
func testPermErr() error {
	return fmt.Errorf("writing to /system/bin: %w", os.ErrPermission)
}

// statExists returns a stat func that reports the path as existing (takes the replace branch).
func statExists(t *testing.T) func(string) (os.FileInfo, error) {
	t.Helper()
	info, err := os.Stat(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	return func(path string) (os.FileInfo, error) { return info, nil }
}

// TestHintSurfaces verifies all errhint.WithFix surfaces in Runner.Run.
// Each test case starts from a fully-mocked-success runner and overrides exactly
// one seam to fail, then asserts the wrapped error prefix and hint fragment.
func TestHintSurfaces(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(*testing.T, *Runner)
		wantPrefix string
		wantHint   string
	}{
		{
			name: "check fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.check = func(_ context.Context) (*CheckResult, error) {
					return nil, errors.New("network error")
				}
			},
			wantPrefix: "checking for updates:",
			wantHint:   "check network connectivity, or set GITHUB_TOKEN if rate limited",
		},
		{
			name: "download fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.download = func(_ context.Context, url string) (*DownloadResult, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantPrefix: "downloading update:",
			wantHint:   "check network connectivity and retry: rimba update",
		},
		{
			name: "verifyChecksum fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.verifyChecksum = func(_ context.Context, _ *CheckResult, _ *DownloadResult) error {
					return errors.New("checksum mismatch: got aaa, want bbb")
				}
			},
			wantPrefix: "integrity check failed:",
			wantHint:   "the downloaded release failed integrity verification",
		},
		{
			name: "prepareBinary fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.prepareBinary = func(path string) error { return errors.New("codesign failed") }
			},
			wantPrefix: "preparing binary:",
			wantHint:   retryUpdateHint,
		},
		{
			name: "executable fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.executable = func() (string, error) { return "", errors.New("no executable") }
			},
			wantPrefix: "locating current binary:",
			wantHint:   reinstallHint,
		},
		{
			name: "evalSymlinks fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.evalSymlinks = func(path string) (string, error) {
					return "", errors.New("dangling symlink")
				}
			},
			wantPrefix: "resolving binary path:",
			wantHint:   reinstallHint,
		},
		{
			name: "replace fails non-permission",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return errors.New("disk full") }
			},
			wantPrefix: "replacing binary:",
			wantHint:   retryUpdateHint,
		},
		{
			name: "replace perm: userInstallDir fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return testPermErr() }
				r.userInstallDir = func() (string, error) { return "", errors.New("no home dir") }
			},
			wantPrefix: "getting user install dir:",
			wantHint:   "set HOME to your user home directory and retry: rimba update",
		},
		{
			name: "replace perm: mkdirAll fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return testPermErr() }
				r.mkdirAll = func(path string, perm os.FileMode) error {
					return errors.New("permission denied")
				}
			},
			wantPrefix: "creating install dir:",
			wantHint:   "check write permissions for ~/.local/bin and retry: rimba update",
		},
		{
			name: "replace perm: stat NotExist, readFile fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return testPermErr() }
				r.readFile = func(path string) ([]byte, error) { return nil, errors.New("read error") }
			},
			wantPrefix: "reading new binary:",
			wantHint:   retryUpdateHint,
		},
		{
			name: "replace perm: stat NotExist, writeFile fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return testPermErr() }
				r.writeFile = func(path string, data []byte, perm os.FileMode) error {
					return errors.New("write error")
				}
			},
			wantPrefix: "writing binary:",
			wantHint:   "check write permissions for " + testUserDir + " and retry: rimba update",
		},
		{
			name: "replace perm: stat exists, second replace fails",
			setupFn: func(t *testing.T, r *Runner) {
				t.Helper()
				n := 0
				r.replace = func(_, _ string) error {
					n++
					if n == 1 {
						return testPermErr()
					}
					return errors.New("replace failed")
				}
				r.stat = statExists(t)
			},
			wantPrefix: "replacing binary:",
			wantHint:   "check write permissions for " + testUserDir + " and retry: rimba update",
		},
		{
			name: "replace perm: ensurePath fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.replace = func(_, _ string) error { return testPermErr() }
				r.ensurePath = func(dir string) error { return errors.New("cannot write rc file") }
			},
			wantPrefix: "updating PATH:",
			wantHint:   `add ` + testUserDir + ` to PATH manually: export PATH="` + testUserDir + `:$PATH"`,
		},
		{
			name: "execCommand fails",
			setupFn: func(_ *testing.T, r *Runner) {
				r.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
					return nil, errors.New("exec failed")
				}
			},
			wantPrefix: "verifying new binary:",
			wantHint: "the new binary at " + testCurrentBinary +
				" may be corrupt — retry: rimba update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestRunner(t)
			tt.setupFn(t, r)
			err := r.Run(context.Background())
			requireErrContains(t, err, "To fix:", tt.wantPrefix, tt.wantHint)
		})
	}
}

func TestRunHappyPath(t *testing.T) {
	r, out := newTestRunner(t)
	onSuccessCalled := false
	r.OnSuccess = func() { onSuccessCalled = true }

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Updated successfully:") {
		t.Errorf("output = %q, want 'Updated successfully:'", got)
	}
	if !onSuccessCalled {
		t.Error("OnSuccess should have been called")
	}
	if strings.Contains(got, "To complete migration") {
		t.Errorf("output = %q, migration hint should not appear when installed == current", got)
	}
}

func TestRunUpToDate(t *testing.T) {
	r, out := newTestRunner(t)
	r.check = func(_ context.Context) (*CheckResult, error) {
		return &CheckResult{CurrentVersion: "1.0.0", UpToDate: true}, nil
	}

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Already up to date") {
		t.Errorf("output = %q, want 'Already up to date'", out.String())
	}
}

func TestRunPermFallbackFreshInstall(t *testing.T) {
	r, out := newTestRunner(t)
	r.replace = func(_, _ string) error { return testPermErr() }
	onSuccessCalled := false
	r.OnSuccess = func() { onSuccessCalled = true }

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Cannot write to") {
		t.Errorf("output = %q, want 'Cannot write to'", got)
	}
	if !strings.Contains(got, "To complete migration") {
		t.Errorf("output = %q, want migration hint when installed != current", got)
	}
	if !onSuccessCalled {
		t.Error("OnSuccess should have been called on fallback success path")
	}
}

func TestRunPermFallbackExistingFile(t *testing.T) {
	r, out := newTestRunner(t)
	n := 0
	r.replace = func(_, _ string) error {
		n++
		if n == 1 {
			return testPermErr()
		}
		return nil
	}
	r.stat = statExists(t)
	onSuccessCalled := false
	r.OnSuccess = func() { onSuccessCalled = true }

	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Updated successfully:") {
		t.Errorf("output = %q, want 'Updated successfully:'", out.String())
	}
	if !onSuccessCalled {
		t.Error("OnSuccess should have been called on fallback success path")
	}
}

func TestRunCtxCancelledBeforeInstall(t *testing.T) {
	r, _ := newTestRunner(t)
	replaceCalled := false
	r.replace = func(_, _ string) error {
		replaceCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if replaceCalled {
		t.Error("replace should not be called after ctx cancellation")
	}
}

// TestRunReplaceNotCalledOnChecksumFailure asserts that a checksum mismatch is
// fail-closed: no swap ever reaches the installed binary.
func TestRunReplaceNotCalledOnChecksumFailure(t *testing.T) {
	r, _ := newTestRunner(t)
	replaceCalled := false
	r.replace = func(_, _ string) error {
		replaceCalled = true
		return nil
	}
	r.verifyChecksum = func(_ context.Context, _ *CheckResult, _ *DownloadResult) error {
		return errors.New("checksum mismatch: got aaa, want bbb")
	}

	err := r.Run(context.Background())
	requireErrContains(t, err, "integrity check failed:", "checksum mismatch")
	if replaceCalled {
		t.Error("replace should not be called after checksum failure (fail-closed)")
	}
}

func TestRunVerifyFailsAfterFallbackInstall(t *testing.T) {
	r, _ := newTestRunner(t)
	r.replace = func(_, _ string) error { return testPermErr() }
	r.execCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("exec failed")
	}

	err := r.Run(context.Background())
	requireErrContains(t, err, "To fix:", "verifying new binary:", testUserDir+"/rimba")
}

func TestNewRunner(t *testing.T) {
	r := NewRunner("1.2.3")
	if r.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", r.Version, "1.2.3")
	}
	if r.Out == nil {
		t.Error("Out should not be nil")
	}
	if r.Spinner == nil {
		t.Error("Spinner should not be nil")
	}
}

func TestDefaultExecCommand(t *testing.T) {
	out, err := defaultExecCommand(context.Background(), "echo", "rimba-test")
	if err != nil {
		t.Fatalf("defaultExecCommand: %v", err)
	}
	if !strings.Contains(string(out), "rimba-test") {
		t.Errorf("output = %q, expected to contain 'rimba-test'", string(out))
	}
}

func TestRunNilDefaults(t *testing.T) {
	r, _ := newTestRunner(t)
	r.Spinner = nil
	r.Out = nil
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error with nil Spinner/Out: %v", err)
	}
}
