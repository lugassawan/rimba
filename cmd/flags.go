package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

// validateTypeFilter returns nil for empty input or a recognized prefix; otherwise an error
// listing the valid types. Shared by exec and list.
func validateTypeFilter(typeFilter string) error {
	if typeFilter == "" || resolver.ValidPrefixType(typeFilter) {
		return nil
	}
	valid := make([]string, 0, len(resolver.AllPrefixes()))
	for _, p := range resolver.AllPrefixes() {
		valid = append(valid, strings.TrimSuffix(p, "/"))
	}
	return fmt.Errorf("invalid type %q; valid types: %s", typeFilter, strings.Join(valid, ", "))
}

// typeFilterCompletion returns a cobra.CompletionFunc that completes against resolver.AllPrefixes().
func typeFilterCompletion() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var types []string
		for _, p := range resolver.AllPrefixes() {
			t := strings.TrimSuffix(p, "/")
			if strings.HasPrefix(t, toComplete) {
				types = append(types, t)
			}
		}
		return types, cobra.ShellCompDirectiveNoFileComp
	}
}
