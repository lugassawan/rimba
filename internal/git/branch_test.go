package git_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/testutil"
)

const (
	skipIntegration  = "skipping integration test"
	fatalAddWorktree = "AddWorktree: %v"
	branchToDelete   = "to-delete"
)

func TestBranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	if !git.BranchExists(context.Background(), r, "main") {
		t.Error("expected main branch to exist")
	}

	if git.BranchExists(context.Background(), r, "nonexistent") {
		t.Error("expected nonexistent branch to not exist")
	}
}

func TestDeleteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a branch
	testutil.GitCmd(t, repo, "branch", branchToDelete)

	if !git.BranchExists(context.Background(), r, branchToDelete) {
		t.Fatal("branch should exist before delete")
	}

	if err := git.DeleteBranch(context.Background(), r, branchToDelete, false); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if git.BranchExists(context.Background(), r, branchToDelete) {
		t.Error("branch should not exist after delete")
	}
}

func TestDeleteBranchLeadingDashName(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// git branch/checkout refuse to create a ref beginning with "-", so use
	// update-ref to create the fixture directly.
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-leading-dash", "HEAD")

	if err := git.DeleteBranch(context.Background(), r, "-leading-dash", false); err != nil {
		t.Fatalf("DeleteBranch on leading-dash name: %v", err)
	}

	if git.BranchExists(context.Background(), r, "-leading-dash") {
		t.Error("branch should not exist after delete")
	}
}

func TestRenameBranchLeadingDashOldName(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-leading-dash-rename", "HEAD")

	if err := git.RenameBranch(context.Background(), r, "-leading-dash-rename", "renamed-ok"); err != nil {
		t.Fatalf("RenameBranch from leading-dash name: %v", err)
	}

	if !git.BranchExists(context.Background(), r, "renamed-ok") {
		t.Error("expected renamed-ok branch to exist")
	}
	if git.BranchExists(context.Background(), r, "-leading-dash-rename") {
		t.Error("expected old leading-dash branch to be gone")
	}
}

func TestLastCommitTime(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	ct, err := git.LastCommitTime(context.Background(), r, "main")
	if err != nil {
		t.Fatalf("LastCommitTime: %v", err)
	}

	// Should be within the last minute (just created)
	if time.Since(ct) > time.Minute {
		t.Errorf("commit time %v is too old", ct)
	}
}

func TestLastCommitInfo(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	ct, subject, err := git.LastCommitInfo(context.Background(), r, "main")
	if err != nil {
		t.Fatalf("LastCommitInfo: %v", err)
	}

	if time.Since(ct) > time.Minute {
		t.Errorf("commit time %v is too old", ct)
	}
	if subject == "" {
		t.Error("expected non-empty commit subject")
	}
}

func TestLastCommitInfoError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	_, _, err := git.LastCommitInfo(context.Background(), r, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestLastCommitInfoLeadingDashBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-lci-dash", "HEAD")

	_, subject, err := git.LastCommitInfo(context.Background(), r, "-lci-dash")
	if err != nil {
		t.Fatalf("LastCommitInfo on leading-dash branch: %v", err)
	}
	if subject == "" {
		t.Error("expected non-empty commit subject")
	}
}

func TestLastCommitTimeError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	_, err := git.LastCommitTime(context.Background(), r, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestLocalBranches(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	branches, err := git.LocalBranches(context.Background(), r)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}

	if len(branches) == 0 {
		t.Fatal("expected at least one branch")
	}

	if !slices.Contains(branches, "main") {
		t.Errorf("expected 'main' in branches, got %v", branches)
	}
}

func TestLocalBranchesMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "branch", "feature/test-branch")

	branches, err := git.LocalBranches(context.Background(), r)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}

	if len(branches) < 2 {
		t.Fatalf("expected at least 2 branches, got %d: %v", len(branches), branches)
	}
}

func TestIsDirty(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	dirty, err := git.IsDirty(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("clean repo should not be dirty")
	}

	// Make it dirty
	testutil.CreateFile(t, repo, "new.txt", "dirty")

	dirty, err = git.IsDirty(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("repo with untracked file should be dirty")
	}
}

func TestMergedBranchesLeadingDashBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-merged-dash", "HEAD")

	branches, err := git.MergedBranches(context.Background(), r, "-merged-dash")
	if err != nil {
		t.Fatalf("MergedBranches on leading-dash branch: %v", err)
	}
	if !slices.Contains(branches, "main") {
		t.Errorf("expected main (same commit) to be merged into -merged-dash, got %v", branches)
	}
}

func TestIsSquashMergedLeadingDashBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "checkout", "-b", "feature-tmp")
	testutil.CreateFile(t, repo, "squash.txt", "squash content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "feature commit")
	tip := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "feature-tmp"))
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-squash-dash", tip)

	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "--squash", tip)
	testutil.GitCmd(t, repo, "commit", "-m", "squash merge feature")

	merged, err := git.IsSquashMerged(context.Background(), r, "main", "-squash-dash")
	if err != nil {
		t.Fatalf("IsSquashMerged on leading-dash branch: %v", err)
	}
	if !merged {
		t.Error("expected leading-dash branch to be detected as squash-merged")
	}
}

func TestIsSquashMergedIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a feature branch with a commit
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/squash-test")
	testutil.CreateFile(t, repo, "squash.txt", "squash content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "feature commit")

	// Go back to main and squash-merge
	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "--squash", "feature/squash-test")
	testutil.GitCmd(t, repo, "commit", "-m", "squash merge feature")

	// The branch should be detected as squash-merged
	merged, err := git.IsSquashMerged(context.Background(), r, "main", "feature/squash-test")
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if !merged {
		t.Error("expected squash-merged branch to be detected")
	}
}

func TestIsSquashMergedIntegrationNotMerged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Create a feature branch with a commit but don't merge it
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/unmerged")
	testutil.CreateFile(t, repo, "unmerged.txt", "unmerged content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "unmerged commit")

	testutil.GitCmd(t, repo, "checkout", "main")

	merged, err := git.IsSquashMerged(context.Background(), r, "main", "feature/unmerged")
	if err != nil {
		t.Fatalf("IsSquashMerged: %v", err)
	}
	if merged {
		t.Error("expected unmerged branch to not be detected as squash-merged")
	}
}

func TestCurrentBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	branch, err := git.CurrentBranch(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "main")
	}
}

func TestCurrentBranchAfterSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "checkout", "-b", "feature/cur-branch-test")

	branch, err := git.CurrentBranch(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("CurrentBranch after switch: %v", err)
	}
	if branch != "feature/cur-branch-test" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "feature/cur-branch-test")
	}
}

func TestCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "branch", "feature/checkout-test")

	if err := git.Checkout(context.Background(), r, repo, "feature/checkout-test"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	branch, err := git.CurrentBranch(context.Background(), r, repo)
	if err != nil {
		t.Fatalf("CurrentBranch after Checkout: %v", err)
	}
	if branch != "feature/checkout-test" {
		t.Errorf("branch after Checkout = %q, want %q", branch, "feature/checkout-test")
	}
}

func TestCheckoutError(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	if err := git.Checkout(context.Background(), r, repo, "nonexistent-branch"); err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestMergeBaseLeadingDashRef(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-merge-base-dash", "HEAD")

	sha, err := git.MergeBase(context.Background(), r, "main", "-merge-base-dash")
	if err != nil {
		t.Fatalf("MergeBase with leading-dash ref: %v", err)
	}
	if sha == "" {
		t.Error("expected non-empty merge-base SHA")
	}
}

func TestIsMergeBaseAncestorLeadingDashRef(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-ancestor-dash", "HEAD")

	if !git.IsMergeBaseAncestor(context.Background(), r, "-ancestor-dash", "main") {
		t.Error("expected leading-dash ref to be recognized as an ancestor of main")
	}
}

func TestFirstParentChainSHAsFreshBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// A branch created from main with no additional commits has its tip on main's mainline.
	testutil.GitCmd(t, repo, "checkout", "-b", "feature/fresh")
	tip := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "feature/fresh"))
	testutil.GitCmd(t, repo, "checkout", "main")

	mainline, err := git.FirstParentChainSHAs(context.Background(), r, "main")
	if err != nil {
		t.Fatalf("FirstParentChainSHAs: %v", err)
	}
	if !git.IsSHAOnChain(tip, mainline) {
		t.Error("expected true for a fresh branch with no own commits")
	}
}

func TestFirstParentChainSHAsMergeCommitMerged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "checkout", "-b", "feature/merge-commit")
	testutil.CreateFile(t, repo, "own.txt", "content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "own commit")
	tip := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "feature/merge-commit"))
	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "--no-ff", "feature/merge-commit", "-m", "merge commit")

	mainline, err := git.FirstParentChainSHAs(context.Background(), r, "main")
	if err != nil {
		t.Fatalf("FirstParentChainSHAs: %v", err)
	}
	if git.IsSHAOnChain(tip, mainline) {
		t.Error("expected false: branch tip is the merge's second parent, off mainline")
	}
}

func TestFirstParentChainSHAsFastForward(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "checkout", "-b", "feature/ff")
	testutil.CreateFile(t, repo, "own.txt", "content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "own commit")
	tip := strings.TrimSpace(testutil.GitCmd(t, repo, "rev-parse", "feature/ff"))
	testutil.GitCmd(t, repo, "checkout", "main")
	testutil.GitCmd(t, repo, "merge", "feature/ff")

	// Accepted false-negative: a fast-forward merge leaves the branch tip on
	// mainline, so clean --merged will not remove it.
	mainline, err := git.FirstParentChainSHAs(context.Background(), r, "main")
	if err != nil {
		t.Fatalf("FirstParentChainSHAs: %v", err)
	}
	if !git.IsSHAOnChain(tip, mainline) {
		t.Error("expected true: fast-forward merge leaves branch tip on mainline")
	}
}

func TestFirstParentChainSHAsLeadingDashMergeRef(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-fpc-dash", "HEAD")

	mainline, err := git.FirstParentChainSHAs(context.Background(), r, "-fpc-dash")
	if err != nil {
		t.Fatalf("FirstParentChainSHAs on leading-dash mergeRef: %v", err)
	}
	if len(mainline) == 0 {
		t.Error("expected at least one SHA on the mainline chain")
	}
}

func TestMainlinePatchIDsSinceLeadingDashMergeBase(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// -mpids-dash must be mergeBase, not mergeRef, so the dash leads the combined range arg.
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-mpids-dash", "HEAD")
	testutil.CreateFile(t, repo, "mp.txt", "content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "mainline patch commit")

	pids, err := git.MainlinePatchIDsSince(context.Background(), r, "-mpids-dash", "main")
	if err != nil {
		t.Fatalf("MainlinePatchIDsSince with leading-dash mergeBase: %v", err)
	}
	if len(pids) == 0 {
		t.Error("expected at least one patch-id since the leading-dash merge-base")
	}
}

func TestCurrentBranchDetachedHead(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegration)
	}

	repo := testutil.NewTestRepo(t)
	r := &git.ExecRunner{Dir: repo}

	// Detach HEAD by checking out the current commit SHA directly.
	sha := testutil.GitCmd(t, repo, "rev-parse", "HEAD")
	testutil.GitCmd(t, repo, "checkout", "--detach", strings.TrimSpace(sha))

	_, err := git.CurrentBranch(context.Background(), r, repo)
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
}
