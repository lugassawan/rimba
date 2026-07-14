package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/testutil"
)

func mockCommonDirRunner(commonDir string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return commonDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

// writeLockFile creates a lock backdated well past the doctor --fix age
// gate, matching the common case in these tests: a lock left by a process
// that's clearly no longer running. Use writeFreshLockFile for the opposite.
func writeLockFile(t *testing.T, commonDir string) string {
	t.Helper()
	return writeLockFileWithAge(t, commonDir, time.Hour)
}

// writeFreshLockFile creates a lock too young for doctor --fix to touch,
// simulating one that may still belong to a running git process.
func writeFreshLockFile(t *testing.T, commonDir string) string {
	t.Helper()
	return writeLockFileWithAge(t, commonDir, 0)
}

func writeLockFileWithAge(t *testing.T, commonDir string, age time.Duration) string {
	t.Helper()
	lockDir := filepath.Join(commonDir, "worktrees", "wt-a")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := filepath.Join(lockDir, "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if age > 0 {
		old := time.Now().Add(-age)
		if err := os.Chtimes(lockPath, old, old); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
	}
	return lockPath
}

func TestDoctorNoLocks(t *testing.T) {
	commonDir := t.TempDir()
	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No stale index.lock files found.") {
		t.Errorf("output = %q, want no-locks message", buf.String())
	}
}

func TestDoctorReportListsPathAndAge(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFile(t, commonDir)
	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, lockPath) {
		t.Errorf("output = %q, want path %q", out, lockPath)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock file to remain (report-only)")
	}
}

// TestDoctorFixSkipsFreshLocks guards against --fix --force removing a lock
// that may still belong to a running git process: a lock younger than the
// age gate must be skipped even when --force bypasses the confirmation
// prompt.
func TestDoctorFixSkipsFreshLocks(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeFreshLockFile(t, commonDir)
	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Skipping") {
		t.Errorf("output = %q, want a skip notice for a too-young lock", out)
	}
	if strings.Contains(out, "Removed "+lockPath) {
		t.Errorf("output = %q, want the fresh lock NOT removed even under --force", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected fresh lock file to remain")
	}
}

func TestDoctorFixForceRemoves(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFile(t, commonDir)
	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Warning:") {
		t.Errorf("output = %q, want a live-lock warning", out)
	}
	if !strings.Contains(out, "Removed "+lockPath) {
		t.Errorf("output = %q, want removal confirmation for %q", out, lockPath)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed")
	}
}

func TestDoctorFixDeclinedKeepsFiles(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFile(t, commonDir)
	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, false, "")
	cmd.SetIn(strings.NewReader("n\n"))

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Aborted") {
		t.Errorf("output = %q, want 'Aborted'", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock file to remain when --fix is declined")
	}
}

func TestDoctorFixReportsRemovalFailure(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFile(t, commonDir)
	lockDir := filepath.Dir(lockPath)

	// Stripping write on the containing directory makes the file itself
	// unremovable regardless of its own permissions.
	if err := os.Chmod(lockDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockDir, 0o755) })

	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Failed to remove") {
		t.Errorf("output = %q, want a removal-failure message", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock file to remain after a failed removal")
	}
}

func TestDoctorScanWorktreeLocksError(t *testing.T) {
	// An unmatched '[' makes the glob pattern malformed.
	restore := overrideNewRunner(mockCommonDirRunner(filepath.Join(t.TempDir(), "unmatched[bracket")))
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error when ScanWorktreeLocks fails")
	}
}

func TestDoctorCommonDirError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error when CommonDir resolution fails")
	}
}

// TestDoctorThreeWaySplit exercises all three buckets in one run: dead
// owner (auto-recovered), alive owner (skipped), and markerless (age-based).
func TestDoctorThreeWaySplit(t *testing.T) {
	commonDir := t.TempDir()

	deadLock := writeLockFileWithAge(t, commonDir, operations.MinLockAge+time.Second)
	plantSweepManifest(t, commonDir, testutil.DeadPID(t), []string{filepath.Dir(deadLock)})

	aliveLockDir := filepath.Join(commonDir, "worktrees", "wt-alive")
	if err := os.MkdirAll(aliveLockDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	aliveLock := filepath.Join(aliveLockDir, "index.lock")
	if err := os.WriteFile(aliveLock, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	old := time.Now().Add(-(operations.MinLockAge + time.Second))
	if err := os.Chtimes(aliveLock, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	plantSweepManifest(t, commonDir, os.Getpid(), []string{aliveLockDir})

	markerlessLock := filepath.Join(commonDir, "worktrees", "wt-markerless", "index.lock")
	if err := os.MkdirAll(filepath.Dir(markerlessLock), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(markerlessLock, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(markerlessLock, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Recovered 1 stale index.lock file(s)") || !strings.Contains(out, deadLock) {
		t.Errorf("output = %q, want the dead-owner lock reported as recovered", out)
	}
	if _, err := os.Stat(deadLock); !os.IsNotExist(err) {
		t.Error("expected the dead-owner lock to be removed")
	}

	if !strings.Contains(out, "Skipping "+aliveLock+": owned by a sweep that is still running.") {
		t.Errorf("output = %q, want the alive-owner lock reported as skipped", out)
	}
	if _, err := os.Stat(aliveLock); err != nil {
		t.Error("expected the alive-owner lock to remain even under --fix --force")
	}

	if !strings.Contains(out, "Removed "+markerlessLock) {
		t.Errorf("output = %q, want the markerless lock removed via the normal --fix flow", out)
	}
	if _, err := os.Stat(markerlessLock); !os.IsNotExist(err) {
		t.Error("expected the markerless lock to be removed")
	}
}

// TestDoctorFixRecoversAliveMarkerPastCeiling guards the Windows/PID-reuse
// escape hatch: an "alive" manifest past the ceiling falls back to --fix.
func TestDoctorFixRecoversAliveMarkerPastCeiling(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFileWithAge(t, commonDir, operations.MinLockAge+time.Second)
	adminDir := filepath.Dir(lockPath)
	// Comfortably past operations.aliveMarkerCeiling (unexported, so a generous margin).
	ancientStart := time.Now().Add(-time.Hour).UnixNano()
	plantSweepManifestWithStart(t, commonDir, os.Getpid(), []string{adminDir}, ancientStart)

	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "owned by a sweep that is still running") {
		t.Errorf("output = %q, want the past-ceiling manifest treated as markerless, not skipped", out)
	}
	if !strings.Contains(out, "Removed "+lockPath) {
		t.Errorf("output = %q, want the lock removed via the normal --fix flow", out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected the lock to be removed once its manifest is past the alive-marker ceiling")
	}
}

// setupInterruptedWorktree creates a real worktree directory at <root>/wt-<name>
// whose `.git` pointer file resolves to an admin dir under commonDir,
// mirroring `git worktree add`'s on-disk layout closely enough for
// ScanInterruptedWorktrees's filesystem-based admin-dir lookup.
func setupInterruptedWorktree(t *testing.T, root, commonDir, name string) string {
	t.Helper()
	wtPath := filepath.Join(root, "wt-"+name)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("MkdirAll worktree: %v", err)
	}
	adminDir := filepath.Join(commonDir, "worktrees", name)
	if err := os.MkdirAll(adminDir, 0o755); err != nil {
		t.Fatalf("MkdirAll admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".git"), []byte("gitdir: "+adminDir+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .git: %v", err)
	}
	return wtPath
}

// interruptedWorktreePorcelain builds a single-entry `git worktree list
// --porcelain` block for path/branch.
func interruptedWorktreePorcelain(path, branch string) string {
	return "worktree " + path + "\nHEAD abc123\nbranch refs/heads/" + branch + "\n"
}

// mockInterruptedRunner resolves --git-common-dir to commonDir, `worktree
// list --porcelain` to worktreeListOut, records any `worktree remove`
// invocation into removeCalls, and answers every RunInDir status check with
// statusOut.
func mockInterruptedRunner(commonDir, worktreeListOut, statusOut string, removeCalls *[][]string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return commonDir, nil
			case len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList:
				return worktreeListOut, nil
			case len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdRemove:
				*removeCalls = append(*removeCalls, append([]string(nil), args...))
				return "", nil
			}
			return "", nil
		},
		runInDir: func(_ string, _ ...string) (string, error) {
			return statusOut, nil
		},
	}
}

// TestDoctorReportsInterruptedWorktree covers the no-`--fix` path: an
// interrupted worktree is listed with its path/branch/deleted-count and the
// remove/doctor-fix hint, and no `worktree remove` is ever issued.
func TestDoctorReportsInterruptedWorktree(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt := setupInterruptedWorktree(t, root, commonDir, "wt-a")

	var removeCalls [][]string
	r := mockInterruptedRunner(commonDir, interruptedWorktreePorcelain(wt, branchFeature), " D a.txt\n", &removeCalls)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, wt) {
		t.Errorf("output = %q, want the interrupted worktree path %q", out, wt)
	}
	if !strings.Contains(out, "rimba remove <task> --force") {
		t.Errorf("output = %q, want the remove --force hint", out)
	}
	if len(removeCalls) != 0 {
		t.Errorf("removeCalls = %+v, want no worktree remove call issued", removeCalls)
	}
}

// TestDoctorFixFinishesInterruptedWorktree covers `--fix --force`: the
// worktree is removed via `git worktree remove --force -- <path>` and the
// outcome is printed.
func TestDoctorFixFinishesInterruptedWorktree(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt := setupInterruptedWorktree(t, root, commonDir, "wt-a")

	var removeCalls [][]string
	r := mockInterruptedRunner(commonDir, interruptedWorktreePorcelain(wt, branchFeature), " D a.txt\n", &removeCalls)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, true, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removed "+wt) {
		t.Errorf("output = %q, want removal confirmation for %q", out, wt)
	}
	if len(removeCalls) != 1 {
		t.Fatalf("removeCalls = %+v, want exactly 1 worktree remove call", removeCalls)
	}
	want := []string{"worktree", "remove", "--force", "--", wt}
	if !reflect.DeepEqual(removeCalls[0], want) {
		t.Errorf("removeCalls[0] = %v, want %v", removeCalls[0], want)
	}
}

// TestDoctorFixDeclinedInterruptedWorktree covers the declined-confirm path:
// answering "n" to the prompt aborts without issuing any worktree removal.
func TestDoctorFixDeclinedInterruptedWorktree(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt := setupInterruptedWorktree(t, root, commonDir, "wt-a")

	var removeCalls [][]string
	r := mockInterruptedRunner(commonDir, interruptedWorktreePorcelain(wt, branchFeature), " D a.txt\n", &removeCalls)
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, true, "")
	cmd.Flags().Bool(flagForce, false, "")
	cmd.SetIn(strings.NewReader("n\n"))

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Aborted.") {
		t.Errorf("output = %q, want 'Aborted.'", out)
	}
	if len(removeCalls) != 0 {
		t.Errorf("removeCalls = %+v, want no worktree remove call issued", removeCalls)
	}
}

func TestDoctorReportsConfidentReapRemovalFailure(t *testing.T) {
	commonDir := t.TempDir()
	lockPath := writeLockFileWithAge(t, commonDir, operations.MinLockAge+time.Second)
	lockDir := filepath.Dir(lockPath)
	plantSweepManifest(t, commonDir, testutil.DeadPID(t), []string{lockDir})

	// Stripping write on the containing directory makes the file itself
	// unremovable regardless of its own permissions.
	if err := os.Chmod(lockDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockDir, 0o755) })

	restore := overrideNewRunner(mockCommonDirRunner(commonDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagFix, false, "")
	cmd.Flags().Bool(flagForce, false, "")

	if err := doctorCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctorCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed to remove") {
		t.Errorf("output = %q, want a removal-failure notice for the confident reap", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain after a failed removal")
	}
}
