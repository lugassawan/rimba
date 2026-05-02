package agentfile

import (
	"os"
	"path/filepath"
)

// StatusGlobal checks the installation state of all user-level agent instruction files.
func StatusGlobal(homeDir string) []FileStatus {
	return checkSpecs(homeDir, GlobalSpecs())
}

// StatusProject checks the installation state of all project-team agent instruction files.
func StatusProject(repoRoot string) []FileStatus {
	return checkSpecs(repoRoot, ProjectSpecs())
}

func checkSpecs(baseDir string, specs []Spec) []FileStatus {
	statuses := make([]FileStatus, 0, len(specs))
	for _, spec := range specs {
		statuses = append(statuses, checkOne(baseDir, spec))
	}
	return statuses
}

func checkOne(baseDir string, spec Spec) FileStatus {
	path := filepath.Join(baseDir, spec.RelPath)

	if spec.Kind == KindWhole {
		_, err := os.Stat(path)
		return FileStatus{RelPath: spec.RelPath, Installed: err == nil}
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return FileStatus{RelPath: spec.RelPath, Installed: false}
	}
	return FileStatus{RelPath: spec.RelPath, Installed: containsBlock(string(existing))}
}
