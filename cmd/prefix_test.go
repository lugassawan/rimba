package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolvePrefixSelection(t *testing.T) {
	tests := []struct {
		name         string
		flag         string // flag to set (empty = default/feature)
		wantPrefix   string
		wantExplicit bool
		wantAlias    bool
	}{
		{"default_feature", "", "feature/", false, false},
		{"bugfix", "bugfix", "bugfix/", true, false},
		{"hotfix", "hotfix", "hotfix/", true, false},
		{"docs", "docs", "docs/", true, false},
		{"test", "test", "test/", true, false},
		{"chore", "chore", "chore/", true, false},
		{"fix_alias", "fix", "bugfix/", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			addPrefixFlags(cmd)

			if tt.flag != "" {
				if err := cmd.Flags().Set(tt.flag, "true"); err != nil {
					t.Fatalf("setting flag %q: %v", tt.flag, err)
				}
			}

			got := resolvePrefixSelection(cmd)
			if got.Prefix != tt.wantPrefix || got.Explicit != tt.wantExplicit || got.Alias != tt.wantAlias {
				t.Errorf("resolvePrefixSelection() = %+v, want {Prefix: %q, Explicit: %v, Alias: %v}",
					got, tt.wantPrefix, tt.wantExplicit, tt.wantAlias)
			}
		})
	}
}

func TestFixBugfixMutuallyExclusive(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addPrefixFlags(cmd)
	if err := cmd.Flags().Set("fix", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("bugfix", "true"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.ValidateFlagGroups(); err == nil {
		t.Error("expected --fix and --bugfix to be mutually exclusive")
	}
}
