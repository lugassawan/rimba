package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
)

func TestValidateTypeFilterEmpty(t *testing.T) {
	if err := validateTypeFilter("", resolver.DefaultPrefixSet()); err != nil {
		t.Errorf("expected nil for empty type, got %v", err)
	}
}

func TestValidateTypeFilterValid(t *testing.T) {
	if err := validateTypeFilter(typeFeature, resolver.DefaultPrefixSet()); err != nil {
		t.Errorf("expected nil for valid type, got %v", err)
	}
}

func TestValidateTypeFilterInvalid(t *testing.T) {
	err := validateTypeFilter("nope", resolver.DefaultPrefixSet())
	if err == nil || !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected invalid type error, got %v", err)
	}
}

// TestValidateTypeFilterCustomPrefixWithoutSlash proves a custom prefix with
// no trailing slash (e.g. "PROJ-") is accepted by its type name.
func TestValidateTypeFilterCustomPrefixWithoutSlash(t *testing.T) {
	ps := resolver.NewPrefixSet([]resolver.PrefixSpec{{Prefix: "PROJ-"}})
	if err := validateTypeFilter("PROJ-", ps); err != nil {
		t.Errorf("expected nil for custom type 'PROJ-', got %v", err)
	}
}

func TestTypeFilterCompletionBuiltins(t *testing.T) {
	cmd, _ := newTestCmd()
	got, _ := typeFilterCompletion()(cmd, nil, "feat")
	if len(got) != 1 || got[0] != typeFeature {
		t.Errorf("completion(%q) = %v, want [%s]", "feat", got, typeFeature)
	}
}

// TestTypeFilterCompletionCustomPrefix proves a custom prefix's type name is
// offered by completion when sourced from the command's context config.
func TestTypeFilterCompletionCustomPrefix(t *testing.T) {
	cmd, _ := newTestCmd()
	cfg := &config.Config{
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "PROJ-"}},
		},
	}
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	got, _ := typeFilterCompletion()(cmd, nil, "PROJ")
	if len(got) != 1 || got[0] != "PROJ-" {
		t.Errorf("completion(%q) = %v, want [PROJ-]", "PROJ", got)
	}
}
