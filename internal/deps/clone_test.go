package deps

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testFileTxt = "file.txt"
	testPkgJSON = "pkg.json"
	testDataTxt = "data.txt"
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

	writeFile(t, src, testFileTxt, "new content")

	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dst, "old.txt", "old content")

	if err := CloneDir(src, dst); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, testFileTxt), "new content")

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
	writeFile(t, dst, "stale.txt", "stale data")

	// Create src with new content
	writeFile(t, src, "fresh.txt", "fresh data")

	if err := CloneDir(src, dst); err != nil {
		t.Fatalf("CloneDir failed: %v", err)
	}

	// New content should be present
	assertFileContent(t, filepath.Join(dst, "fresh.txt"), "fresh data")

	// Old content should be gone
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
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

	err := cloneIfParentExists(srcPath, dstWT, relPath)
	if err != filepath.SkipDir {
		t.Errorf("expected filepath.SkipDir, got: %v", err)
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

	fn := walkCloneFunc(srcWT, dstWT, DirNodeModules)

	// Call the returned WalkDirFunc with a non-nil error — should return nil (skip)
	err := fn("/some/path", nil, os.ErrPermission)
	if err != nil {
		t.Errorf("expected nil when err is non-nil, got: %v", err)
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

	fn := walkCloneFunc(srcWT, dstWT, DirNodeModules)

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

	err := cloneIfParentExists(srcPath, dstWT, relPath)
	if err != filepath.SkipDir {
		t.Errorf("expected filepath.SkipDir on clone failure, got: %v", err)
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
	writeFile(t, yarnCache, "dep.zip", "cached")

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
	assertFileContent(t, filepath.Join(dstWT, ".yarn", "cache", "dep.zip"), "cached")
}
