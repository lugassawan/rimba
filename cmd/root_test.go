package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
)

func TestPersistentPreRunESkipsCompletion(t *testing.T) {
	preRunE := rootCmd.PersistentPreRunE

	for _, name := range []string{"completion", "__complete"} {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{Use: name}
			if err := preRunE(cmd, nil); err != nil {
				t.Fatalf("expected nil error for %q command, got %v", name, err)
			}
		})
	}
}

func TestPersistentPreRunESkipsAnnotated(t *testing.T) {
	preRunE := rootCmd.PersistentPreRunE

	cmd := &cobra.Command{
		Use:         "skip-me",
		Annotations: map[string]string{"skipConfig": "true"},
	}
	if err := preRunE(cmd, nil); err != nil {
		t.Fatalf("expected nil for annotated command, got %v", err)
	}
}

func TestPersistentPreRunESkipsAnnotatedParent(t *testing.T) {
	preRunE := rootCmd.PersistentPreRunE

	parent := &cobra.Command{
		Use:         "parent",
		Annotations: map[string]string{"skipConfig": "true"},
	}
	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)

	if err := preRunE(child, nil); err != nil {
		t.Fatalf("expected nil for child of annotated parent, got %v", err)
	}
}

func TestPersistentPreRunERepoRootError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	preRunE := rootCmd.PersistentPreRunE
	cmd := &cobra.Command{Use: "test-cmd"}

	err := preRunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error from RepoRoot")
	}
}

func TestPersistentPreRunEConfigLoadError(t *testing.T) {
	dir := t.TempDir() // no .rimba.toml file

	r := repoRootRunner(dir, nil)
	restore := overrideNewRunner(r)
	defer restore()

	preRunE := rootCmd.PersistentPreRunE
	cmd := &cobra.Command{Use: "test-cmd"}
	cmd.SetContext(context.Background())

	err := preRunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error from config.Load")
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Errorf("error = %q, want substring %q", err.Error(), "config not found")
	}
}

func TestPersistentPreRunESuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "../worktrees", DefaultSource: "main"}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, nil)
	restore := overrideNewRunner(r)
	defer restore()

	preRunE := rootCmd.PersistentPreRunE
	cmd := &cobra.Command{Use: "test-cmd"}
	cmd.SetContext(context.Background())

	if err := preRunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded := config.FromContext(cmd.Context())
	if loaded == nil {
		t.Fatal("expected config in context after successful PreRunE")
	}
	if loaded.DefaultSource != "main" {
		t.Errorf("DefaultSource = %q, want %q", loaded.DefaultSource, "main")
	}
}

func TestRootHelpFunctionSubcommand(t *testing.T) {
	sub := &cobra.Command{Use: "sub", Short: "a subcommand"}
	rootCmd.AddCommand(sub)
	defer rootCmd.RemoveCommand(sub)

	buf := new(strings.Builder)
	sub.SetOut(buf)
	sub.SetErr(buf)

	// The help function should not print banner for subcommands
	rootCmd.HelpFunc()(sub, nil)
	out := buf.String()

	if strings.Contains(out, "rimba") && strings.Contains(out, `\_`) {
		t.Error("banner should not be printed for subcommands")
	}
	if !strings.Contains(out, "a subcommand") {
		t.Errorf("expected subcommand help text, got %q", out)
	}
}
