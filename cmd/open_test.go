package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
)

const (
	porcelainWithLogin = "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
		wtFeatureLogin + "\nHEAD def456\nbranch refs/heads/feature/login\n"
)

func TestOpenPrintPath(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelainWithLogin, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	openCmd.SetOut(buf)
	openCmd.SetErr(buf)
	openCmd.SetArgs([]string{"login"})
	if err := openCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("openCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/wt/feature-login") {
		t.Errorf("output = %q, want path containing '/wt/feature-login'", out)
	}
}

func TestOpenWorktreeNotFound(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := openCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent worktree")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("error = %q, want 'worktree not found'", err.Error())
	}
}

// newResolveCmd creates a test cobra.Command pre-configured with shortcut flags and config context.
func newResolveCmd(cfg *config.Config, withVal string, ide, agent bool) *cobra.Command {
	cmd, _ := newTestCmd()
	if cfg != nil {
		cmd.SetContext(config.WithConfig(context.Background(), cfg))
	}
	cmd.Flags().String(flagWith, withVal, "")
	cmd.Flags().Bool(flagIDE, false, "")
	cmd.Flags().Bool(flagAgent, false, "")
	if ide {
		_ = cmd.Flags().Set(flagIDE, "true")
	}
	if agent {
		_ = cmd.Flags().Set(flagAgent, "true")
	}
	return cmd
}

func TestResolveOpenCommand(t *testing.T) {
	cfgFull := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: "main",
		Open:          map[string]string{"ide": "code .", "agent": "claude", "test": "npm test"},
	}
	cfgNoOpen := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cfgEmpty := &config.Config{WorktreeDir: "../wt", DefaultSource: "main", Open: map[string]string{"empty": ""}}

	tests := []struct {
		name       string
		cfg        *config.Config
		withVal    string
		ide        bool
		agent      bool
		inlineArgs []string
		wantArgs   []string
		wantErr    string
	}{
		{
			name: "--with resolves shortcut", cfg: cfgFull,
			withVal: "test", wantArgs: []string{"npm", "test"},
		},
		{
			name: "--ide resolves to ide shortcut", cfg: cfgFull,
			ide: true, wantArgs: []string{"code", "."},
		},
		{
			name: "--agent resolves to agent shortcut", cfg: cfgFull,
			agent: true, wantArgs: []string{"claude"},
		},
		{
			name: "shortcut not found lists available", cfg: cfgFull,
			withVal: "missing", wantErr: "agent, ide, test",
		},
		{
			name: "no open config section", cfg: cfgNoOpen,
			withVal: "ide", wantErr: "no [open] section",
		},
		{
			name: "empty shortcut value", cfg: cfgEmpty,
			withVal: "empty", wantErr: "empty value",
		},
		{
			name: "shortcut with inline args errors", cfg: cfgFull,
			withVal: "test", inlineArgs: []string{"extra"}, wantErr: "cannot combine",
		},
		{
			name: "no shortcut no args returns nil",
		},
		{
			name:       "inline args passed through",
			inlineArgs: []string{"pwd"}, wantArgs: []string{"pwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newResolveCmd(tt.cfg, tt.withVal, tt.ide, tt.agent)
			args, err := resolveOpenCommand(cmd, tt.inlineArgs)
			assertResolveResult(t, args, err, tt.wantArgs, tt.wantErr)
		})
	}
}

// assertResolveResult validates the output of resolveOpenCommand against expected args or error.
func assertResolveResult(t *testing.T, args []string, err error, wantArgs []string, wantErr string) {
	t.Helper()
	if wantErr != "" {
		if err == nil {
			t.Fatalf("expected error containing %q, got nil", wantErr)
		}
		if !strings.Contains(err.Error(), wantErr) {
			t.Errorf("error = %q, want substring %q", err.Error(), wantErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestAvailableShortcuts(t *testing.T) {
	m := map[string]string{"zed": "z", "alpha": "a", "mid": "m"}
	got := availableShortcuts(m)
	if got != "alpha, mid, zed" {
		t.Errorf("availableShortcuts = %q, want %q", got, "alpha, mid, zed")
	}
}

func TestFlagForShortcut(t *testing.T) {
	if f := flagForShortcut(true, false); f != flagIDE {
		t.Errorf("flagForShortcut(true, false) = %q, want %q", f, flagIDE)
	}
	if f := flagForShortcut(false, true); f != flagAgent {
		t.Errorf("flagForShortcut(false, true) = %q, want %q", f, flagAgent)
	}
	if f := flagForShortcut(false, false); f != flagWith {
		t.Errorf("flagForShortcut(false, false) = %q, want %q", f, flagWith)
	}
}
