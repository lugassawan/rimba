//go:build unix

package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/proc"
)

// shellRunResult captures ShellRunner's exitCode/err tuple for cross-goroutine assertions.
type shellRunResult struct {
	exitCode int
	err      error
}

// TestShellRunnerReapsGrandchildOnCancel proves cancellation reaps the whole
// forked process tree (e.g. npm/go test children), not just the direct "sh" child.
func TestShellRunnerReapsGrandchildOnCancel(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	pidFile := filepath.Join(dir, "pid")

	command := "sleep 30 & echo $! > " + pidFile + "; touch " + started + "; wait"

	ctx, cancel := context.WithCancel(context.Background())
	run := ShellRunner()

	done := make(chan shellRunResult, 1)
	go func() {
		_, _, exitCode, err := run(ctx, dir, command)
		done <- shellRunResult{exitCode, err}
	}()

	waitForFile(t, started, "shell command never started")
	grandchildPID := readPID(t, pidFile)

	cancel()

	var result shellRunResult
	select {
	case result = <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("ShellRunner did not return within 8s of cancellation; grandchild may be holding the output pipe open")
	}
	if result.exitCode >= 0 || result.err != nil {
		t.Errorf("exitCode=%d err=%v, want a signal-killed result classifyResult can label Cancelled", result.exitCode, result.err)
	}

	waitUntilDead(t, grandchildPID, 2*time.Second)
}

// TestConfigureProcessGroupGracefulSIGTERM proves a grandchild that honors
// SIGTERM is reaped by the group signal alone, well before the SIGKILL grace expires.
func TestConfigureProcessGroupGracefulSIGTERM(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	pidFile := filepath.Join(dir, "pid")
	command := "sleep 30 & echo $! > " + pidFile + "; touch " + started + "; wait"

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cleanup := configureProcessGroup(cmd, 2*time.Second)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForFile(t, started, "script never started")
	grandchildPID := readPID(t, pidFile)

	cancel()
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		cleanup()
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cmd.Wait did not return within 3s")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("graceful SIGTERM took %s, want well under the 2s SIGKILL grace", elapsed)
	}
	waitUntilDead(t, grandchildPID, time.Second)
}

// TestConfigureProcessGroupEscalatesToSIGKILL proves a group that keeps
// respawning children past a TERM-ignoring shell is reaped by the SIGKILL backstop.
func TestConfigureProcessGroupEscalatesToSIGKILL(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	command := "trap '' TERM; touch " + started + "; while true; do sleep 1; done"

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	grace := 200 * time.Millisecond
	cleanup := configureProcessGroup(cmd, grace)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForFile(t, started, "script never started")
	pid := cmd.Process.Pid

	cancel()
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		cleanup()
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(grace + 3*time.Second):
		t.Fatal("cmd.Wait did not return after the SIGKILL escalation window")
	}
	if elapsed := time.Since(start); elapsed < grace {
		t.Errorf("escalation fired too early (%s < grace %s); SIGTERM should have been ignored", elapsed, grace)
	}
	if proc.Alive(pid) {
		t.Errorf("pid %d still alive after SIGKILL escalation", pid)
	}
}

// waitUntilDead polls proc.Alive(pid) until false or timeout elapses. A
// sibling process's death isn't observable the instant cmd.Wait() returns for
// its parent, so a single-shot check races; polling avoids that flake.
func waitUntilDead(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for proc.Alive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("pid %d still alive %s after signal", pid, timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readPID reads and parses a PID written to path by a shell script.
func readPID(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	return pid
}

// waitForFile polls until path exists or fails the test with msg.
func waitForFile(t *testing.T, path, msg string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
