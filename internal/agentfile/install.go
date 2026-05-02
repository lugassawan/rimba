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

	action := actionCreated
	if _, err := os.Stat(path); err == nil {
		action = actionUpdated
	}

	if err := os.WriteFile(path, []byte(spec.Content()), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
}

func installBlock(path string, spec Spec) (Result, error) {
	if !ensureDir(filepath.Dir(path)) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	block := spec.Content()
	action := actionCreated

	if content != "" {
		if containsBlock(content) {
			content = removeBlock(content)
		}
		action = actionUpdated
		// Append block to existing content (with removed old block if any)
		content = strings.TrimRight(content, "\n")
		if content != "" {
			content = content + "\n\n" + block + "\n"
		} else {
			content = block + "\n"
		}
	} else {
		content = block + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
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
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
		}
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	if !containsBlock(content) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	cleaned := removeBlock(content)
	if strings.TrimSpace(cleaned) == "" {
		if err := os.Remove(path); err != nil {
			return Result{RelPath: spec.RelPath}, fmt.Errorf("remove file: %w", err)
		}
		return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
	}

	if err := os.WriteFile(path, []byte(cleaned), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
}
