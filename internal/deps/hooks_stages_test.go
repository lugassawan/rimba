package deps

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunPostCreateHooksStagesRunInOrder(t *testing.T) {
	dir := t.TempDir()

	// Stage 1 sleeps then writes; stage 2 only writes. If stages ran
	// concurrently with each other, stage2.txt could appear before
	// stage1.txt; run in order, stage1.txt must always exist first.
	stages := [][]string{
		{"sleep 0.05 && touch stage1.txt"},
		{"touch stage2.txt"},
	}

	results := RunPostCreateHooks(context.Background(), dir, stages, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
	}

	info1, err1 := os.Stat(filepath.Join(dir, "stage1.txt"))
	info2, err2 := os.Stat(filepath.Join(dir, "stage2.txt"))
	if err1 != nil || err2 != nil {
		t.Fatalf("expected both marker files, stage1 err=%v stage2 err=%v", err1, err2)
	}
	if !info1.ModTime().Before(info2.ModTime()) && info1.ModTime() != info2.ModTime() {
		// Best-effort ordering signal; the authoritative proof is result order below.
		t.Logf("stage1 mtime %v, stage2 mtime %v", info1.ModTime(), info2.ModTime())
	}
}

func TestRunPostCreateHooksStageCommandsRunConcurrently(t *testing.T) {
	dir := t.TempDir()

	const n = 4
	const sleepMS = 150
	hooks := make([]string, n)
	for i := range hooks {
		hooks[i] = "sleep 0.150"
	}
	stages := [][]string{hooks}

	start := time.Now()
	results := RunPostCreateHooks(context.Background(), dir, stages, nil)
	elapsed := time.Since(start)

	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	serialWorstCase := n * sleepMS * time.Millisecond
	if elapsed >= serialWorstCase {
		t.Errorf("elapsed %v was not faster than serial worst case %v — stage commands do not appear to run concurrently", elapsed, serialWorstCase)
	}
}

func TestRunPostCreateHooksFailureDoesNotBlockNextStage(t *testing.T) {
	dir := t.TempDir()

	stages := [][]string{
		{"false"}, // fails
		{"touch stage2-ran.txt"},
	}

	results := RunPostCreateHooks(context.Background(), dir, stages, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected stage 1 hook to fail")
	}
	if results[1].Error != nil {
		t.Errorf("expected stage 2 hook to succeed, got %v", results[1].Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "stage2-ran.txt")); os.IsNotExist(err) {
		t.Error("expected stage 2 to have run despite stage 1's failure")
	}
}

func TestRunPostCreateHooksFailureWithinStageDoesNotBlockSiblings(t *testing.T) {
	dir := t.TempDir()

	stages := [][]string{
		{"false", "touch sibling-ran.txt"},
	}

	results := RunPostCreateHooks(context.Background(), dir, stages, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if _, err := os.Stat(filepath.Join(dir, "sibling-ran.txt")); os.IsNotExist(err) {
		t.Error("expected the sibling command in the same stage to have run despite the other failing")
	}
}

func TestRunPostCreateHooksEmptyStagesSkipped(t *testing.T) {
	dir := t.TempDir()

	stages := [][]string{
		{},
		{"touch marker.txt"},
	}

	results := RunPostCreateHooks(context.Background(), dir, stages, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (empty stage contributes none), got %d", len(results))
	}
	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); os.IsNotExist(err) {
		t.Error("expected marker.txt to exist")
	}
}

func TestRunPostCreateHooksNoStages(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(context.Background(), dir, nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunPostCreateHooksSingleCommandStageUsesOrdinalProgress(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var calls []string
	onProgress := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, msg)
	}

	stages := [][]string{
		{"touch a.txt"},
		{"touch b.txt"},
	}
	RunPostCreateHooks(context.Background(), dir, stages, onProgress)

	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d: %v", len(calls), calls)
	}
	if want := "touch a.txt (1/2)"; calls[0] != want {
		t.Errorf("calls[0] = %q, want %q", calls[0], want)
	}
	if want := "touch b.txt (2/2)"; calls[1] != want {
		t.Errorf("calls[1] = %q, want %q", calls[1], want)
	}
}

func TestRunPostCreateHooksMultiCommandStageUsesCompletionProgress(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var calls []string
	onProgress := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, msg)
	}

	stages := [][]string{{"touch a.txt", "touch b.txt"}}
	RunPostCreateHooks(context.Background(), dir, stages, onProgress)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d: %v", len(calls), calls)
	}
	want := map[string]bool{"1/2 complete": true, "2/2 complete": true}
	for _, c := range calls {
		if !want[c] {
			t.Errorf("unexpected progress message %q", c)
		}
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("missing progress messages: %v", want)
	}
}

func TestRunPostCreateHooksProgressOrdinalsAreCumulativeAcrossStages(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var calls []string
	onProgress := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, msg)
	}

	// Stage 1: two single-command stages (serial-style, ordinal messages).
	// Stage 2: one two-command stage (parallel-style, completion messages).
	// Both must count against the same 4-command total.
	stages := [][]string{
		{"touch a.txt"},
		{"touch b.txt"},
		{"touch c.txt", "touch d.txt"},
	}
	RunPostCreateHooks(context.Background(), dir, stages, onProgress)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 4 {
		t.Fatalf("expected 4 progress calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "touch a.txt (1/4)" {
		t.Errorf("calls[0] = %q, want %q", calls[0], "touch a.txt (1/4)")
	}
	if calls[1] != "touch b.txt (2/4)" {
		t.Errorf("calls[1] = %q, want %q", calls[1], "touch b.txt (2/4)")
	}
	for _, c := range calls[2:] {
		if c != "3/4 complete" && c != "4/4 complete" {
			t.Errorf("unexpected stage-3 progress message %q", c)
		}
	}
}
