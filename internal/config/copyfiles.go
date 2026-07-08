package config

import "strings"

// CandidateCopyFiles returns the curated files and dirs considered for
// copy_files auto-detection on rimba init.
func CandidateCopyFiles() (files, dirs []string) {
	files = []string{
		".env", ".env.local",
		".env.development", ".env.development.local",
		".env.production", ".env.production.local",
		".env.test", ".env.test.local",
		".envrc", ".tool-versions", ".python-version", ".dev.vars", ".npmrc",
	}
	dirs = []string{".vscode", ".idea", ".cursor", ".claude"}
	return files, dirs
}

// DetectCopyFiles matches candidates against ignored, in candidate order: a
// file matches by exact path, a dir matches if any ignored path sits under it.
func DetectCopyFiles(ignored []string) []string {
	files, dirs := CandidateCopyFiles()
	ignoredSet := make(map[string]bool, len(ignored))
	for _, p := range ignored {
		ignoredSet[p] = true
	}

	var detected []string
	for _, f := range files {
		if ignoredSet[f] {
			detected = append(detected, f)
		}
	}
	for _, d := range dirs {
		if dirHasIgnoredFile(d, ignored) {
			detected = append(detected, d)
		}
	}
	return detected
}

// dirHasIgnoredFile reports whether any ignored path sits under dir/.
func dirHasIgnoredFile(dir string, ignored []string) bool {
	prefix := dir + "/"
	for _, p := range ignored {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
