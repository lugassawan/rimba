package e2e_test

import (
	"testing"
)

func TestCompletionShells(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir()

	tests := []struct {
		shell  string
		marker string
	}{
		{"bash", "complete"},
		{"zsh", "#compdef rimba"},
		{"fish", "complete -c rimba"},
	}

	for _, tc := range tests {
		t.Run(tc.shell, func(t *testing.T) {
			r := rimbaSuccess(t, dir, "completion", tc.shell)
			assertContains(t, r.Stdout, tc.marker)
		})
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	dir := t.TempDir()
	rimbaFail(t, dir, "completion", "fish2")
}
