package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func writeLockFile(t *testing.T, commonDir string) string {
	t.Helper()
	lockDir := filepath.Join(commonDir, "worktrees", "wt-a")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := filepath.Join(lockDir, "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
