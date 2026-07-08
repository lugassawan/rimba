package resolver_test

import (
	"slices"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestNewPrefixSetNilParityWithAllPrefixes(t *testing.T) {
	got := resolver.NewPrefixSet(nil).Strip()
	want := resolver.AllPrefixes()
	if !slices.Equal(got, want) {
		t.Errorf("NewPrefixSet(nil).Strip() = %v, want %v (parity with AllPrefixes())", got, want)
	}
}

func TestNewPrefixSetNilParityWithPrefixTokenToString(t *testing.T) {
	tests := []string{"feature", "bugfix", "hotfix", "docs", "test", "chore", "fix", "nope", ""}
	set := resolver.NewPrefixSet(nil)
	for _, token := range tests {
		wantPrefix, wantAlias, wantOk := resolver.PrefixTokenToString(token)
		gotPrefix, gotAlias, gotOk := set.TokenToPrefix(token)
		if gotPrefix != wantPrefix || gotAlias != wantAlias || gotOk != wantOk {
			t.Errorf("TokenToPrefix(%q) = (%q, %v, %v), want (%q, %v, %v) (parity with PrefixTokenToString)",
				token, gotPrefix, gotAlias, gotOk, wantPrefix, wantAlias, wantOk)
		}
	}
}

func TestNewPrefixSetNilParityWithValidPrefixType(t *testing.T) {
	tests := []string{"feature", "bugfix", "hotfix", "docs", "test", "chore", "fix", "nope", ""}
	set := resolver.NewPrefixSet(nil)
	for _, token := range tests {
		want := resolver.ValidPrefixType(token)
		got := set.ValidType(token)
		if got != want {
			t.Errorf("ValidType(%q) = %v, want %v (parity with ValidPrefixType)", token, got, want)
		}
	}
}

func TestNewPrefixSetFoldsAliasesIntoExistingBuiltin(t *testing.T) {
	set := resolver.NewPrefixSet([]resolver.PrefixSpec{
		{Prefix: bugfixPrefix, Aliases: []string{"defect"}},
	})

	gotPrefix, gotAlias, gotOk := set.TokenToPrefix("defect")
	if gotPrefix != bugfixPrefix || !gotAlias || !gotOk {
		t.Errorf("TokenToPrefix(%q) = (%q, %v, %v), want (%q, true, true)", "defect", gotPrefix, gotAlias, gotOk, bugfixPrefix)
	}

	strip := set.Strip()
	count := 0
	for _, p := range strip {
		if p == bugfixPrefix {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Strip() contains %q %d times, want exactly once (got %v)", bugfixPrefix, count, strip)
	}
}

func TestNewPrefixSetRegistersBrandNewPrefix(t *testing.T) {
	set := resolver.NewPrefixSet([]resolver.PrefixSpec{
		{Prefix: "PROJ-", Aliases: []string{"proj"}},
	})

	strip := set.Strip()
	if !slices.Contains(strip, "PROJ-") {
		t.Errorf("Strip() = %v, want to contain %q", strip, "PROJ-")
	}

	gotPrefix, gotAlias, gotOk := set.TokenToPrefix("proj")
	if gotPrefix != "PROJ-" || !gotAlias || !gotOk {
		t.Errorf(`TokenToPrefix("proj") = (%q, %v, %v), want ("PROJ-", true, true)`, gotPrefix, gotAlias, gotOk)
	}

	if !set.HasCustom() {
		t.Error("HasCustom() = false, want true after registering a new prefix")
	}
}

func TestPrefixSetTypeName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"built-in with trailing slash", featurePrefix, "feature"},
		{"slash-less custom prefix", "PROJ-", "PROJ-"},
	}
	set := resolver.DefaultPrefixSet()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := set.TypeName(tt.prefix)
			if got != tt.want {
				t.Errorf("TypeName(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestPrefixSetTypeToPrefix(t *testing.T) {
	set := resolver.NewPrefixSet([]resolver.PrefixSpec{
		{Prefix: "PROJ-", Aliases: []string{"proj"}},
	})

	tests := []struct {
		name     string
		typeName string
		want     string
	}{
		{"built-in", "feature", featurePrefix},
		{"slash-less custom prefix round-trips", "PROJ-", "PROJ-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := set.TypeToPrefix(tt.typeName)
			if !ok || got != tt.want {
				t.Errorf("TypeToPrefix(%q) = (%q, %v), want (%q, true)", tt.typeName, got, ok, tt.want)
			}
		})
	}
}

func TestPrefixSetTypeToPrefixNotFound(t *testing.T) {
	set := resolver.NewPrefixSet([]resolver.PrefixSpec{
		{Prefix: "PROJ-", Aliases: []string{"proj"}},
	})

	if got, ok := set.TypeToPrefix("nonexistent"); ok {
		t.Errorf("TypeToPrefix(%q) = (%q, true), want ok=false", "nonexistent", got)
	}
}

func TestPrefixSetIsOrphan(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		mainBranch string
		want       bool
	}{
		{"branch under registered prefix", featurePrefix + taskMyTask, "main", false},
		{"branch under unregistered prefix", "PROJ-" + taskMyTask, "main", true},
		{"branch equals main, prefix matches", featurePrefix + taskMyTask, featurePrefix + taskMyTask, false},
		{"branch equals main, no prefix matches at all", "main", "main", false},
	}
	set := resolver.DefaultPrefixSet()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := set.IsOrphan(tt.branch, tt.mainBranch)
			if got != tt.want {
				t.Errorf("IsOrphan(%q, %q) = %v, want %v", tt.branch, tt.mainBranch, got, tt.want)
			}
		})
	}
}

func TestPrefixSetHasCustom(t *testing.T) {
	if resolver.DefaultPrefixSet().HasCustom() {
		t.Error("DefaultPrefixSet().HasCustom() = true, want false")
	}

	custom := resolver.NewPrefixSet([]resolver.PrefixSpec{
		{Prefix: "PROJ-", Aliases: []string{"proj"}},
	})
	if !custom.HasCustom() {
		t.Error("NewPrefixSet with a custom spec: HasCustom() = false, want true")
	}
}
