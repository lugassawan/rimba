package deps

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	goosDarwin = "darwin"
	goosLinux  = "linux"
)

// cowCopyCmd builds the copy-on-write copy command for the host OS.
// A package var so tests can inject a failing first-copy seam.
var cowCopyCmd = func(src, dst string) *exec.Cmd {
	switch runtime.GOOS {
	case goosDarwin:
		return exec.Command("cp", "-c", "-R", src, dst)
	case goosLinux:
		return exec.Command("cp", "--reflink=auto", "-R", src, dst)
	default:
		return exec.Command("cp", "-R", src, dst)
	}
}

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

	var cloneErrs []error
	baseName := filepath.Base(mod.Dir)
	_ = filepath.WalkDir(searchRoot, walkCloneFunc(srcWT, dstWT, baseName, &cloneErrs))

	if err := cloneExtraDirs(srcWT, dstWT, mod.ExtraDirs); err != nil {
		cloneErrs = append(cloneErrs, err)
	}

	return errors.Join(cloneErrs...)
}

// walkCloneFunc returns a WalkDirFunc that clones directories matching baseName.
// Clone failures are appended to errs; walking continues regardless.
func walkCloneFunc(srcWT, dstWT, baseName string, errs *[]error) fs.WalkDirFunc {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			*errs = append(*errs, err)
			return nil // keep walking
		}
		if !d.IsDir() {
			return nil
		}

		if d.Name() != baseName {
			return nil
		}

		relPath, _ := filepath.Rel(srcWT, path)
		return cloneIfParentExists(path, dstWT, relPath, errs)
	}
}

// cloneIfParentExists clones srcPath to dstWT/relPath if the parent dir exists in dstWT.
// Clone failures are appended to errs and the directory is skipped.
func cloneIfParentExists(srcPath, dstWT, relPath string, errs *[]error) error {
	dstParent := filepath.Join(dstWT, filepath.Dir(relPath))
	if _, err := os.Stat(dstParent); os.IsNotExist(err) {
		return filepath.SkipDir
	}

	dst := filepath.Join(dstWT, relPath)
	if err := CloneDir(srcPath, dst); err != nil {
		*errs = append(*errs, fmt.Errorf("clone %s: %w", relPath, err))
		return filepath.SkipDir // continue walking other dirs
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

func cmdErr(prefix string, out []byte, err error) error {
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%s: %s: %w", prefix, msg, err)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func cowCopy(src, dst string) error {
	out, err := cowCopyCmd(src, dst).CombinedOutput()
	if err == nil {
		return nil
	}
	cowErr := cmdErr("cow copy", out, err)

	// Only attempt the fallback on platforms where CoW was actually tried.
	// On the default branch cowCopyCmd already emits a plain cp -R, so
	// retrying an identical command on failure adds no value.
	if runtime.GOOS != goosDarwin && runtime.GOOS != goosLinux {
		return cowErr
	}

	// A failed CoW cp can leave a partially written dst; remove it so the
	// fallback lands src's contents AS dst rather than nested inside dst/<base>.
	if rmErr := os.RemoveAll(dst); rmErr != nil {
		return errors.Join(cowErr, fmt.Errorf("clean dst before fallback: %w", rmErr))
	}

	if out, fbErr := exec.Command("cp", "-R", src, dst).CombinedOutput(); fbErr != nil {
		return errors.Join(cowErr, cmdErr("fallback copy", out, fbErr))
	}
	return nil
}
