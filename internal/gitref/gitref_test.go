package gitref_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/gitref"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errFrag string
	}{
		{name: "simple branch", input: "main"},
		{name: "namespaced branch", input: "feature/foo"},
		{name: "release tag with dots", input: "release-1.2"},
		{name: "alphanumeric", input: "abc123"},
		{name: "underscores", input: "my_branch"},
		{name: "leading dash", input: "-foo", wantErr: true, errFrag: "leading dash"},
		{name: "double option", input: "--upload-pack=x", wantErr: true, errFrag: "leading dash"},
		{name: "dotdot traversal", input: "../x", wantErr: true, errFrag: "contains .."},
		{name: "embedded dotdot", input: "a/../b", wantErr: true, errFrag: "contains .."},
		{name: "leading slash", input: "/abs/path", wantErr: true, errFrag: "leading slash"},
		{name: "semicolon injection", input: "a;b", wantErr: true},
		{name: "space injection", input: "a b", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := gitref.Validate(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate(%q) = nil, want error", tc.input)
				}
				if !errors.Is(err, gitref.ErrUnsafeRefName) {
					t.Errorf("Validate(%q): want errors.Is ErrUnsafeRefName, got %v", tc.input, err)
				}
				if tc.errFrag != "" {
					if msg := err.Error(); !contains(msg, tc.errFrag) {
						t.Errorf("Validate(%q) error = %q, want %q in message", tc.input, msg, tc.errFrag)
					}
				}
			} else if err != nil {
				t.Fatalf("Validate(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
