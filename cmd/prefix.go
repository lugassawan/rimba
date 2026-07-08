package cmd

import (
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

// prefixFlag maps a CLI flag name to its resolver.PrefixType and description.
// Alias marks flags that resolve to a canonical type under a different name
// (e.g. --fix is an alias for --bugfix) rather than being canonical themselves.
type prefixFlag struct {
	Flag  string
	Type  resolver.PrefixType
	Desc  string
	Alias bool
}

// prefixFlags lists the non-default prefix types available as boolean flags.
// "feature" is the default and does not need a flag.
var prefixFlags = []prefixFlag{
	{"bugfix", resolver.PrefixBugfix, "fixing minor bugs that are part of the normal workflow", false},
	{"hotfix", resolver.PrefixHotfix, "urgent fixes that need to be patched directly in production", false},
	{"docs", resolver.PrefixDocs, "changes related to documentation", false},
	{"test", resolver.PrefixTest, "experiments or new tests that might not be merged", false},
	{"chore", resolver.PrefixChore, "non-code tasks like dependency updates", false},
	{"fix", resolver.PrefixBugfix, "alias for --bugfix", true},
}

// addPrefixFlags registers all prefix boolean flags on cmd and marks them mutually exclusive.
func addPrefixFlags(cmd *cobra.Command) {
	names := make([]string, len(prefixFlags))
	for i, pf := range prefixFlags {
		cmd.Flags().Bool(pf.Flag, false, pf.Desc)
		names[i] = pf.Flag
	}
	cmd.MarkFlagsMutuallyExclusive(names...)
}

// prefixSelection is the resolved outcome of scanning a command's prefix flags.
type prefixSelection struct {
	Prefix   string
	Explicit bool
	Alias    bool
}

// resolvePrefixSelection checks which prefix flag is set and returns the corresponding
// branch prefix string (e.g. "bugfix/"), whether a flag was explicitly set, and whether
// that flag was an alias (e.g. --fix). Falls back to the default ("feature/") when no
// flag is set.
func resolvePrefixSelection(cmd *cobra.Command) prefixSelection {
	for _, pf := range prefixFlags {
		if set, _ := cmd.Flags().GetBool(pf.Flag); set {
			s, _ := resolver.PrefixString(pf.Type)
			return prefixSelection{Prefix: s, Explicit: true, Alias: pf.Alias}
		}
	}
	s, _ := resolver.PrefixString(resolver.DefaultPrefixType)
	return prefixSelection{Prefix: s}
}
