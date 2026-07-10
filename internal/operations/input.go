package operations

import (
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/resolver"
)

// TaskKind classifies how ClassifyTaskInput resolved its input.
type TaskKind int

const (
	// KindStandard means the input has no service component: no "/", a
	// valid prefix, or a known alias.
	KindStandard TaskKind = iota
	// KindService means the part before "/" is a real service directory.
	KindService
	// KindUnknownService means the part before "/" is neither a valid
	// prefix, a known alias, nor a real service directory.
	KindUnknownService
)

// TaskInput is the result of classifying a raw task input string.
type TaskInput struct {
	Service, Task string
	Kind          TaskKind
}

// ClassifyTaskInput parses user input into service and task components,
// reporting which decision branch produced the result. Prefer
// ResolveTaskInput unless you must distinguish an unknown service (see
// cmd/duplicate.go's --as guard).
//
// Decision logic:
//  1. No "/" in input → KindStandard ("", input)
//  2. Part before "/" is a known canonical prefix → KindStandard, sanitize rest
//  3. Part before "/" is a directory in repoRoot → KindService (service, sanitized rest)
//  4. Part before "/" is a known alias (e.g. "fix") → KindStandard, sanitize rest
//  5. Otherwise → KindUnknownService, sanitize full input
//
// Canonical prefixes are checked before the directory match (unchanged,
// pre-existing precedence); aliases are checked after, so a real service
// directory that happens to share an alias's name (e.g. "fix") is not
// shadowed by the alias.
func ClassifyTaskInput(input, repoRoot string, ps *resolver.PrefixSet) TaskInput {
	candidate, rest := resolver.SplitServiceInput(input)
	if candidate == "" {
		return TaskInput{Task: input, Kind: KindStandard}
	}

	if ps.ValidType(candidate) {
		return TaskInput{Task: resolver.SanitizeTask(rest), Kind: KindStandard}
	}

	dirPath := filepath.Join(repoRoot, candidate)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return TaskInput{Service: candidate, Task: resolver.SanitizeTask(rest), Kind: KindService}
	}

	if _, _, ok := ps.TokenToPrefix(candidate); ok {
		return TaskInput{Task: resolver.SanitizeTask(rest), Kind: KindStandard}
	}

	return TaskInput{Task: resolver.SanitizeTask(input), Kind: KindUnknownService}
}

// ResolveTaskInput parses user input into service and task components.
// It validates that the candidate service directory exists in the repo root.
// It is a thin projection over ClassifyTaskInput for callers that don't need
// to distinguish an unknown service from a plain task name.
func ResolveTaskInput(input, repoRoot string, ps *resolver.PrefixSet) (service, task string) {
	r := ClassifyTaskInput(input, repoRoot, ps)
	return r.Service, r.Task
}
