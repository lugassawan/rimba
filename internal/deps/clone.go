package deps

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CloneDir copies a directory from src to dst using CoW (copy-on-write) when available.
// Falls back to regular copy if CoW is not supported.
func CloneDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	if _, err := os.Stat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	if err := cowCopy(src, dst); err != nil {
		return err
	}
	return nil
}

// CloneModule clones a module's dependency directories from one worktree to another.
func CloneModule(srcWT, dstWT string, mod Module) error {
	if mod.Recursive {
		return cloneRecursive(srcWT, dstWT, mod)
	}
	return cloneSingle(srcWT, dstWT, mod)
}

func cloneSingle(srcWT, dstWT string, mod Module) error {
	src := filepath.Join(srcWT, mod.Dir)
	dst := filepath.Join(dstWT, mod.Dir)

	if _, err := os.Stat(src); err != nil {
		return err
	}

	if err := CloneDir(src, dst); err != nil {
		return err
	}

	return cloneExtraDirs(srcWT, dstWT, mod.ExtraDirs)
}

func cloneRecursive(srcWT, dstWT string, mod Module) error {
	searchRoot := srcWT
	if mod.WorkDir != "" {
		searchRoot = filepath.Join(srcWT, mod.WorkDir)
	}

	baseName := filepath.Base(mod.Dir)
	err := filepath.WalkDir(searchRoot, walkCloneFunc(srcWT, dstWT, baseName))
	if err != nil {
		return err
	}

	return cloneExtraDirs(srcWT, dstWT, mod.ExtraDirs)
}

// walkCloneFunc returns a WalkDirFunc that clones directories matching baseName.
func walkCloneFunc(srcWT, dstWT, baseName string) fs.WalkDirFunc {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // WalkDir: skip errors and continue walking
		}
		if !d.IsDir() {
			return nil
		}

		if d.Name() != baseName {
			return nil
		}

		relPath, _ := filepath.Rel(srcWT, path)
		return cloneIfParentExists(path, dstWT, relPath)
	}
}

// cloneIfParentExists clones srcPath to dstWT/relPath if the parent dir exists in dstWT.
func cloneIfParentExists(srcPath, dstWT, relPath string) error {
	dstParent := filepath.Join(dstWT, filepath.Dir(relPath))
	if _, err := os.Stat(dstParent); os.IsNotExist(err) {
		return filepath.SkipDir
	}

	dst := filepath.Join(dstWT, relPath)
	if err := CloneDir(srcPath, dst); err != nil { //nolint:nilerr // best-effort clone; continue walking
		return filepath.SkipDir
	}
	return filepath.SkipDir // don't descend into cloned dir
}

func cloneExtraDirs(srcWT, dstWT string, extraDirs []string) error {
	for _, extra := range extraDirs {
		extraSrc := filepath.Join(srcWT, extra)
		extraDst := filepath.Join(dstWT, extra)
		if _, err := os.Stat(extraSrc); err == nil {
			if err := CloneDir(extraSrc, extraDst); err != nil {
				return err
			}
		}
	}
	return nil
}

func cowCopy(src, dst string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("cp", "-c", "-R", src, dst)
	case "linux":
		cmd = exec.Command("cp", "--reflink=auto", "-R", src, dst)
	default:
		cmd = exec.Command("cp", "-R", src, dst)
	}

	if err := cmd.Run(); err != nil {
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			fallback := exec.Command("cp", "-R", src, dst)
			return fallback.Run()
		}
		return err
	}
	return nil
}
