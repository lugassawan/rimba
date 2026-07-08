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
//  2. Part before "/" is a known canonical prefix → standard mode, sanitize rest
//  3. Part before "/" is a directory in repoRoot → monorepo (service, sanitized rest)
//  4. Part before "/" is a known alias (e.g. "fix") → standard mode, sanitize rest
//  5. Otherwise → standard mode, sanitize full input
//
// Canonical prefixes are checked before the directory match (unchanged,
// pre-existing precedence); aliases are checked after, so a real service
// directory that happens to share an alias's name (e.g. "fix") is not
// shadowed by the alias.
func ResolveTaskInput(input, repoRoot string, ps *resolver.PrefixSet) (service, task string) {
	candidate, rest := resolver.SplitServiceInput(input)
	if candidate == "" {
		return "", input
	}

	if ps.ValidType(candidate) {
		return "", resolver.SanitizeTask(rest)
	}

	dirPath := filepath.Join(repoRoot, candidate)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return candidate, resolver.SanitizeTask(rest)
	}

	if _, _, ok := ps.TokenToPrefix(candidate); ok {
		return "", resolver.SanitizeTask(rest)
	}

	return "", resolver.SanitizeTask(input)
}
