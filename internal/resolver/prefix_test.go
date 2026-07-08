package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestPrefixTokenToString(t *testing.T) {
	tests := []struct {
		token      string
		wantPrefix string
		wantAlias  bool
		wantOk     bool
	}{
		{"fix", bugfixPrefix, true, true},
		{"bugfix", bugfixPrefix, false, true},
		{"feature", featurePrefix, false, true},
		{"hotfix", hotfixPrefix, false, true},
		{"docs", docsPrefix, false, true},
		{"test", testPrefix, false, true},
		{"chore", chorePrefix, false, true},
		{"nope", "", false, false},
		{"", "", false, false},
	}
	for _, tt := range tests {
		gotPrefix, gotAlias, gotOk := resolver.PrefixTokenToString(tt.token)
		if gotPrefix != tt.wantPrefix || gotAlias != tt.wantAlias || gotOk != tt.wantOk {
			t.Errorf("PrefixTokenToString(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.token, gotPrefix, gotAlias, gotOk, tt.wantPrefix, tt.wantAlias, tt.wantOk)
		}
	}
}

func TestValidPrefixTypeRejectsAlias(t *testing.T) {
	if resolver.ValidPrefixType("fix") {
		t.Error("ValidPrefixType(\"fix\") = true, want false — aliases must not widen the canonical prefix set")
	}
}
