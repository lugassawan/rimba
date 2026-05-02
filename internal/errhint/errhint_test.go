package errhint_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/errhint"
)

var errSentinel = errors.New("sentinel boom")

func TestWithFix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		fix        string
		wantNil    bool
		wantSubstr []string
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			fix:     "do the thing",
			wantNil: true,
		},
		{
			name: "wraps error message with fix hint",
			err:  errors.New("something failed"),
			fix:  "run rimba init",
			wantSubstr: []string{
				"something failed",
				"To fix: run rimba init",
			},
		},
		{
			name: "preserves sentinel for errors.Is",
			err:  errSentinel,
			fix:  "retry",
			wantSubstr: []string{
				"sentinel boom",
				"To fix: retry",
			},
		},
		{
			name: "exec provide --all hint",
			err:  errors.New("provide --all or --type to select worktrees"),
			fix:  "run: rimba exec --all <cmd>  OR  rimba exec --type <prefix> <cmd>",
			wantSubstr: []string{
				"provide --all or --type",
				"To fix: run: rimba exec --all",
			},
		},
		{
			name: "worktree not found hint",
			err:  errors.New(`worktree not found for task "mytask"`),
			fix:  "run: rimba list  to see available worktrees",
			wantSubstr: []string{
				"worktree not found",
				"To fix: run: rimba list",
			},
		},
		{
			name: "not a git repository hint",
			err:  errors.New("not a git repository"),
			fix:  "run from inside a git repository, or run: git init",
			wantSubstr: []string{
				"not a git repository",
				"To fix: run from inside a git repository",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := errhint.WithFix(tc.err, tc.fix)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil error, got nil")
			}
			msg := got.Error()
			for _, sub := range tc.wantSubstr {
				if !strings.Contains(msg, sub) {
					t.Errorf("error %q missing substring %q", msg, sub)
				}
			}
		})
	}
}

func TestWithFixPreservesSentinel(t *testing.T) {
	t.Parallel()

	wrapped := errhint.WithFix(errSentinel, "try again")
	if !errors.Is(wrapped, errSentinel) {
		t.Fatalf("errors.Is failed to match sentinel through wrap: %v", wrapped)
	}
	if unwrapped := errors.Unwrap(wrapped); !errors.Is(unwrapped, errSentinel) {
		t.Fatalf("errors.Unwrap = %v, want %v", unwrapped, errSentinel)
	}
}
