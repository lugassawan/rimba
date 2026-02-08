package cmd

import (
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

// prefixFlag maps a CLI flag name to its resolver.PrefixType.
type prefixFlag struct {
	Flag string
	Type resolver.PrefixType
}

// prefixFlags lists the non-default prefix types available as boolean flags.
// "feature" is the default and does not need a flag.
var prefixFlags = []prefixFlag{
	{"bugfix", resolver.PrefixBugfix},
	{"hotfix", resolver.PrefixHotfix},
	{"docs", resolver.PrefixDocs},
	{"test", resolver.PrefixTest},
	{"chore", resolver.PrefixChore},
}

// addPrefixFlags registers all prefix boolean flags on cmd and marks them mutually exclusive.
func addPrefixFlags(cmd *cobra.Command) {
	names := make([]string, len(prefixFlags))
	for i, pf := range prefixFlags {
		cmd.Flags().Bool(pf.Flag, false, "Use "+pf.Flag+"/ branch prefix")
		names[i] = pf.Flag
	}
	cmd.MarkFlagsMutuallyExclusive(names...)
}

// resolvedPrefixString checks which prefix flag is set and returns the corresponding
// branch prefix string (e.g. "bugfix/"). Falls back to the default ("feature/").
func resolvedPrefixString(cmd *cobra.Command) string {
	for _, pf := range prefixFlags {
		if set, _ := cmd.Flags().GetBool(pf.Flag); set {
			s, _ := resolver.PrefixString(pf.Type)
			return s
		}
	}
	s, _ := resolver.PrefixString(resolver.DefaultPrefixType)
	return s
}
