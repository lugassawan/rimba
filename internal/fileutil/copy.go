package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const copyErrFmt = "copy %s: %w"

// CopyEntries copies the listed files or directories from src directory to dst directory.
// Missing source entries are silently skipped. Returns the list of entries actually copied
// and the list of nested symlink paths (relative to src) that were skipped without being copied.
func CopyEntries(src, dst string, entries []string) (copied []string, skippedSymlinks []string, err error) {
	dstRoot, err := resolveDstRoot(dst)
	if err != nil {
		return nil, nil, err
	}
	copied = make([]string, 0, len(entries))
	for _, name := range entries {
		srcPath, err := ContainedJoin(src, name)
		if err != nil {
			return copied, skippedSymlinks, fmt.Errorf(copyErrFmt, name, err)
		}
		dstPath, err := ContainedJoin(dst, name)
		if err != nil {
			return copied, skippedSymlinks, fmt.Errorf(copyErrFmt, name, err)
		}

		ok, syms, copyErr := copyEntry(srcPath, dstPath, name, dstRoot)
		if copyErr != nil {
			return copied, skippedSymlinks, copyErr
		}
		if ok {
			copied = append(copied, name)
		}
		for _, p := range syms {
			rel, relErr := filepath.Rel(src, p)
			if relErr != nil {
				rel = p
			}
			skippedSymlinks = append(skippedSymlinks, rel)
		}
	}
	return copied, skippedSymlinks, nil
}

// SkippedEntries returns the entries from requested that are not in copied.
func SkippedEntries(requested, copied []string) []string {
	set := make(map[string]struct{}, len(copied))
	for _, c := range copied {
		set[c] = struct{}{}
	}
	var skipped []string
	for _, r := range requested {
		if _, ok := set[r]; !ok {
			skipped = append(skipped, r)
		}
	}
	return skipped
}

// copyEntry copies a single file or directory. Returns false if the source does not exist.
func copyEntry(srcPath, dstPath, name, dstRoot string) (bool, []string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf(copyErrFmt, name, err)
	}

	if info.IsDir() {
		syms, err := copyDir(srcPath, dstPath, dstRoot)
		if err != nil {
			return false, nil, fmt.Errorf(copyErrFmt, name, err)
		}
		return true, syms, nil
	}

	if err := assertContained(dstRoot, dstPath); err != nil {
		return false, nil, fmt.Errorf(copyErrFmt, name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return false, nil, fmt.Errorf(copyErrFmt, name, err)
	}
	if err := copyFile(srcPath, dstPath, dstRoot); err != nil {
		return false, nil, fmt.Errorf(copyErrFmt, name, err)
	}
	return true, nil, nil
}

func copyDir(src, dst, dstRoot string) ([]string, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if err := assertContained(dstRoot, dst); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	var skippedSymlinks []string
	for _, entry := range entries {
		syms, err := copyDirEntry(src, dst, dstRoot, entry)
		if err != nil {
			return nil, err
		}
		skippedSymlinks = append(skippedSymlinks, syms...)
	}
	return skippedSymlinks, nil
}

func copyDirEntry(src, dst, dstRoot string, entry os.DirEntry) ([]string, error) {
	if entry.Type()&os.ModeSymlink != 0 {
		return []string{filepath.Join(src, entry.Name())}, nil
	}
	srcPath := filepath.Join(src, entry.Name())
	dstPath := filepath.Join(dst, entry.Name())
	if entry.IsDir() {
		return copyDir(srcPath, dstPath, dstRoot)
	}
	return nil, copyFile(srcPath, dstPath, dstRoot)
}

func copyFile(src, dst, dstRoot string) (retErr error) {
	if err := assertContained(dstRoot, dst); err != nil {
		return err
	}
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(filepath.Clean(dst), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode()) //nolint:gosec // dst validated by assertContained (symlink-resolved) before write
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}

// resolveDstRoot resolves the symlink-canonical form of dst. If dst does not exist yet
// (git worktree add has not run), "" is returned so assertContained becomes a no-op —
// no symlinks can exist inside a non-existent directory. Non-ENOENT errors are surfaced.
func resolveDstRoot(dst string) (string, error) {
	resolved, err := filepath.EvalSymlinks(dst)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve dst %q: %w", dst, err)
	}
	return "", nil
}

// resolveExistingAncestor returns EvalSymlinks of the deepest existing ancestor of p.
// Walks up one component at a time while EvalSymlinks reports path-not-exist. The walk
// always terminates: filepath.Dir bottoms out at "/" which exists and resolves.
func resolveExistingAncestor(p string) (string, error) {
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		p = filepath.Dir(p)
	}
}

// assertContained verifies that the deepest existing ancestor of path resolves inside root
// (root must already be symlink-resolved). A symlinked directory component that redirects
// a write outside the destination tree is rejected with ErrPathEscapes.
// A blank root (dst did not exist at CopyEntries call time) is a no-op.
func assertContained(root, path string) error {
	if root == "" {
		return nil
	}
	resolved, err := resolveExistingAncestor(path)
	if err != nil {
		return err
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return fmt.Errorf("path %q resolves outside %q via symlink: %w", path, root, ErrPathEscapes)
	}
	return nil
}
