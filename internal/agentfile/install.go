package agentfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/fileutil"
)

// InstallGlobal creates or updates all agent instruction files under homeDir.
func InstallGlobal(homeDir string) ([]Result, error) {
	return installSpecs(homeDir, GlobalSpecs())
}

// UninstallGlobal removes rimba content from all user-level agent instruction files.
func UninstallGlobal(homeDir string) ([]Result, error) {
	return uninstallSpecs(homeDir, GlobalSpecs())
}

// InstallProject creates or updates all project-team agent instruction files under repoRoot.
func InstallProject(repoRoot string) ([]Result, error) {
	return installSpecs(repoRoot, ProjectSpecs())
}

// UninstallProject removes rimba content from all project-team agent instruction files.
func UninstallProject(repoRoot string) ([]Result, error) {
	return uninstallSpecs(repoRoot, ProjectSpecs())
}

// InstallLocal creates or updates all project-local agent instruction files and adds them to .gitignore.
func InstallLocal(repoRoot string) ([]Result, error) {
	specs := ProjectSpecs()
	results, err := installSpecs(repoRoot, specs)
	if err != nil {
		return results, err
	}
	for _, spec := range specs {
		if _, gitErr := fileutil.EnsureGitignore(repoRoot, spec.RelPath); gitErr != nil {
			return results, fmt.Errorf("gitignore %s: %w", spec.RelPath, gitErr)
		}
	}
	return results, nil
}

// UninstallLocal removes project-local agent instruction files and their .gitignore entries.
func UninstallLocal(repoRoot string) ([]Result, error) {
	specs := ProjectSpecs()
	results, err := uninstallSpecs(repoRoot, specs)
	if err != nil {
		return results, err
	}
	for _, spec := range specs {
		if _, gitErr := fileutil.RemoveGitignoreEntry(repoRoot, spec.RelPath); gitErr != nil {
			return results, fmt.Errorf("gitignore %s: %w", spec.RelPath, gitErr)
		}
	}
	return results, nil
}

func installSpecs(baseDir string, specs []Spec) ([]Result, error) {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		r, err := installOne(baseDir, spec)
		if err != nil {
			return results, fmt.Errorf("%s: %w", spec.RelPath, err)
		}
		results = append(results, r)
	}
	return results, nil
}

func uninstallSpecs(baseDir string, specs []Spec) ([]Result, error) {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		r, err := uninstallOne(baseDir, spec)
		if err != nil {
			return results, fmt.Errorf("%s: %w", spec.RelPath, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// ensureDir creates the directory (and parents) if possible.
// Returns false if creation fails (e.g. a parent path component is a regular file).
func ensureDir(dir string) bool {
	return os.MkdirAll(dir, 0750) == nil //nolint:gosec // dir needs to be accessible
}

func installOne(baseDir string, spec Spec) (Result, error) {
	path := filepath.Join(baseDir, spec.RelPath)

	if spec.Kind == KindWhole {
		return installWhole(path, spec)
	}
	return installBlock(path, spec)
}

func installWhole(path string, spec Spec) (Result, error) {
	if !ensureDir(filepath.Dir(path)) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	mode := os.FileMode(0644)
	action := actionCreated
	if info, err := os.Stat(path); err == nil {
		action = actionUpdated
		mode = info.Mode()
	}

	if err := os.WriteFile(path, []byte(spec.Content()), mode); err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
}

func installBlock(path string, spec Spec) (Result, error) {
	if !ensureDir(filepath.Dir(path)) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	if isCorruptBlock(content) {
		return Result{RelPath: spec.RelPath, Corrupt: true}, nil
	}

	action := actionCreated
	if content != "" {
		action = actionUpdated
	}

	merged, corrupt := mergeBlockContent(content, spec.Content())
	if corrupt {
		return Result{RelPath: spec.RelPath, Corrupt: true}, nil
	}

	if err := os.WriteFile(path, []byte(merged), mode); err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
}

// mergeBlockContent combines content with the rimba block, replacing any prior
// block. corrupt is true only if content changed shape since the caller's isCorruptBlock check.
func mergeBlockContent(content, block string) (merged string, corrupt bool) {
	if content == "" {
		return block + "\n", false
	}

	if containsBlock(content) {
		cleaned, ok := removeBlockChecked(content)
		if !ok {
			return "", true
		}
		content = cleaned
	}

	content = strings.TrimRight(content, "\n")
	if content == "" {
		return block + "\n", false
	}
	return content + "\n\n" + block + "\n", false
}

// removeBlockChecked strips the rimba block, reporting failure via ok instead of
// an error so corruption flows through Result.Corrupt, never a batch-aborting error.
func removeBlockChecked(content string) (cleaned string, ok bool) {
	cleaned, err := removeBlock(content)
	if err != nil {
		return "", false
	}
	return cleaned, true
}

func uninstallOne(baseDir string, spec Spec) (Result, error) {
	path := filepath.Join(baseDir, spec.RelPath)

	if spec.Kind == KindWhole {
		return uninstallWhole(path, spec)
	}
	return uninstallBlock(path, spec)
}

func uninstallWhole(path string, spec Spec) (Result, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	if err := os.Remove(path); err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("remove file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
}

func uninstallBlock(path string, spec Spec) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
		}
		return Result{RelPath: spec.RelPath}, fmt.Errorf("stat file: %w", err)
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	if isCorruptBlock(content) {
		return Result{RelPath: spec.RelPath, Corrupt: true}, nil
	}
	if !containsBlock(content) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	cleaned, ok := removeBlockChecked(content)
	if !ok {
		return Result{RelPath: spec.RelPath, Corrupt: true}, nil
	}
	if strings.TrimSpace(cleaned) == "" {
		if err := os.Remove(path); err != nil {
			return Result{RelPath: spec.RelPath}, fmt.Errorf("remove file: %w", err)
		}
		return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
	}

	if err := os.WriteFile(path, []byte(cleaned), info.Mode()); err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
}
