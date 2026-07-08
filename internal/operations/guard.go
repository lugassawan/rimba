package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/resolver"
)

// GuardKnownPrefix hard-errors when branch was created under a custom prefix
// that is no longer configured. It is a no-op (nil) whenever ps.HasCustom() is
// false, force is true, branch == mainBranch, or branch matches a known prefix.
func GuardKnownPrefix(ps *resolver.PrefixSet, branch, mainBranch string, force bool) error {
	if !ps.HasCustom() || force || !ps.IsOrphan(branch, mainBranch) {
		return nil
	}
	return errhint.WithFix(
		fmt.Errorf("branch %q uses a prefix that is no longer configured", branch),
		"re-add the prefix to [[resolver.prefix]] in .rimba/settings.toml, or remove this worktree",
	)
}
