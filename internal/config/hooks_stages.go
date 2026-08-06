package config

import (
	"fmt"
)

// PostCreateStages returns c.PostCreate normalized into a canonical list of
// stages: each stage's commands run concurrently, stages run in order. A flat
// list is split according to IsHooksParallel — one command per stage
// (serial) when false, one stage holding every command (parallel) when true.
// A nested list is returned as given; the nested shape alone implies
// parallelism, so IsHooksParallel is not consulted in that case.
func (c *Config) PostCreateStages() ([][]string, error) {
	stages, err := NormalizeHookStages(c.PostCreate, c.IsHooksParallel())
	if err != nil {
		return nil, fmt.Errorf("post_create: %w", err)
	}
	return stages, nil
}

// PostRenameStages is PostCreateStages for c.PostRename. Its flat form always
// normalizes to fully-serial stages, ignoring IsHooksParallel — post-rename
// hooks are not wired to [hooks] parallel (only rimba add's post-create hooks
// are). Only the nested/staged shape opts a post_rename config into
// parallelism.
func (c *Config) PostRenameStages() ([][]string, error) {
	stages, err := NormalizeHookStages(c.PostRename, false)
	if err != nil {
		return nil, fmt.Errorf("post_rename: %w", err)
	}
	return stages, nil
}

// NormalizeHookStages converts a hook-list value into a canonical
// [][]string of stages. raw may be nil; a native Go []string or [][]string
// (set directly by callers constructing a Config in code, e.g. tests and
// DefaultConfig); or the []any/[]any-of-[]any go-toml/v2 produces for an
// untyped array field loaded from a TOML file. Either representation may
// hold a flat array of strings (a plain hook list) or a nested array of
// string arrays (explicit stages).
//
// A flat list becomes one single-command stage per entry (serial) when
// flatParallel is false, or one stage holding every entry (parallel) when
// true. A nested list is returned as given, one stage per inner array,
// regardless of flatParallel — the nested shape is itself the parallelism
// declaration.
func NormalizeHookStages(raw any, flatParallel bool) ([][]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []string:
		if len(v) == 0 {
			return nil, nil
		}
		return flatStringsToStages(v, flatParallel), nil
	case [][]string:
		if len(v) == 0 {
			return nil, nil
		}
		return v, nil
	case []any:
		return normalizeAnyEntries(v, flatParallel)
	default:
		return nil, fmt.Errorf("must be an array of strings or an array of arrays of strings, got %T", raw)
	}
}

func normalizeAnyEntries(entries []any, flatParallel bool) ([][]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	switch entries[0].(type) {
	case string:
		return normalizeFlatEntries(entries, flatParallel)
	case []any:
		return normalizeNestedEntries(entries)
	default:
		return nil, fmt.Errorf("entry 0: must be a string or an array of strings, got %T", entries[0])
	}
}

func flatStringsToStages(cmds []string, flatParallel bool) [][]string {
	if flatParallel {
		return [][]string{cmds}
	}
	stages := make([][]string, len(cmds))
	for i, c := range cmds {
		stages[i] = []string{c}
	}
	return stages
}

func normalizeFlatEntries(entries []any, flatParallel bool) ([][]string, error) {
	cmds := make([]string, len(entries))
	for i, e := range entries {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("mixed flat/nested entries: entry %d is %T, want string", i, e)
		}
		cmds[i] = s
	}
	return flatStringsToStages(cmds, flatParallel), nil
}

func normalizeNestedEntries(entries []any) ([][]string, error) {
	stages := make([][]string, len(entries))
	for i, e := range entries {
		group, ok := e.([]any)
		if !ok {
			return nil, fmt.Errorf("mixed flat/nested entries: stage %d is %T, want an array of strings", i, e)
		}
		cmds := make([]string, len(group))
		for j, c := range group {
			s, ok := c.(string)
			if !ok {
				return nil, fmt.Errorf("stage %d, entry %d: must be a string, got %T (nesting deeper than 2 levels is not supported)", i, j, c)
			}
			cmds[j] = s
		}
		stages[i] = cmds
	}
	return stages, nil
}
