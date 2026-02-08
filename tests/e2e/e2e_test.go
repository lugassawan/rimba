package e2e_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/testutil"
)

const (
	skipE2E       = "skipping e2e test"
	defaultPrefix = "feature/"
	bugfixPrefix  = "bugfix/"
	configFile    = ".rimba.toml"
	gitignoreFile = ".gitignore"

	// Output messages asserted in tests.
	msgCreatedWorktree = "Created worktree"
	msgRemovedWorktree = "Removed worktree"
	msgDeletedBranch   = "Deleted branch"

	// Flags reused across tests.
	flagInto = "--into"

	// Task names reused across tests.
	taskRm         = "rm-task"
	taskRmBranch   = "rm-branch"
	taskKeepBranch = "keep-branch"
	taskFix123     = "fix-123"
	taskForce      = "force-task"
	taskList       = "list-task"
	taskDotfile    = "dotfile-test"
	task1          = "task-1"
	task2          = "task-2"
	taskRmPartial  = "rm-partial-task"
	taskDirtyOne   = "dirty-one"

	// Duplicate test constants.
	taskDupA      = "task-a"
	taskDupB      = "task-b"
	taskMyCopy    = "my-copy"
	secretContent = "SECRET=test"

	// Merge test constants.
	taskMergeMain   = "merge-main"
	taskMergeKeep   = "merge-keep"
	taskMergeSrc    = "merge-src"
	taskMergeDelSrc = "merge-del-src"

	// Shared test data.
	fileFromMain    = "from-main.txt"
	contentMainChg  = "main change"
	commitUpdateMsg = "update main"

	// Shared flags.
	flagMergedE2E  = "--merged"
	flagForceE2E   = "--force"
	flagDryRunE2E  = "--dry-run"
)

var (
	binaryPath string
	coverDir   string
)

func TestMain(m *testing.M) {
	// Find the project root (two levels up from tests/e2e/)
	projRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve project root: %v\n", err)
		os.Exit(1)
	}

	// Create a temp directory for the binary
	binDir, err := os.MkdirTemp("", "rimba-e2e-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(binDir, "rimba")

	// Create coverage directory
	coverDir, err = os.MkdirTemp("", "rimba-e2e-cover-*")
	if err != nil {
		os.RemoveAll(binDir)
		fmt.Fprintf(os.Stderr, "failed to create cover dir: %v\n", err)
		os.Exit(1)
	}

	// Build the binary with coverage instrumentation
	build := exec.Command("go", "build", "-cover", "-o", binaryPath, ".")
	build.Dir = projRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build rimba binary:\n%s\n%v\n", out, err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Merge coverage data
	coverOut := filepath.Join(projRoot, "coverage-e2e.out")
	merge := exec.Command("go", "tool", "covdata", "textfmt", "-i="+coverDir, "-o="+coverOut)
	if out, err := merge.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "coverage merge (non-fatal): %s: %v\n", out, err)
	} else {
		fmt.Fprintf(os.Stdout, "E2E coverage written to %s\n", coverOut)
	}

	os.RemoveAll(binDir)
	os.RemoveAll(coverDir)
	os.Exit(code)
}

// result holds the captured output and exit code from a command invocation.
type result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// rimba runs the compiled binary with the given arguments in the specified directory.
func rimba(t *testing.T, dir string, args ...string) result {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir, "NO_COLOR=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	r := result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run rimba: %v", err)
		}
	}

	return r
}

// rimbaSuccess runs the binary and fails the test if the exit code is not 0.
func rimbaSuccess(t *testing.T, dir string, args ...string) result {
	t.Helper()
	r := rimba(t, dir, args...)
	if r.ExitCode != 0 {
		t.Fatalf("rimba %v: expected exit 0, got %d\nstdout: %s\nstderr: %s",
			args, r.ExitCode, r.Stdout, r.Stderr)
	}
	return r
}

// rimbaFail runs the binary and fails the test if the exit code is 0.
func rimbaFail(t *testing.T, dir string, args ...string) result {
	t.Helper()
	r := rimba(t, dir, args...)
	if r.ExitCode == 0 {
		t.Fatalf("rimba %v: expected non-zero exit, got 0\nstdout: %s\nstderr: %s",
			args, r.Stdout, r.Stderr)
	}
	return r
}

// setupRepo creates a fresh temporary git repository.
func setupRepo(t *testing.T) string {
	t.Helper()
	return testutil.NewTestRepo(t)
}

// setupInitializedRepo creates a temp git repo and runs `rimba init`.
func setupInitializedRepo(t *testing.T) string {
	t.Helper()
	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")
	return repo
}

// assertContains fails the test if s does not contain substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

// assertNotContains fails the test if s contains substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", substr, s)
	}
}

// assertFileExists fails the test if the path does not exist.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// assertFileNotExists fails the test if the path exists.
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file NOT to exist: %s", path)
	}
}
