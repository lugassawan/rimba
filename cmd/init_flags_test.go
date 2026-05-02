package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitFlagValidationErrors(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
	restore := overrideNewRunner(r)
	defer restore()

	tests := []struct {
		name    string
		setup   func(*cobra.Command)
		wantErr string
	}{
		{
			name: "--local without --agents",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Bool(flagLocal, true, "")
			},
			wantErr: "--local requires --agents",
		},
		{
			name: "--uninstall without target",
			setup: func(cmd *cobra.Command) {
				cmd.Flags().Bool(flagUninstall, true, "")
			},
			wantErr: "--uninstall requires",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := newTestCmd()
			tt.setup(cmd)
			err := initCmd.RunE(cmd, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInitRepoRootError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return "", errors.New("not a git repository")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()

	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when repo root fails")
	}
}
