package operations

import (
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/resolver"
)

// ResolveTaskInput parses user input into service and task components.
// It validates that the candidate service directory exists in the repo root.
//
// Decision logic:
//  1. No "/" in input → standard mode ("", input)
//  2. Part before "/" is a known prefix → standard mode, sanitize rest
//  3. Part before "/" is a directory in repoRoot → monorepo (service, sanitized rest)
//  4. Otherwise → standard mode, sanitize full input
func ResolveTaskInput(input, repoRoot string) (service, task string) {
	candidate, rest := resolver.SplitServiceInput(input)
	if candidate == "" {
		return "", input
	}

	// Known prefix → standard mode, sanitize rest
	if resolver.ValidPrefixType(candidate) {
		return "", resolver.SanitizeTask(rest)
	}

	// Check if candidate is a valid service directory
	dirPath := filepath.Join(repoRoot, candidate)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return candidate, resolver.SanitizeTask(rest)
	}

	// Not a prefix, not a valid dir → treat full input as task
	return "", resolver.SanitizeTask(input)
}
