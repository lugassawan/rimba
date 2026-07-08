package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/resolver"
)

// GuardKnownPrefix hard-errors when branch was created under a custom prefix
// that is no longer configured; no-op if ps.HasCustom() is false or force is true.
func GuardKnownPrefix(ps *resolver.PrefixSet, branch, mainBranch string, force bool) error {
	if !ps.HasCustom() || force || !ps.IsOrphan(branch, mainBranch) {
		return nil
	}
	return errhint.WithFix(
		fmt.Errorf("branch %q uses a prefix that is no longer configured", branch),
		"re-add the prefix to [[resolver.prefix]] in .rimba/settings.toml, or remove this worktree",
	)
}
