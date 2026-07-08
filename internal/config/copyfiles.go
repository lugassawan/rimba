package config

import "strings"

// CandidateCopyFiles returns the curated candidates considered for
// auto-detected copy_files on fresh init: files matched by exact path, and
// dirs matched when they contain at least one git-ignored untracked file.
func CandidateCopyFiles() (files, dirs []string) {
	files = []string{
		".env", ".env.local", ".env.development.local", ".env.production.local",
		".envrc", ".tool-versions", ".python-version", ".dev.vars", ".npmrc",
	}
	dirs = []string{".vscode", ".idea", ".cursor", ".claude"}
	return files, dirs
}

// DetectCopyFiles maps git-ignored untracked paths against the candidate
// list and returns matched candidates in candidate order, deduped. A file
// candidate matches an exact path; a dir candidate matches if any ignored
// path sits under it.
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
