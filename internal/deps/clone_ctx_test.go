package deps

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCloneDirCancelledReturnsFast(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so cp never starts

	start := time.Now()
	err := CloneDir(ctx, src, dst)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("expected fast return on pre-cancelled ctx, took %v", elapsed)
	}
}

func TestCloneDirCancelledKillsChild(t *testing.T) {
	if runtime.GOOS != goosDarwin && runtime.GOOS != goosLinux {
		t.Skip("process-group kill semantics only asserted on darwin/linux")
	}

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	orig := cowCopyCmd
	cowCopyCmd = func(ctx context.Context, s, d string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	}
	t.Cleanup(func() { cowCopyCmd = orig })

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- CloneDir(ctx, src, dst)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected non-nil error when clone is cancelled mid-copy")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CloneDir did not return within 2s after cancellation")
	}
}

func TestCloneModuleRecursiveCancelledBailsEarly(t *testing.T) {
	srcWT := t.TempDir()
	dstWT := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the walk bails on its first entry

	mod := Module{Dir: DirNodeModules, Recursive: true}

	err := CloneModule(ctx, srcWT, dstWT, mod)
	if err == nil {
		t.Fatal("expected error when ctx is cancelled before recursive clone")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' in error, got: %v", err)
	}
}
