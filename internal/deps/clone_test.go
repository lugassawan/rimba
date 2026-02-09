package deps

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testFileTxt = "file.txt"
	testPkgJSON = "pkg.json"
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
