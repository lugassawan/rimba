package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestHasExplicitPrefixFlag(t *testing.T) {
	t.Run("no flag set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		addPrefixFlags(cmd)
		if hasExplicitPrefixFlag(cmd) {
			t.Error("expected false when no flag set")
		}
	})

	t.Run("bugfix flag set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		addPrefixFlags(cmd)
		if err := cmd.Flags().Set("bugfix", "true"); err != nil {
			t.Fatal(err)
		}
		if !hasExplicitPrefixFlag(cmd) {
			t.Error("expected true when bugfix flag set")
		}
	})
}

func TestResolvedPrefixString(t *testing.T) {
	tests := []struct {
		name string
		flag string // flag to set (empty = default/feature)
		want string
	}{
		{"default_feature", "", "feature/"},
		{"bugfix", "bugfix", "bugfix/"},
		{"hotfix", "hotfix", "hotfix/"},
		{"docs", "docs", "docs/"},
		{"test", "test", "test/"},
		{"chore", "chore", "chore/"},
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

			got := resolvedPrefixString(cmd)
			if got != tt.want {
				t.Errorf("resolvedPrefixString() = %q, want %q", got, tt.want)
			}
		})
	}
}
