package cmd

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

// validateTypeFilter returns nil for empty input or a recognized prefix; otherwise an error
// listing the valid types. Shared by exec and list.
func validateTypeFilter(typeFilter string, ps *resolver.PrefixSet) error {
	if typeFilter == "" || ps.ValidType(typeFilter) {
		return nil
	}
	valid := strings.Join(ps.TypeNames(), ", ")
	return errhint.WithFix(
		fmt.Errorf("invalid type %q; valid types: %s", typeFilter, valid),
		"choose one of the configured prefix types: "+valid,
	)
}

// typeFilterCompletion returns a cobra.CompletionFunc that completes against the
// configured PrefixSet resolved from the command's context.
func typeFilterCompletion() cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ps := config.PrefixSetFromContext(cmdContext(cmd))
		var types []string
		for _, t := range ps.TypeNames() {
			if strings.HasPrefix(t, toComplete) {
				types = append(types, t)
			}
		}
		return types, cobra.ShellCompDirectiveNoFileComp
	}
}
