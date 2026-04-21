package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// ghPRViewStubScript is a `gh` stub that handles auth + pr view for add-pr tests.
// PR metadata is read from $GH_STUB_DIR/pr_view_<num>.json.
const ghPRViewStubScript = `#!/bin/sh
case "$1" in
  auth)
    if [ -f "$GH_STUB_DIR/unauth" ]; then
      echo "not logged in" >&2
      exit 1
    fi
    echo "Logged in"
    exit 0
    ;;
  pr)
    if [ "$2" = "view" ]; then
      num="$3"
      f="$GH_STUB_DIR/pr_view_$num.json"
      if [ -f "$f" ]; then
        cat "$f"
        exit 0
      fi
      echo "PR not found" >&2
      exit 1
    fi
    ;;
esac
echo "unexpected gh invocation: $*" >&2
exit 2
`

// stubGhPRView installs the PR-view-capable gh stub and returns dir + env.
func stubGhPRView(t *testing.T) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(ghPRViewStubScript), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	env = []string{"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")}
	return dir, env
}

// writeStubPRView writes canned JSON for a specific PR number.
func writeStubPRView(t *testing.T, stubDir string, num int, content string) {
	t.Helper()
	p := filepath.Join(stubDir, "pr_view_"+itoa(num)+".json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write pr_view stub: %v", err)
	}
}

// itoa converts an int to a string without importing strconv at file scope.
func itoa(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// setupRepoWithRemoteAndBranch creates a bare origin, a clone with rimba
// initialised, and a named branch pushed to origin (simulating a PR head ref).
func setupRepoWithRemoteAndBranch(t *testing.T, branchName string) string {
	t.Helper()

	dir := t.TempDir()

	bareDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareDir, "init", "--bare", "-b", "main")

	repo := filepath.Join(dir, "repo")
	cmd := exec.Command("git", "clone", bareDir, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}
	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")

	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "main")

	// Create the PR head branch and push to origin.
	testutil.GitCmd(t, repo, "checkout", "-b", branchName)
	testutil.CreateFile(t, repo, branchName+".txt", "pr content\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "pr commit")
	testutil.GitCmd(t, repo, "push", "origin", branchName)
	testutil.GitCmd(t, repo, "checkout", "main")

	rimbaSuccess(t, repo, "init")

	return repo
}

func TestAddPRWorktreeSameRepo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepoWithRemoteAndBranch(t, "fix-login-redirect")

	stubDir, env := stubGhPRView(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	writeStubPRView(t, stubDir, 42, `{
		"number": 42,
		"title": "Fix login redirect",
		"headRefName": "fix-login-redirect",
		"headRepository": {"name": "rimba"},
		"headRepositoryOwner": {"login": "lugassawan"},
		"isCrossRepository": false
	}`)

	r := rimbaWithEnv(t, repo, env, "add", "pr:42", "--skip-deps", "--skip-hooks")
	if r.ExitCode != 0 {
		t.Fatalf("exit=%d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	assertContains(t, r.Stdout, "Created worktree for PR #42")
	assertContains(t, r.Stdout, "review/42-fix-login-redirect")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := "review/42-fix-login-redirect"
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)
}

func TestAddPRWorktreeTaskOverride(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepoWithRemoteAndBranch(t, "fix-login-redirect")

	stubDir, env := stubGhPRView(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	writeStubPRView(t, stubDir, 42, `{
		"number": 42,
		"title": "Fix login redirect",
		"headRefName": "fix-login-redirect",
		"headRepository": {"name": "rimba"},
		"headRepositoryOwner": {"login": "lugassawan"},
		"isCrossRepository": false
	}`)

	r := rimbaWithEnv(t, repo, env, "add", "pr:42", "--task", "my-review", "--skip-deps", "--skip-hooks")
	if r.ExitCode != 0 {
		t.Fatalf("exit=%d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	assertContains(t, r.Stdout, "my-review")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	wtPath := resolver.WorktreePath(wtDir, "my-review")
	assertFileExists(t, wtPath)
}

func TestAddPRWorktreeGhUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	stubDir, env := stubGhPRView(t)
	env = append(env, "GH_STUB_DIR="+stubDir)

	// Create the "unauth" file to make the stub fail auth status.
	if err := os.WriteFile(filepath.Join(stubDir, "unauth"), []byte(""), 0o644); err != nil {
		t.Fatalf("write unauth: %v", err)
	}

	r := rimbaWithEnv(t, repo, env, "add", "pr:42", "--skip-deps", "--skip-hooks")
	if r.ExitCode == 0 {
		t.Fatal("expected non-zero exit when gh is not authenticated")
	}
	if !strings.Contains(r.Stderr, "gh auth login") {
		t.Errorf("expected 'gh auth login' hint in error, got:\n%s", r.Stderr)
	}
}

func TestAddPRWorktreeCrossFork(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir()

	// Set up main bare origin + clone.
	bareOriginDir := filepath.Join(dir, "origin.git")
	if err := os.MkdirAll(bareOriginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareOriginDir, "init", "--bare", "-b", "main")

	repo := filepath.Join(dir, "repo")
	cloneCmd := exec.Command("git", "clone", bareOriginDir, repo)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", out, err)
	}
	testutil.GitCmd(t, repo, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, repo, "config", "user.name", "Test")
	testutil.CreateFile(t, repo, "README.md", "# Test\n")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "initial commit")
	testutil.GitCmd(t, repo, "push", "-u", "origin", "main")
	rimbaSuccess(t, repo, "init")

	// Set up a bare "fork" repo with feat-oauth branch.
	bareForkDir := filepath.Join(dir, "fork.git")
	if err := os.MkdirAll(bareForkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitBare(t, bareForkDir, "init", "--bare", "-b", "main")
	// Clone the fork, add a branch, push it.
	forkClone := filepath.Join(dir, "fork-clone")
	cloneFork := exec.Command("git", "clone", bareForkDir, forkClone)
	if out, err := cloneFork.CombinedOutput(); err != nil {
		t.Fatalf("clone fork: %s: %v", out, err)
	}
	testutil.GitCmd(t, forkClone, "config", "user.email", "test@test.com")
	testutil.GitCmd(t, forkClone, "config", "user.name", "Test")
	testutil.CreateFile(t, forkClone, "README.md", "# Fork\n")
	testutil.GitCmd(t, forkClone, "add", ".")
	testutil.GitCmd(t, forkClone, "commit", "-m", "fork initial")
	testutil.GitCmd(t, forkClone, "push", "-u", "origin", "main")
	testutil.GitCmd(t, forkClone, "checkout", "-b", "feat-oauth")
	testutil.CreateFile(t, forkClone, "oauth.txt", "oauth\n")
	testutil.GitCmd(t, forkClone, "add", ".")
	testutil.GitCmd(t, forkClone, "commit", "-m", "add oauth")
	testutil.GitCmd(t, forkClone, "push", "origin", "feat-oauth")

	// Redirect the exact fork remote URL to the local bare fork via url.insteadOf.
	// The remote URL AddPRWorktree constructs is "https://github.com/<owner>/<repo>.git".
	forkURL := "file://" + bareForkDir
	githubExact := "https://github.com/test-fork/rimba.git"
	testutil.GitCmd(t, repo, "config", "url."+forkURL+".insteadOf", githubExact)

	stubDir, env := stubGhPRView(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	writeStubPRView(t, stubDir, 99, `{
		"number": 99,
		"title": "Add OAuth support",
		"headRefName": "feat-oauth",
		"headRepository": {"name": "rimba"},
		"headRepositoryOwner": {"login": "test-fork"},
		"isCrossRepository": true
	}`)

	r := rimbaWithEnv(t, repo, env, "add", "pr:99", "--skip-deps", "--skip-hooks")
	if r.ExitCode != 0 {
		t.Fatalf("exit=%d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	assertContains(t, r.Stdout, "Created worktree for PR #99")
	assertContains(t, r.Stdout, "review/99-add-oauth-support")

	// Verify fork remote was added.
	remoteOut, err := exec.Command("git", "-C", repo, "remote", "-v").Output()
	if err != nil {
		t.Fatalf("git remote -v: %v", err)
	}
	if !strings.Contains(string(remoteOut), "gh-fork-test-fork") {
		t.Errorf("expected gh-fork-test-fork remote, got:\n%s", remoteOut)
	}

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	wtPath := resolver.WorktreePath(wtDir, "review/99-add-oauth-support")
	assertFileExists(t, wtPath)
}
