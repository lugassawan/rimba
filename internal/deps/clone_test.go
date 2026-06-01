package deps

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testFileTxt   = "file.txt"
	testPkgJSON   = "pkg.json"
	testDataTxt   = "data.txt"
	testStaleTxt  = "stale.txt"
	testDepZip    = "dep.zip"
	valNewContent = "new content"
)

func TestCloneDirBasic(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "cloned")

	writeFile(t, src, testFileTxt, "hello")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "sub"), "nested.txt", "world")

	if err := CloneDir(src, dst); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, testFileTxt), "hello")
	assertFileContent(t, filepath.Join(dst, "sub", "nested.txt"), "world")
}

func TestCloneDirSuccess(t *testing.T) {
	src := t.TempDir()
	dstParent := t.TempDir()
	dst := filepath.Join(dstParent, "target")

	writeFile(t, src, testDataTxt, "some data")

	if err := CloneDir(src, dst); err != nil {
		t.Fatalf("CloneDir failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, testDataTxt), "some data")
}

func TestCloneDirOverwritesExisting(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	writeFile(t, src, testFileTxt, valNewContent)

	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dst, "old.txt", "old content")

	if err := CloneDir(src, dst); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, testFileTxt), valNewContent)

	if _, err := os.Stat(filepath.Join(dst, "old.txt")); !os.IsNotExist(err) {
		t.Error("expected old.txt to be removed")
	}
}

func TestCloneDirOverwriteExisting(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	// Create dst with a different file first
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dst, testStaleTxt, "stale data")

	// Create src with new content
	writeFile(t, src, "fresh.txt", "fresh data")

	if err := CloneDir(src, dst); err != nil {
		t.Fatalf("CloneDir failed: %v", err)
	}

	// New content should be present
	assertFileContent(t, filepath.Join(dst, "fresh.txt"), "fresh data")

	// Old content should be gone
	if _, err := os.Stat(filepath.Join(dst, testStaleTxt)); !os.IsNotExist(err) {
		t.Error("expected stale.txt to be removed after overwrite")
	}
}

func TestCloneModuleSingle(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	vendorDir := filepath.Join(srcWT, DirVendor)
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, vendorDir, "module.go", "package foo")

	mod := Module{Dir: DirVendor}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dstWT, DirVendor, "module.go"), "package foo")
}

func TestCloneModuleSingleWithExtraDirs(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, DirNodeModules), testPkgJSON, "{}")

	if err := os.MkdirAll(filepath.Join(srcWT, ".yarn", "cache"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, ".yarn", "cache"), "data.zip", "cache")

	mod := Module{
		Dir:       DirNodeModules,
		ExtraDirs: []string{DirYarnCache},
	}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dstWT, DirNodeModules, testPkgJSON), "{}")
	assertFileContent(t, filepath.Join(dstWT, ".yarn", "cache", "data.zip"), "cache")
}

func TestCloneModuleRecursive(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, DirNodeModules), ".package-lock.json", "{}")

	if err := os.MkdirAll(filepath.Join(srcWT, "app-a", DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, "app-a", DirNodeModules), testPkgJSON, "app-a")

	// Dst must have the parent dir for the nested clone to work
	if err := os.MkdirAll(filepath.Join(dstWT, "app-a"), 0755); err != nil {
		t.Fatal(err)
	}

	mod := Module{
		Dir:       DirNodeModules,
		Recursive: true,
	}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dstWT, DirNodeModules, ".package-lock.json"), "{}")
	assertFileContent(t, filepath.Join(dstWT, "app-a", DirNodeModules, testPkgJSON), "app-a")
}

func TestCloneModuleRecursiveSkipsMissingParent(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcWT, "app-b", DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, "app-b", DirNodeModules), testPkgJSON, "app-b")

	mod := Module{
		Dir:       DirNodeModules,
		Recursive: true,
	}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dstWT, "app-b", DirNodeModules)); !os.IsNotExist(err) {
		t.Error("expected app-b/node_modules to not be cloned (parent missing in dst)")
	}
}

func TestCloneSingleSourceNotFound(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	mod := Module{Dir: "nonexistent_deps"}

	err := cloneSingle(srcWT, dstWT, mod)
	if err == nil {
		t.Fatal("expected error when source dir does not exist")
	}

	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got: %v", err)
	}
}

func TestCloneExtraDirsSuccess(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create srcWT/.yarn/cache with a file
	yarnCacheDir := filepath.Join(srcWT, ".yarn", "cache")
	if err := os.MkdirAll(yarnCacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, yarnCacheDir, "dep-1.0.0.zip", "cached-content")

	err := cloneExtraDirs(srcWT, dstWT, []string{DirYarnCache})
	if err != nil {
		t.Fatalf("cloneExtraDirs failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dstWT, ".yarn", "cache", "dep-1.0.0.zip"), "cached-content")
}

func TestCloneExtraDirsMissing(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Call with a non-existent extra dir — should not error
	err := cloneExtraDirs(srcWT, dstWT, []string{"does/not/exist"})
	if err != nil {
		t.Fatalf("expected no error for missing extra dir, got: %v", err)
	}

	// Verify nothing was created in dstWT
	if _, err := os.Stat(filepath.Join(dstWT, "does")); !os.IsNotExist(err) {
		t.Error("expected nothing to be created for missing extra dir")
	}
}

func TestCloneIfParentExistsNoParent(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create a source path to clone from
	srcPath := filepath.Join(srcWT, "packages", "foo", "node_modules")
	if err := os.MkdirAll(srcPath, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, srcPath, "index.js", "module.exports = {}")

	// dstWT does NOT have packages/foo — parent is missing
	relPath := filepath.Join("packages", "foo", "node_modules")

	var errs []error
	err := cloneIfParentExists(srcPath, dstWT, relPath, &errs)
	if !errors.Is(err, filepath.SkipDir) {
		t.Errorf("expected filepath.SkipDir, got: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no collected errors for missing-parent skip, got: %v", errs)
	}

	// Verify node_modules was NOT cloned
	if _, statErr := os.Stat(filepath.Join(dstWT, relPath)); !os.IsNotExist(statErr) {
		t.Error("expected node_modules to not be cloned when parent is missing")
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Errorf("expected content %q, got %q in %s", expected, string(data), path)
	}
}

func TestCowCopySuccess(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "copied")

	writeFile(t, src, testDataTxt, "cow-content")

	if err := cowCopy(src, dst); err != nil {
		t.Fatalf("cowCopy failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, testDataTxt), "cow-content")
}

func TestCowCopyNonexistentSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")
	err := cowCopy("/nonexistent/path/src", dst)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestCloneDirMkdirAllError(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, testDataTxt, "hello")

	// Create a regular file where MkdirAll needs a directory.
	// CloneDir calls MkdirAll(filepath.Dir(dst)), so if dst is "blockfile/child",
	// then filepath.Dir(dst) is "blockfile" — which is a regular file, not a directory.
	base := t.TempDir()
	blockFile := filepath.Join(base, "blockfile")
	if err := os.WriteFile(blockFile, []byte("i am a file"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(blockFile, "child", "target")
	err := CloneDir(src, dst)
	if err == nil {
		t.Fatal("expected error when MkdirAll cannot create parent")
	}
}

func TestCloneRecursiveWithWorkDir(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create srcWT/api/node_modules with a file
	nmDir := filepath.Join(srcWT, "api", DirNodeModules)
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, nmDir, testPkgJSON, "api-deps")

	// Create dstWT/api so the parent exists
	if err := os.MkdirAll(filepath.Join(dstWT, "api"), 0755); err != nil {
		t.Fatal(err)
	}

	mod := Module{
		Dir:       DirNodeModules,
		Recursive: true,
		WorkDir:   "api",
	}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatalf("CloneModule with WorkDir failed: %v", err)
	}

	assertFileContent(t, filepath.Join(dstWT, "api", DirNodeModules, testPkgJSON), "api-deps")
}

func TestWalkCloneFuncSkipsErrors(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	var errs []error
	fn := walkCloneFunc(srcWT, dstWT, DirNodeModules, &errs)

	// Call the returned WalkDirFunc with a non-nil error — should return nil (keep walking)
	err := fn("/some/path", nil, os.ErrPermission)
	if err != nil {
		t.Errorf("expected nil when err is non-nil, got: %v", err)
	}
	if len(errs) != 1 || !errors.Is(errs[0], os.ErrPermission) {
		t.Errorf("expected traversal error to be collected, got: %v", errs)
	}
}

func TestWalkCloneFuncSkipsFiles(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create a file so we can get its DirEntry
	filePath := filepath.Join(srcWT, "somefile.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(srcWT)
	if err != nil {
		t.Fatal(err)
	}

	var fileEntry os.DirEntry
	for _, e := range entries {
		if e.Name() == "somefile.txt" {
			fileEntry = e
			break
		}
	}
	if fileEntry == nil {
		t.Fatal("could not find file entry")
	}

	var errs []error
	fn := walkCloneFunc(srcWT, dstWT, DirNodeModules, &errs)

	result := fn(filePath, fileEntry, nil)
	if result != nil {
		t.Errorf("expected nil for file entry, got: %v", result)
	}
}

func TestCloneIfParentExistsCloneFails(t *testing.T) {
	dstWT := t.TempDir()

	// Create the parent in dstWT so cloneIfParentExists doesn't skip due to missing parent
	parentDir := filepath.Join(dstWT, "packages", "foo")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// srcPath is nonexistent — CloneDir will fail
	srcPath := filepath.Join(t.TempDir(), "nonexistent", "node_modules")
	relPath := filepath.Join("packages", "foo", "node_modules")

	var errs []error
	err := cloneIfParentExists(srcPath, dstWT, relPath, &errs)
	if !errors.Is(err, filepath.SkipDir) {
		t.Errorf("expected filepath.SkipDir on clone failure, got: %v", err)
	}
	if len(errs) == 0 {
		t.Error("expected clone failure to be collected in errs")
	}
}

func TestCloneExtraDirsCloneError(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create extra dir source
	extraSrc := filepath.Join(srcWT, ".yarn", "cache")
	if err := os.MkdirAll(extraSrc, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, extraSrc, testDepZip, "cached")

	// Block the destination parent with a regular file so CloneDir MkdirAll fails
	blockFile := filepath.Join(dstWT, ".yarn")
	if err := os.WriteFile(blockFile, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	err := cloneExtraDirs(srcWT, dstWT, []string{".yarn/cache"})
	if err == nil {
		t.Fatal("expected error when extra dir clone fails")
	}
}

func TestCloneDirRemoveAllError(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, testDataTxt, "hello")

	// Create dst inside a parent that becomes read-only after dst is created
	parent := t.TempDir()
	dst := filepath.Join(parent, "target")
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dst, testStaleTxt, "old data")

	// Make parent read-only so RemoveAll(dst) fails
	if err := os.Chmod(parent, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0755) })

	err := CloneDir(src, dst)
	if err == nil {
		t.Fatal("expected error when RemoveAll fails on read-only parent")
	}
}

func TestCloneRecursiveWithExtraDirs(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create root-level node_modules
	if err := os.MkdirAll(filepath.Join(srcWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, DirNodeModules), "root.json", "{}")

	// Create .yarn/cache with data
	yarnCache := filepath.Join(srcWT, ".yarn", "cache")
	if err := os.MkdirAll(yarnCache, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, yarnCache, testDepZip, "cached")

	// Create nested app-a/node_modules
	if err := os.MkdirAll(filepath.Join(srcWT, "app-a", DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, "app-a", DirNodeModules), "app.json", "app-a-deps")

	// dstWT must have app-a dir for nested clone to succeed
	if err := os.MkdirAll(filepath.Join(dstWT, "app-a"), 0755); err != nil {
		t.Fatal(err)
	}

	mod := Module{
		Dir:       DirNodeModules,
		Recursive: true,
		ExtraDirs: []string{DirYarnCache},
	}

	if err := CloneModule(srcWT, dstWT, mod); err != nil {
		t.Fatalf("CloneModule recursive with ExtraDirs failed: %v", err)
	}

	// Verify root node_modules
	assertFileContent(t, filepath.Join(dstWT, DirNodeModules, "root.json"), "{}")

	// Verify nested node_modules
	assertFileContent(t, filepath.Join(dstWT, "app-a", DirNodeModules, "app.json"), "app-a-deps")

	// Verify extra dir (.yarn/cache) was cloned
	assertFileContent(t, filepath.Join(dstWT, ".yarn", "cache", testDepZip), "cached")
}

func TestCloneModuleRecursiveSearchRootNotFound(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// WorkDir points to a non-existent directory — WalkDir fails on Lstat
	mod := Module{Dir: DirNodeModules, Recursive: true, WorkDir: "nonexistent"}
	err := CloneModule(srcWT, dstWT, mod)
	if err == nil {
		t.Fatal("expected error when WorkDir does not exist")
	}
}

func TestCloneModuleRecursiveExtraDirError(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create a node_modules in srcWT root so walk finds it
	if err := os.MkdirAll(filepath.Join(srcWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(srcWT, DirNodeModules), "pkg.json", "{}")

	// Create a valid ExtraDir source
	extraSrc := filepath.Join(srcWT, ".yarn", "cache")
	if err := os.MkdirAll(extraSrc, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, extraSrc, testDepZip, "data")

	// Block the ExtraDir dst by placing a regular file where the parent dir would go
	if err := os.WriteFile(filepath.Join(dstWT, ".yarn"), []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	mod := Module{Dir: DirNodeModules, Recursive: true, ExtraDirs: []string{DirYarnCache}}
	err := CloneModule(srcWT, dstWT, mod)
	if err == nil {
		t.Fatal("expected error when ExtraDir clone fails in recursive mode")
	}
}

// TestCowCopyFallbackNotNested verifies that a partial CoW failure (debris left at dst)
// is corrected before the fallback: dst is re-cleaned so src lands AS dst, not nested.
func TestCowCopyFallbackNotNested(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	writeFile(t, src, "file.txt", "source-content")

	// Pre-create dst with debris to simulate a partially-written CoW copy.
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "debris"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	orig := cowCopyCmd
	cowCopyCmd = func(s, d string) *exec.Cmd { return exec.Command("false") }
	t.Cleanup(func() { cowCopyCmd = orig })

	if err := cowCopy(src, dst); err != nil {
		t.Fatalf("cowCopy returned unexpected error: %v", err)
	}

	// src's file must land directly at dst/file.txt, not nested as dst/<base(src)>/file.txt.
	assertFileContent(t, filepath.Join(dst, "file.txt"), "source-content")

	// Debris left by the partial CoW copy must be gone (dst was re-cleaned before fallback).
	if _, err := os.Stat(filepath.Join(dst, "debris")); !os.IsNotExist(err) {
		t.Error("expected debris to be removed (dst was not re-cleaned before fallback)")
	}

	// Nested layout dst/<basename(src)>/* must not exist.
	if _, err := os.Stat(filepath.Join(dst, filepath.Base(src))); !os.IsNotExist(err) {
		t.Error("expected fallback to place src contents AS dst, not nested inside dst/<base(src)>")
	}
}

// TestCowCopyDoubleFailurePreservesBothCauses verifies that when both the CoW attempt
// and the fallback cp fail, the returned error names both causes.
func TestCowCopyDoubleFailurePreservesBothCauses(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")

	orig := cowCopyCmd
	cowCopyCmd = func(s, d string) *exec.Cmd { return exec.Command("false") }
	t.Cleanup(func() { cowCopyCmd = orig })

	// Nonexistent src causes the fallback cp -R to also fail.
	err := cowCopy("/nonexistent/does/not/exist/src", dst)
	if err == nil {
		t.Fatal("expected error when both CoW and fallback copy fail")
	}
	if !strings.Contains(err.Error(), "cow copy") {
		t.Errorf("expected 'cow copy' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fallback copy") {
		t.Errorf("expected 'fallback copy' in error, got: %v", err)
	}
}

// TestCowCopyRemoveAllError covers the branch where dst cleanup fails after a CoW failure.
func TestCowCopyRemoveAllError(t *testing.T) {
	parent := t.TempDir()
	dst := filepath.Join(parent, "target")
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}

	// Make parent read-only so os.RemoveAll(dst) fails.
	if err := os.Chmod(parent, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0755) })

	orig := cowCopyCmd
	cowCopyCmd = func(s, d string) *exec.Cmd { return exec.Command("false") }
	t.Cleanup(func() { cowCopyCmd = orig })

	if err := cowCopy("/some/src", dst); err == nil {
		t.Fatal("expected error when RemoveAll cannot delete dst")
	}
}

func TestCloneModuleRecursiveAggregatesErrors(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	// Create two nested node_modules dirs in srcWT
	for _, app := range []string{"app-a", "app-b"} {
		if err := os.MkdirAll(filepath.Join(srcWT, app, DirNodeModules), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(srcWT, app, DirNodeModules), "pkg.json", app)
	}

	// app-a: create the parent dir normally so it clones fine
	if err := os.MkdirAll(filepath.Join(dstWT, "app-a"), 0755); err != nil {
		t.Fatal(err)
	}

	// app-b: place a regular file named "app-b" so MkdirAll(dstWT/app-b) fails
	// inside CloneDir, causing the clone to fail and be collected in errs.
	if err := os.WriteFile(filepath.Join(dstWT, "app-b"), []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	mod := Module{Dir: DirNodeModules, Recursive: true}
	err := CloneModule(srcWT, dstWT, mod)

	// app-a should have been cloned successfully despite app-b failing
	assertFileContent(t, filepath.Join(dstWT, "app-a", DirNodeModules, "pkg.json"), "app-a")

	// Error must be returned and mention the failed path
	if err == nil {
		t.Fatal("expected aggregated error for partial clone failure")
	}
	if !strings.Contains(err.Error(), "app-b") {
		t.Errorf("error = %q, want it to mention 'app-b'", err.Error())
	}
}
