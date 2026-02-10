package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const skipShortBench = "skipping integration benchmark in short mode"

// newBenchRepo creates a temporary git repo for benchmarks.
func newBenchRepo(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	repo := filepath.Join(dir, "bench-repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		b.Fatalf("mkdir: %v", err)
	}

	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "bench@test.com"},
		{"git", "config", "user.name", "Bench"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("cmd %v: %s: %v", args, out, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Bench\n"), 0644); err != nil {
		b.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("cmd %v: %s: %v", args, out, err)
		}
	}
	return repo
}

// addBenchWorktrees creates n worktrees in the repo.
func addBenchWorktrees(b *testing.B, repo string, n int) []string {
	b.Helper()
	var paths []string
	for i := 0; i < n; i++ {
		branch := filepath.Join("feature", "bench-"+string(rune('a'+i)))
		wtPath := filepath.Join(filepath.Dir(repo), "wt-"+string(rune('a'+i)))
		cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "main")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("worktree add: %s: %v", out, err)
		}
		paths = append(paths, wtPath)
	}
	return paths
}

func BenchmarkListWorktrees5(b *testing.B) {
	if testing.Short() {
		b.Skip(skipShortBench)
	}
	repo := newBenchRepo(b)
	addBenchWorktrees(b, repo, 5)
	r := &ExecRunner{Dir: repo}

	b.ResetTimer()
	for b.Loop() {
		if _, err := ListWorktrees(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListWorktrees10(b *testing.B) {
	if testing.Short() {
		b.Skip(skipShortBench)
	}
	repo := newBenchRepo(b)
	addBenchWorktrees(b, repo, 10)
	r := &ExecRunner{Dir: repo}

	b.ResetTimer()
	for b.Loop() {
		if _, err := ListWorktrees(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsDirty(b *testing.B) {
	if testing.Short() {
		b.Skip(skipShortBench)
	}
	repo := newBenchRepo(b)
	wts := addBenchWorktrees(b, repo, 1)
	r := &ExecRunner{Dir: repo}

	b.ResetTimer()
	for b.Loop() {
		if _, err := IsDirty(r, wts[0]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAheadBehind(b *testing.B) {
	if testing.Short() {
		b.Skip(skipShortBench)
	}
	repo := newBenchRepo(b)
	wts := addBenchWorktrees(b, repo, 1)
	r := &ExecRunner{Dir: repo}

	b.ResetTimer()
	for b.Loop() {
		_, _, err := AheadBehind(r, wts[0])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsDirtyParallel(b *testing.B) {
	if testing.Short() {
		b.Skip(skipShortBench)
	}
	repo := newBenchRepo(b)
	wts := addBenchWorktrees(b, repo, 4)
	r := &ExecRunner{Dir: repo}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := IsDirty(r, wts[i%len(wts)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
