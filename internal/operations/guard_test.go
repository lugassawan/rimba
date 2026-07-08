package operations

import (
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const (
	mainBranchGuard = "main"
	prefixTask      = "TASK-"
)

func TestGuardKnownPrefix(t *testing.T) {
	t.Parallel()

	customSet := resolver.NewPrefixSet([]resolver.PrefixSpec{{Prefix: testCustomPrefix}})

	tests := []struct {
		name       string
		ps         *resolver.PrefixSet
		branch     string
		mainBranch string
		force      bool
		wantErr    bool
	}{
		{
			name:       "no custom prefixes is always a no-op",
			ps:         resolver.DefaultPrefixSet(),
			branch:     "some-random-branch-name",
			mainBranch: mainBranchGuard,
			force:      false,
			wantErr:    false,
		},
		{
			name:       "custom prefixes configured and branch orphaned",
			ps:         customSet,
			branch:     "OLD-999",
			mainBranch: mainBranchGuard,
			force:      false,
			wantErr:    true,
		},
		{
			name:       "custom prefixes configured, orphaned, but force bypasses",
			ps:         customSet,
			branch:     "OLD-999",
			mainBranch: mainBranchGuard,
			force:      true,
			wantErr:    false,
		},
		{
			name:       "branch matches a still-configured custom prefix",
			ps:         customSet,
			branch:     branchProj123,
			mainBranch: mainBranchGuard,
			force:      false,
			wantErr:    false,
		},
		{
			name:       "main branch is always excluded even though it matches no prefix",
			ps:         customSet,
			branch:     mainBranchGuard,
			mainBranch: mainBranchGuard,
			force:      false,
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := GuardKnownPrefix(tc.ps, tc.branch, tc.mainBranch, tc.force)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("GuardKnownPrefix() = nil, want error")
				}
				if !strings.Contains(err.Error(), "To fix:") {
					t.Errorf("GuardKnownPrefix() error = %q, want a fix hint", err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("GuardKnownPrefix() = %v, want nil", err)
			}
		})
	}
}

// TestGuardKnownPrefixEndToEnd verifies a branch whose custom prefix is later
// removed from config becomes orphaned and GuardKnownPrefix hard-errors on it.
func TestGuardKnownPrefixEndToEnd(t *testing.T) {
	t.Parallel()

	repo := testutil.NewTestRepo(t)
	testutil.GitCmd(t, repo, "branch", branchProj123)
	worktreeDir := t.TempDir() + "/proj-123"
	testutil.GitCmd(t, repo, "worktree", "add", worktreeDir, branchProj123)

	// Config 1: PROJ- is configured. The branch is not orphaned.
	cfg1 := &config.Config{
		DefaultSource: mainBranchGuard,
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: testCustomPrefix}},
		},
	}
	ps1 := cfg1.PrefixSet()
	if !ps1.HasCustom() {
		t.Fatalf("cfg1: HasCustom() = false, want true")
	}
	if ps1.IsOrphan(branchProj123, mainBranchGuard) {
		t.Errorf("cfg1: IsOrphan(%q) = true, want false (prefix still configured)", branchProj123)
	}
	if err := GuardKnownPrefix(ps1, branchProj123, mainBranchGuard, false); err != nil {
		t.Errorf("GuardKnownPrefix with prefix still configured = %v, want nil", err)
	}

	// Config 2: PROJ- was removed, TASK- configured instead — PROJ-123 no
	// longer matches any registered prefix, so it is now orphaned.
	cfg2 := &config.Config{
		DefaultSource: mainBranchGuard,
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: prefixTask}},
		},
	}
	ps2 := cfg2.PrefixSet()
	if !ps2.HasCustom() {
		t.Fatalf("cfg2: HasCustom() = false, want true")
	}
	if !ps2.IsOrphan(branchProj123, mainBranchGuard) {
		t.Errorf("cfg2: IsOrphan(%q) = false, want true (PROJ- prefix removed)", branchProj123)
	}

	err := GuardKnownPrefix(ps2, branchProj123, mainBranchGuard, false)
	if err == nil {
		t.Fatalf("GuardKnownPrefix() = nil, want error for orphaned branch")
	}
	if !strings.Contains(err.Error(), "re-add the prefix") {
		t.Errorf("GuardKnownPrefix() error = %q, want it to mention re-adding the prefix", err.Error())
	}

	if err := GuardKnownPrefix(ps2, branchProj123, mainBranchGuard, true); err != nil {
		t.Errorf("GuardKnownPrefix(force=true) = %v, want nil", err)
	}

	// Config 3: no custom prefixes at all. HasCustom() is false, so the guard
	// is a no-op even though the branch would "orphan" under cfg1/cfg2.
	cfg3 := &config.Config{DefaultSource: mainBranchGuard}
	ps3 := cfg3.PrefixSet()
	if ps3.HasCustom() {
		t.Fatalf("cfg3: HasCustom() = true, want false (no [resolver] section)")
	}
	if err := GuardKnownPrefix(ps3, branchProj123, mainBranchGuard, false); err != nil {
		t.Errorf("GuardKnownPrefix() with no custom prefixes = %v, want nil (no-op parity guarantee)", err)
	}
	// Confirm this holds regardless of branch naming, including branches that
	// look nothing like any built-in prefix.
	if err := GuardKnownPrefix(ps3, "totally-unprefixed-branch", mainBranchGuard, false); err != nil {
		t.Errorf("GuardKnownPrefix() with no custom prefixes = %v, want nil regardless of branch naming", err)
	}
}
