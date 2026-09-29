package observability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const rotationRepo = "/repo/rotation"

// fakeClock is a settable clock for driving day rotation deterministically.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{t: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func rotationDayFile(cacheDir, day, suffix string) string {
	return filepath.Join(cacheDir, "rimba", RepoPrefix(rotationRepo)+"-"+day+suffix)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return string(data)
}

func TestFileSinkRotatesAcrossMidnight(t *testing.T) {
	cacheDir := t.TempDir()
	clock := newFakeClock(time.Date(2026, 3, 1, 23, 59, 0, 0, time.UTC))
	sink, err := newFileSinkAt(cacheDir, rotationRepo, 14, clock.Now)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	defer func() { _ = sink.Close() }()

	if err := sink.WriteLog(map[string]string{"n": "day1-log"}); err != nil {
		t.Fatalf("WriteLog day1: %v", err)
	}
	if err := sink.WriteMetric(map[string]string{"n": "day1-metric"}); err != nil {
		t.Fatalf("WriteMetric day1: %v", err)
	}

	clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
	if err := sink.WriteLog(map[string]string{"n": "day2-log"}); err != nil {
		t.Fatalf("WriteLog day2: %v", err)
	}
	if err := sink.WriteMetric(map[string]string{"n": "day2-metric"}); err != nil {
		t.Fatalf("WriteMetric day2: %v", err)
	}

	tests := []struct {
		day, suffix, want, notWant string
	}{
		{"2026-03-01", ".log.jsonl", "day1-log", "day2-log"},
		{"2026-03-02", ".log.jsonl", "day2-log", "day1-log"},
		{"2026-03-01", ".metrics.jsonl", "day1-metric", "day2-metric"},
		{"2026-03-02", ".metrics.jsonl", "day2-metric", "day1-metric"},
	}
	for _, tt := range tests {
		got := readFileString(t, rotationDayFile(cacheDir, tt.day, tt.suffix))
		if !strings.Contains(got, tt.want) || strings.Contains(got, tt.notWant) {
			t.Errorf("%s%s = %q, want only %q", tt.day, tt.suffix, got, tt.want)
		}
	}
}

// Blocking each stream's next-day path in turn covers both the log-open and the
// log-opened-then-metrics-failed rotation failures.
func TestFileSinkRotationFailureKeepsOldFilesThenRetries(t *testing.T) {
	for _, blockedSuffix := range []string{".log.jsonl", ".metrics.jsonl"} {
		t.Run(blockedSuffix, func(t *testing.T) {
			cacheDir := t.TempDir()
			clock := newFakeClock(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
			sink, err := newFileSinkAt(cacheDir, rotationRepo, 14, clock.Now)
			if err != nil {
				t.Fatalf("newFileSinkAt: %v", err)
			}
			defer func() { _ = sink.Close() }()

			blocker := rotationDayFile(cacheDir, "2026-03-02", blockedSuffix)
			if err := os.Mkdir(blocker, 0o755); err != nil {
				t.Fatalf("Mkdir blocker: %v", err)
			}

			clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
			if err := sink.WriteLog(map[string]string{"n": "log-during-failure"}); err != nil {
				t.Fatalf("WriteLog during failed rotation: %v", err)
			}
			if err := sink.WriteMetric(map[string]string{"n": "metric-during-failure"}); err != nil {
				t.Fatalf("WriteMetric during failed rotation: %v", err)
			}
			if got := readFileString(t, rotationDayFile(cacheDir, "2026-03-01", ".log.jsonl")); !strings.Contains(got, "log-during-failure") {
				t.Errorf("log record should land in the previous day's file, got %q", got)
			}
			if got := readFileString(t, rotationDayFile(cacheDir, "2026-03-01", ".metrics.jsonl")); !strings.Contains(got, "metric-during-failure") {
				t.Errorf("metric record should land in the previous day's file, got %q", got)
			}

			if err := os.Remove(blocker); err != nil {
				t.Fatalf("Remove blocker: %v", err)
			}
			if err := sink.WriteLog(map[string]string{"n": "log-after-unblock"}); err != nil {
				t.Fatalf("WriteLog after unblock: %v", err)
			}
			if err := sink.WriteMetric(map[string]string{"n": "metric-after-unblock"}); err != nil {
				t.Fatalf("WriteMetric after unblock: %v", err)
			}
			if got := readFileString(t, rotationDayFile(cacheDir, "2026-03-02", ".log.jsonl")); !strings.Contains(got, "log-after-unblock") {
				t.Errorf("next write should rotate to the new day's log file, got %q", got)
			}
			if got := readFileString(t, rotationDayFile(cacheDir, "2026-03-02", ".metrics.jsonl")); !strings.Contains(got, "metric-after-unblock") {
				t.Errorf("next write should rotate to the new day's metrics file, got %q", got)
			}
		})
	}
}

func TestFileSinkRotationRecreatesRemovedCacheDir(t *testing.T) {
	cacheDir := t.TempDir()
	clock := newFakeClock(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	sink, err := newFileSinkAt(cacheDir, rotationRepo, 14, clock.Now)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	defer func() { _ = sink.Close() }()

	if err := os.RemoveAll(filepath.Join(cacheDir, "rimba")); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
	if err := sink.WriteLog(map[string]string{"n": "healed"}); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	if got := readFileString(t, rotationDayFile(cacheDir, "2026-03-02", ".log.jsonl")); !strings.Contains(got, "healed") {
		t.Errorf("rotation should recreate the cache dir and write there, got %q", got)
	}
}

func TestFileSinkRotationPrunesExpiredDayFiles(t *testing.T) {
	cacheDir := t.TempDir()
	clock := newFakeClock(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	sink, err := newFileSinkAt(cacheDir, rotationRepo, 3, clock.Now)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	defer func() { _ = sink.Close() }()

	// Created after open, so only the rotation-time prune can remove it.
	stale := rotationDayFile(cacheDir, "2026-02-25", ".log.jsonl")
	if err := os.WriteFile(stale, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile stale: %v", err)
	}

	clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
	if err := sink.WriteLog(map[string]string{"n": "x"}); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("rotation should have pruned the file older than retention")
	}
}

func TestFileSinkWriteAfterCloseReturnsErrClosed(t *testing.T) {
	cacheDir := t.TempDir()
	clock := newFakeClock(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	sink, err := newFileSinkAt(cacheDir, rotationRepo, 14, clock.Now)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Errorf("second Close should be a no-op, got %v", err)
	}

	// A later day would rotate if the closed sink were still live.
	clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
	if err := sink.WriteLog(map[string]string{"n": "x"}); !errors.Is(err, os.ErrClosed) {
		t.Errorf("WriteLog after Close = %v, want os.ErrClosed", err)
	}
	if err := sink.WriteMetric(map[string]string{"n": "x"}); !errors.Is(err, os.ErrClosed) {
		t.Errorf("WriteMetric after Close = %v, want os.ErrClosed", err)
	}
	if _, err := os.Stat(rotationDayFile(cacheDir, "2026-03-02", ".log.jsonl")); err == nil {
		t.Error("closed sink must not create new day files")
	}
}

func TestFileSinkConcurrentWritesAcrossRotation(t *testing.T) {
	cacheDir := t.TempDir()
	clock := newFakeClock(time.Date(2026, 3, 1, 23, 59, 0, 0, time.UTC))
	sink, err := newFileSinkAt(cacheDir, rotationRepo, 14, clock.Now)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}

	const writers, perWriter = 8, 50
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for i := range perWriter {
				if w == 0 && i == perWriter/2 {
					clock.Set(time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC))
				}
				if err := sink.WriteLog(map[string]int{"w": w, "i": i}); err != nil {
					t.Errorf("WriteLog: %v", err)
				}
				if err := sink.WriteMetric(map[string]int{"w": w, "i": i}); err != nil {
					t.Errorf("WriteMetric: %v", err)
				}
			}
		})
	}
	wg.Wait()
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, suffix := range []string{".log.jsonl", ".metrics.jsonl"} {
		lines := 0
		for _, day := range []string{"2026-03-01", "2026-03-02"} {
			lines += strings.Count(readFileString(t, rotationDayFile(cacheDir, day, suffix)), "\n")
		}
		if want := writers * perWriter; lines != want {
			t.Errorf("%s total lines = %d, want %d", suffix, lines, want)
		}
	}
}
