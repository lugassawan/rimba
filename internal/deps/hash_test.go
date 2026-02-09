package deps

import (
	"testing"
)

const testLockfile = "lock.yaml"

func TestHashLockfileValid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, testLockfile, "some content")

	hash, err := HashLockfile(dir, testLockfile)
	if err != nil {
		t.Fatal(err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// Verify deterministic
	hash2, err := HashLockfile(dir, testLockfile)
	if err != nil {
		t.Fatal(err)
	}
	if hash != hash2 {
		t.Errorf("expected deterministic hash, got %s and %s", hash, hash2)
	}
}

func TestHashLockfileMissing(t *testing.T) {
	dir := t.TempDir()

	hash, err := HashLockfile(dir, "nonexistent.lock")
	if err != nil {
		t.Fatal(err)
	}

	if hash != "" {
		t.Errorf("expected empty hash for missing file, got %s", hash)
	}
}

func TestHashLockfileDifferentContent(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	writeFile(t, dir1, testLockfile, "content A")
	writeFile(t, dir2, testLockfile, "content B")

	hash1, _ := HashLockfile(dir1, testLockfile)
	hash2, _ := HashLockfile(dir2, testLockfile)

	if hash1 == hash2 {
		t.Error("expected different hashes for different content")
	}
}

func TestHashModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfilePnpm, "lockfile content")
	writeFile(t, dir, LockfileGo, "go sum content")

	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
		{Dir: DirVendor, Lockfile: LockfileGo},
	}

	result, err := HashModules(dir, modules)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	if result[0].Hash == "" {
		t.Errorf("expected non-empty hash for %s", LockfilePnpm)
	}
	if result[1].Hash == "" {
		t.Errorf("expected non-empty hash for %s", LockfileGo)
	}
}
