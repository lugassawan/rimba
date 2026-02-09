package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	porcelainWorktree  = "worktree "
	fmtExpectedResults = "expected 1 result(s), got %d"
)

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	worktreeOutput string
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.worktreeOutput, nil
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	return m.Run(args...)
}

func mockWorktreeList(paths ...string) string {
	var b strings.Builder
	branches := []string{"refs/heads/main", "refs/heads/feature/task-1", "refs/heads/feature/other", "refs/heads/feature/new"}
	hashes := []string{"abc123", "def456", "ghi789", "jkl012"}
	for i, p := range paths {
		if i > 0 {
			b.WriteString("\n")
		}
		branch := branches[i%len(branches)]
		hash := hashes[i%len(hashes)]
		b.WriteString(porcelainWorktree + p + "\nHEAD " + hash + "\nbranch " + branch + "\n")
	}
	return b.String()
}

func TestManagerInstallClone(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "lockfile-v6-content")
	writeFile(t, newWT, LockfilePnpm, "lockfile-v6-content")

	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existingWT, DirNodeModules), "package.json", "{}")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "pnpm install --frozen-lockfile",
			Recursive:  false,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Error("expected Cloned=true")
	}
	if r.Source != existingWT {
		t.Errorf("expected source %s, got %s", existingWT, r.Source)
	}
	if r.Error != nil {
		t.Errorf("expected no error, got %v", r.Error)
	}

	assertFileContent(t, filepath.Join(newWT, DirNodeModules, "package.json"), "{}")
}

func TestManagerInstallNoMatchCloneOnly(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfileGo, "go sum content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirVendor,
			Lockfile:   LockfileGo,
			InstallCmd: "go mod vendor",
			CloneOnly:  true,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no match)")
	}
	if r.Error != nil {
		t.Error("expected no error for CloneOnly skip")
	}
}

func TestManagerInstallNoLockfile(t *testing.T) {
	newWT := t.TempDir()

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:      DirNodeModules,
			Lockfile: LockfilePnpm,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false")
	}
	if r.Error != nil {
		t.Error("expected no error for missing lockfile")
	}
}

func TestManagerInstallHashMismatch(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "old content")
	writeFile(t, newWT, LockfilePnpm, "new content")

	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo skipped",
		},
	}

	results := mgr.Install(newWT, modules)

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (hash mismatch)")
	}
}

func TestManagerInstallPreferSource(t *testing.T) {
	sourceWT := t.TempDir()
	otherWT := t.TempDir()
	newWT := t.TempDir()

	lockContent := "same-lockfile-content"
	writeFile(t, sourceWT, LockfilePnpm, lockContent)
	writeFile(t, otherWT, LockfilePnpm, lockContent)
	writeFile(t, newWT, LockfilePnpm, lockContent)

	for _, dir := range []string{sourceWT, otherWT} {
		if err := os.MkdirAll(filepath.Join(dir, DirNodeModules), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, DirNodeModules), "origin.txt", dir)
	}

	runner := &mockRunner{worktreeOutput: mockWorktreeList(sourceWT, otherWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.InstallPreferSource(newWT, sourceWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Fatal("expected Cloned=true")
	}
	if r.Source != sourceWT {
		t.Errorf("expected source to be sourceWT, got %s", r.Source)
	}

	assertFileContent(t, filepath.Join(newWT, DirNodeModules, "origin.txt"), sourceWT)
}
