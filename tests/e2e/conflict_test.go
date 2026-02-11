package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// conflictPair sets up two worktrees with commits and returns the worktree paths.
type conflictPair struct {
	repo, pathA, pathB string
}

func setupConflictPair(t *testing.T) conflictPair {
	t.Helper()

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDupA, flagSkipDepsE2E, flagSkipHooksE2E)
	rimbaSuccess(t, repo, "add", taskDupB, flagSkipDepsE2E, flagSkipHooksE2E)

	cfg, err := config.Load(filepath.Join(repo, configFile))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	pathA := resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskDupA))
	pathB := resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskDupB))

	return conflictPair{repo: repo, pathA: pathA, pathB: pathB}
}

func TestConflictCheckOverlap(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cp := setupConflictPair(t)

	testutil.CreateFile(t, cp.pathA, fileShared, pkgA)
	testutil.GitCmd(t, cp.pathA, "add", ".")
	testutil.GitCmd(t, cp.pathA, "commit", "-m", "modify shared in A")

	testutil.CreateFile(t, cp.pathB, fileShared, pkgB)
	testutil.GitCmd(t, cp.pathB, "add", ".")
	testutil.GitCmd(t, cp.pathB, "commit", "-m", "modify shared in B")

	r := rimbaSuccess(t, cp.repo, cmdConflictCheck)
	assertContains(t, r.Stdout, fileShared)
	assertContains(t, r.Stdout, "overlaps")
}

func TestConflictCheckNoOverlap(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cp := setupConflictPair(t)

	testutil.CreateFile(t, cp.pathA, "a.go", pkgA)
	testutil.GitCmd(t, cp.pathA, "add", ".")
	testutil.GitCmd(t, cp.pathA, "commit", "-m", "add a.go")

	testutil.CreateFile(t, cp.pathB, "b.go", pkgB)
	testutil.GitCmd(t, cp.pathB, "add", ".")
	testutil.GitCmd(t, cp.pathB, "commit", "-m", "add b.go")

	r := rimbaSuccess(t, cp.repo, cmdConflictCheck)
	assertContains(t, r.Stdout, "No file overlaps")
}

func TestConflictCheckSpecificPair(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cp := setupConflictPair(t)

	testutil.CreateFile(t, cp.pathA, fileCommon, pkgA)
	testutil.GitCmd(t, cp.pathA, "add", ".")
	testutil.GitCmd(t, cp.pathA, "commit", "-m", "add common")

	testutil.CreateFile(t, cp.pathB, fileCommon, pkgB)
	testutil.GitCmd(t, cp.pathB, "add", ".")
	testutil.GitCmd(t, cp.pathB, "commit", "-m", "add common")

	r := rimbaSuccess(t, cp.repo, cmdConflictCheck, taskDupA, taskDupB)
	assertContains(t, r.Stdout, fileCommon)
}

func TestConflictCheckTooFewBranches(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDupA, flagSkipDepsE2E, flagSkipHooksE2E)

	r := rimbaSuccess(t, repo, cmdConflictCheck)
	assertContains(t, r.Stdout, "Need at least 2 branches")
}

func TestMergePlan(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cp := setupConflictPair(t)

	testutil.CreateFile(t, cp.pathA, fileShared, "a content\n")
	testutil.GitCmd(t, cp.pathA, "add", ".")
	testutil.GitCmd(t, cp.pathA, "commit", "-m", "A changes")

	testutil.CreateFile(t, cp.pathB, fileShared, "b content\n")
	testutil.GitCmd(t, cp.pathB, "add", ".")
	testutil.GitCmd(t, cp.pathB, "commit", "-m", "B changes")

	r := rimbaSuccess(t, cp.repo, "merge-plan")
	assertContains(t, r.Stdout, "merge order")
}

func TestMergePlanNoOverlaps(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cp := setupConflictPair(t)

	testutil.CreateFile(t, cp.pathA, "unique-a.go", pkgA)
	testutil.GitCmd(t, cp.pathA, "add", ".")
	testutil.GitCmd(t, cp.pathA, "commit", "-m", "add a")

	testutil.CreateFile(t, cp.pathB, "unique-b.go", pkgB)
	testutil.GitCmd(t, cp.pathB, "add", ".")
	testutil.GitCmd(t, cp.pathB, "commit", "-m", "add b")

	r := rimbaSuccess(t, cp.repo, "merge-plan")
	assertContains(t, r.Stdout, "any order")
}
