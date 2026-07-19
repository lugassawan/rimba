package deps

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSameDeviceSamePath(t *testing.T) {
	dir := t.TempDir()
	same, ok := sameDevice(dir, dir)
	if runtime.GOOS == goosWindows {
		if ok {
			t.Error("expected ok=false on windows (no comparable device id)")
		}
		return
	}
	if !ok {
		t.Fatal("expected ok=true for two existing paths")
	}
	if !same {
		t.Error("expected same=true: a path compared with itself is always on the same device")
	}
}

func TestSameDeviceNonexistentPath(t *testing.T) {
	dir := t.TempDir()
	_, ok := sameDevice(filepath.Join(dir, "does-not-exist"), dir)
	if ok {
		t.Error("expected ok=false when one path doesn't exist")
	}
}

func TestProbeCowCapableUnsupportedDir(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	if probeCowCapable(context.Background(), dir) {
		t.Error("expected false when the temp file can't even be created")
	}
}

func TestProbeCowCapableInjectedSuccess(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}
	orig := cowProbeCmd
	cowProbeCmd = func(ctx context.Context, src, dst string) *exec.Cmd { return exec.CommandContext(ctx, "true") }
	t.Cleanup(func() { cowProbeCmd = orig })

	if !probeCowCapable(context.Background(), t.TempDir()) {
		t.Error("expected true when the probe command succeeds")
	}
}

func TestProbeCowCapableInjectedFailure(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}
	orig := cowProbeCmd
	cowProbeCmd = func(ctx context.Context, src, dst string) *exec.Cmd { return exec.CommandContext(ctx, "false") }
	t.Cleanup(func() { cowProbeCmd = orig })

	if probeCowCapable(context.Background(), t.TempDir()) {
		t.Error("expected false when the probe command fails")
	}
}

func TestProbeCowCapableUnsupportedOS(t *testing.T) {
	if probeCowCapableSupportedOS() {
		t.Skip("only meaningful on a platform where the probe is a no-op")
	}
	if probeCowCapable(context.Background(), t.TempDir()) {
		t.Error("expected false on a platform with no CoW probe")
	}
}

func probeCowCapableSupportedOS() bool {
	return runtime.GOOS == goosDarwin || runtime.GOOS == goosLinux
}

// TestCowEligibleRealImplementation exercises the real (non-overridden)
// cowEligible closure end-to-end: same-device src/dst plus an injected
// always-succeeding probe must report eligible, and the result must be
// cached (the injected command is only ever invoked once per dstDir).
func TestCowEligibleRealImplementation(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}

	var invocations int
	orig := cowProbeCmd
	cowProbeCmd = func(ctx context.Context, src, dst string) *exec.Cmd {
		invocations++
		return exec.CommandContext(ctx, "true")
	}
	t.Cleanup(func() { cowProbeCmd = orig })

	dstDir := t.TempDir()
	t.Cleanup(func() { cowProbeCache.Delete(dstDir) })

	if !cowEligible(context.Background(), dstDir, dstDir) {
		t.Fatal("expected eligible=true: same path, injected probe succeeds")
	}
	if !cowEligible(context.Background(), dstDir, dstDir) {
		t.Fatal("expected eligible=true on second call (cached)")
	}
	if invocations != 1 {
		t.Errorf("probe invoked %d times, want 1 (result should be cached per dstDir)", invocations)
	}
}

func TestCowEligibleOverrideEnv(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{name: "forced true", val: "1", want: true},
		{name: "forced false", val: "0", want: false},
		{name: "unrecognized value treated as false", val: "yes", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(cowEligibleOverrideEnv, tt.val)
			// Nonexistent src would normally make sameDevice fail (ineligible)
			// regardless — proves the override short-circuits before that check.
			dir := t.TempDir()
			if got := cowEligible(context.Background(), filepath.Join(dir, "nonexistent-src"), dir); got != tt.want {
				t.Errorf("cowEligible() with override=%q = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestCowEligibleDifferentDevice(t *testing.T) {
	dir := t.TempDir()
	if got := cowEligible(context.Background(), filepath.Join(dir, "nonexistent-src"), dir); got {
		t.Error("expected eligible=false when src doesn't exist (device can't be determined)")
	}
}

func TestCowEligibleProbeFailureIsNotEligible(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}
	orig := cowProbeCmd
	cowProbeCmd = func(ctx context.Context, src, dst string) *exec.Cmd { return exec.CommandContext(ctx, "false") }
	t.Cleanup(func() { cowProbeCmd = orig })

	dstDir := t.TempDir()
	t.Cleanup(func() { cowProbeCache.Delete(dstDir) })

	if cowEligible(context.Background(), dstDir, dstDir) {
		t.Error("expected eligible=false when the underlying probe fails")
	}
}

func TestCowProbeCmdDefaultsFailClosed(t *testing.T) {
	// On any unrecognized GOOS the probe command must fail rather than
	// silently succeed, matching probeCowCapable's own OS gate.
	if probeCowCapableSupportedOS() {
		t.Skip("only meaningful off the recognized darwin/linux probe path")
	}
	cmd := cowProbeCmd(context.Background(), "src", "dst")
	if err := cmd.Run(); err == nil {
		t.Error("expected the default probe command to fail")
	}
}

func TestProbeCowCapableCleansUpTempFiles(t *testing.T) {
	if !probeCowCapableSupportedOS() {
		t.Skip("probe only runs on darwin/linux")
	}
	dir := t.TempDir()
	probeCowCapable(context.Background(), dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected probe temp files to be cleaned up, found %d leftover entries", len(entries))
	}
}
