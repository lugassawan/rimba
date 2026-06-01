package deps

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// relocateVenv rewrites source-worktree absolute paths baked into a cloned .venv.
// It is the PostClone hook for Python presets.
func relocateVenv(srcWT, dstWT string, mod Module) error {
	if runtime.GOOS == goosWindows {
		return errors.New("venv relocation not supported on Windows")
	}

	// Build source and destination venv paths. We also try the symlink-resolved
	// form of srcWT because tools like `git worktree list` may resolve symlinks
	// (e.g. macOS /tmp → /private/tmp) while the venv baked in the unresolved path.
	srcVenv := filepath.Join(srcWT, mod.Dir)
	dstVenv := filepath.Join(dstWT, mod.Dir)

	// Collect all candidate old-paths to replace (deduplicated).
	oldPaths := dedupePaths(srcVenv, resolveOrKeep(srcWT), mod.Dir)
	newVenv := resolveOrKeep(dstWT)

	var errs []error

	// Rewrite bin/ scripts
	binDir := filepath.Join(dstVenv, "bin")
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(binDir, entry.Name())
			if err := rewriteAllPaths(path, oldPaths, filepath.Join(newVenv, mod.Dir)); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Rewrite pyvenv.cfg
	cfgPath := filepath.Join(dstVenv, "pyvenv.cfg")
	if _, err := os.Lstat(cfgPath); err == nil {
		if err := rewriteAllPaths(cfgPath, oldPaths, filepath.Join(newVenv, mod.Dir)); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// dedupePaths returns a deduplicated list of venv candidate paths to search.
// It includes srcVenv as-is plus the EvalSymlinks-resolved form, if different.
// This handles the macOS /tmp → /private/tmp case and similar platform aliases.
func dedupePaths(srcVenv, resolvedSrcWT, dir string) []string {
	resolvedVenv := filepath.Join(resolvedSrcWT, dir)
	if resolvedVenv == srcVenv {
		return []string{srcVenv}
	}
	// Include both: the raw form (may be what venv tools embedded) and the resolved form.
	return []string{srcVenv, resolvedVenv}
}

// rewriteAllPaths tries each old path in order, applying rewriteTextFile.
// Each call is independent (re-reads the file), so we iterate all candidates
// to handle files that contain multiple path forms (unlikely but safe).
func rewriteAllPaths(path string, oldPaths []string, newPath string) error {
	for _, old := range oldPaths {
		if err := rewriteTextFile(path, []byte(old), []byte(newPath)); err != nil {
			return err
		}
	}
	return nil
}

// rewriteTextFile does a byte replace of oldBytes → newBytes in path.
// Skips symlinks (lstat check) and binary files (NUL byte heuristic).
// Writes atomically with mode preserved.
//
// Note: there is a narrow TOCTOU window between the Lstat and ReadFile calls.
// A symlink created in that window would be followed by ReadFile. This is
// acceptable because the files live in a freshly-cloned .venv whose concurrent
// mutation is not part of the threat model.
func rewriteTextFile(path string, oldBytes, newBytes []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file disappeared — skip
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil // symlink — skip
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if isBinary(data) {
		return nil
	}

	if !strings.Contains(string(data), string(oldBytes)) {
		return nil // nothing to replace
	}

	newData := []byte(strings.ReplaceAll(string(data), string(oldBytes), string(newBytes)))

	return atomicWrite(path, newData, info.Mode())
}

// resolveOrKeep returns filepath.EvalSymlinks(p) if it succeeds, otherwise p.
func resolveOrKeep(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// isBinary returns true if data contains a NUL byte in the first 8 KB.
func isBinary(data []byte) bool {
	check := data
	if len(check) > 8192 {
		check = check[:8192]
	}
	return slices.Contains(check, byte(0))
}

// atomicWrite writes data to path atomically (temp + rename) preserving mode bits.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".rimba-relocate-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if err := writeTmp(tmp, data, mode); err != nil {
		_ = os.Remove(tmpPath) //nolint:gosec // best-effort cleanup
		return err
	}

	return os.Rename(tmpPath, path) //nolint:gosec // tmpPath from os.CreateTemp is safe
}

// writeTmp writes data to tmp, sets mode bits, and closes the file.
func writeTmp(tmp *os.File, data []byte, mode os.FileMode) error {
	defer tmp.Close()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	return tmp.Chmod(mode)
}
