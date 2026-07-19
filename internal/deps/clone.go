package deps

import (
	"context"
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
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// cowCopyCmd builds the copy-on-write copy command for the host OS.
// A package var so tests can inject a failing first-copy seam.
var cowCopyCmd = func(ctx context.Context, src, dst string) *exec.Cmd {
	switch runtime.GOOS {
	case goosDarwin:
		return exec.CommandContext(ctx, "cp", "-c", "-R", src, dst)
	case goosLinux:
		return exec.CommandContext(ctx, "cp", "--reflink=auto", "-R", src, dst)
	default:
		return exec.CommandContext(ctx, "cp", "-R", src, dst)
	}
}

// CloneDir copies a directory from src to dst using CoW (copy-on-write) when available.
// allowByteCopyFallback controls what happens when the CoW attempt errors outright:
// true retries with a plain byte-copy (CloneOnly modules, which have no install
// fallback); false propagates the error so the caller can install instead of
// silently paying for a byte-copy of a potentially huge directory.
func CloneDir(ctx context.Context, src, dst string, allowByteCopyFallback bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	if _, err := os.Stat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	if err := cowCopy(ctx, src, dst, allowByteCopyFallback); err != nil {
		return err
	}
	return nil
}

// CloneModule clones a module's dependency directories from one worktree to another.
// A CoW failure falls back to a byte-copy only when mod has no install
// fallback to prefer instead (allowByteCopyFallback); modules with an
// InstallCmd propagate the error instead so the caller can install rather
// than pay for an unbounded byte-copy (see cowEligible).
func CloneModule(ctx context.Context, srcWT, dstWT string, mod Module) error {
	if mod.Recursive {
		return cloneRecursive(ctx, srcWT, dstWT, mod)
	}
	return cloneSingle(ctx, srcWT, dstWT, mod)
}

// cloneFallbackAllowed reports whether mod has no install fallback to prefer
// over a byte-copy — true for CloneOnly modules (Go vendor, Gradle) and for
// modules with no InstallCmd at all, matching tryCloneFromExisting's own
// "always clone" cases in manager.go.
func cloneFallbackAllowed(mod Module) bool {
	return mod.CloneOnly || mod.InstallCmd == ""
}

func cloneSingle(ctx context.Context, srcWT, dstWT string, mod Module) error {
	src := filepath.Join(srcWT, mod.Dir)
	dst := filepath.Join(dstWT, mod.Dir)

	if _, err := os.Stat(src); err != nil {
		return err
	}

	if err := CloneDir(ctx, src, dst, cloneFallbackAllowed(mod)); err != nil {
		return err
	}

	return cloneExtraDirs(ctx, srcWT, dstWT, mod.ExtraDirs, cloneFallbackAllowed(mod))
}

func cloneRecursive(ctx context.Context, srcWT, dstWT string, mod Module) error {
	searchRoot := srcWT
	if mod.WorkDir != "" {
		searchRoot = filepath.Join(srcWT, mod.WorkDir)
	}

	var cloneErrs []error
	baseName := filepath.Base(mod.Dir)
	_ = filepath.WalkDir(searchRoot, walkCloneFunc(ctx, srcWT, dstWT, baseName, cloneFallbackAllowed(mod), &cloneErrs))

	// Skip extra dirs too once the walk has already bailed on cancellation;
	// there's no point starting more doomed clone attempts.
	if ctx.Err() == nil {
		if err := cloneExtraDirs(ctx, srcWT, dstWT, mod.ExtraDirs, cloneFallbackAllowed(mod)); err != nil {
			cloneErrs = append(cloneErrs, err)
		}
	}

	return errors.Join(cloneErrs...)
}

// walkCloneFunc returns a WalkDirFunc that clones directories matching baseName.
// Clone failures are appended to errs; walking continues regardless. Bails
// immediately once ctx is cancelled instead of issuing a doomed clone per
// remaining directory.
func walkCloneFunc(ctx context.Context, srcWT, dstWT, baseName string, allowByteCopyFallback bool, errs *[]error) fs.WalkDirFunc {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			*errs = append(*errs, err)
			return nil // keep walking
		}
		if ctx.Err() != nil {
			*errs = append(*errs, ctx.Err())
			return ctx.Err()
		}
		if !d.IsDir() {
			return nil
		}

		if d.Name() != baseName {
			return nil
		}

		relPath, _ := filepath.Rel(srcWT, path)
		return cloneIfParentExists(ctx, path, dstWT, relPath, allowByteCopyFallback, errs)
	}
}

// cloneIfParentExists clones srcPath to dstWT/relPath if the parent dir exists in dstWT.
// Clone failures are appended to errs and the directory is skipped.
func cloneIfParentExists(ctx context.Context, srcPath, dstWT, relPath string, allowByteCopyFallback bool, errs *[]error) error {
	dstParent := filepath.Join(dstWT, filepath.Dir(relPath))
	if _, err := os.Stat(dstParent); os.IsNotExist(err) {
		return filepath.SkipDir
	}

	dst := filepath.Join(dstWT, relPath)
	if err := CloneDir(ctx, srcPath, dst, allowByteCopyFallback); err != nil {
		*errs = append(*errs, fmt.Errorf("clone %s: %w", relPath, err))
		return filepath.SkipDir // continue walking other dirs
	}
	return filepath.SkipDir // don't descend into cloned dir
}

func cloneExtraDirs(ctx context.Context, srcWT, dstWT string, extraDirs []string, allowByteCopyFallback bool) error {
	for _, extra := range extraDirs {
		extraSrc := filepath.Join(srcWT, extra)
		extraDst := filepath.Join(dstWT, extra)
		if _, err := os.Stat(extraSrc); err == nil {
			if err := CloneDir(ctx, extraSrc, extraDst, allowByteCopyFallback); err != nil {
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

func cowCopy(ctx context.Context, src, dst string, allowByteCopyFallback bool) error {
	cmd := cowCopyCmd(ctx, src, dst)
	configureProcessGroup(cmd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	cowErr := cmdErr("cow copy", out, err)

	// Propagate the raw CoW failure instead of paying for a byte-copy the
	// caller didn't want — install-capable modules route this back to their
	// install command instead (see cowEligible's doc comment for why).
	if !allowByteCopyFallback {
		return cowErr
	}

	// Skip the fallback (and the RemoveAll before it) once cancelled; both
	// would fail fast anyway, so there's no value in attempting them.
	if ctx.Err() != nil {
		return errors.Join(cowErr, ctx.Err())
	}

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

	fallbackCmd := exec.CommandContext(ctx, "cp", "-R", src, dst)
	configureProcessGroup(fallbackCmd)
	if out, fbErr := fallbackCmd.CombinedOutput(); fbErr != nil {
		return errors.Join(cowErr, cmdErr("fallback copy", out, fbErr))
	}
	return nil
}
