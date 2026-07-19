package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewFileSinkSmoke(t *testing.T) {
	repoRoot := t.TempDir()
	sink, err := NewFileSink(repoRoot, 14)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	t.Cleanup(func() {
		_ = sink.Close()
		cacheDir, cerr := os.UserCacheDir()
		if cerr != nil {
			return
		}
		base := filepath.Base(repoRoot)
		sum := sha256.Sum256([]byte(repoRoot))
		hash := hex.EncodeToString(sum[:])[:8]
		prefix := fmt.Sprintf("rimba-%s-%s", base, hash)
		today := time.Now().Format("2006-01-02")
		_ = os.Remove(filepath.Join(cacheDir, "rimba", prefix+"-"+today+".log.jsonl"))
		_ = os.Remove(filepath.Join(cacheDir, "rimba", prefix+"-"+today+".metrics.jsonl"))
	})
}

func TestFileSinkWriteReadRoundTrip(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := "/repo/myproject"
	sink, err := newFileSinkAt(cacheDir, repoRoot, 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}

	logRec := CommandRecord{SchemaVersion: SchemaVersion, Kind: "command", RunID: "r1", Command: "add"}
	metricRec := SpanRecord{SchemaVersion: SchemaVersion, Kind: "span", RunID: "r1", SpanID: "s1", Name: "command"}

	if err := sink.WriteLog(logRec); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	if err := sink.WriteMetric(metricRec); err != nil {
		t.Fatalf("WriteMetric: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	base := filepath.Base(repoRoot)
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", base, hash)
	today := time.Now().Format("2006-01-02")
	dir := filepath.Join(cacheDir, "rimba")

	logData, err := os.ReadFile(filepath.Join(dir, prefix+"-"+today+".log.jsonl"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logLine := strings.TrimSuffix(string(logData), "\n")
	if strings.Count(string(logData), "\n") != 1 {
		t.Fatalf("expected exactly one newline-terminated line, got %q", string(logData))
	}
	var gotLog CommandRecord
	if err := json.Unmarshal([]byte(logLine), &gotLog); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	if gotLog != logRec {
		t.Errorf("gotLog = %+v, want %+v", gotLog, logRec)
	}

	metricData, err := os.ReadFile(filepath.Join(dir, prefix+"-"+today+".metrics.jsonl"))
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}
	metricLine := strings.TrimSuffix(string(metricData), "\n")
	var gotMetric SpanRecord
	if err := json.Unmarshal([]byte(metricLine), &gotMetric); err != nil {
		t.Fatalf("unmarshal metric line: %v", err)
	}
	if gotMetric != metricRec {
		t.Errorf("gotMetric = %+v, want %+v", gotMetric, metricRec)
	}
}

// TestFileSinkFilePermissions confirms both day-files are created 0600 (owner
// read/write only) — these files can carry secrets via subprocess Args or
// failure Stderr/Error text, so they must not be group/other readable.
func TestFileSinkFilePermissions(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := "/repo/permcheck"
	sink, err := newFileSinkAt(cacheDir, repoRoot, 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	base := filepath.Base(repoRoot)
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", base, hash)
	today := time.Now().Format("2006-01-02")
	dir := filepath.Join(cacheDir, "rimba")

	for _, name := range []string{prefix + "-" + today + ".log.jsonl", prefix + "-" + today + ".metrics.jsonl"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s: perm = %o, want 0600", name, perm)
		}
	}
}

func TestFileSinkConcurrentWrites(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := "/repo/concurrent"
	sink, err := newFileSinkAt(cacheDir, repoRoot, 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}

	const goroutines = 2
	const writesEach = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range writesEach {
				rec := SubprocessRecord{SchemaVersion: SchemaVersion, Kind: "subprocess", RunID: fmt.Sprintf("g%d", id), Seq: uint64(i)}
				if err := sink.WriteLog(rec); err != nil {
					t.Errorf("WriteLog: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	base := filepath.Base(repoRoot)
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", base, hash)
	today := time.Now().Format("2006-01-02")
	dir := filepath.Join(cacheDir, "rimba")

	data, err := os.ReadFile(filepath.Join(dir, prefix+"-"+today+".log.jsonl"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != goroutines*writesEach {
		t.Fatalf("got %d lines, want %d", len(lines), goroutines*writesEach)
	}
	for _, line := range lines {
		var rec SubprocessRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line failed to parse as valid JSON: %q: %v", line, err)
		}
	}
}

func TestFileSinkNaming(t *testing.T) {
	cacheDir := t.TempDir()
	repoRoot := "/some/path/to/myrepo"
	sink, err := newFileSinkAt(cacheDir, repoRoot, 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	defer func() { _ = sink.Close() }()

	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	today := time.Now().Format("2006-01-02")
	wantLog := fmt.Sprintf("rimba-myrepo-%s-%s.log.jsonl", hash, today)
	wantMetric := fmt.Sprintf("rimba-myrepo-%s-%s.metrics.jsonl", hash, today)

	dir := filepath.Join(cacheDir, "rimba")
	if _, err := os.Stat(filepath.Join(dir, wantLog)); err != nil {
		t.Errorf("expected log file %q to exist: %v", wantLog, err)
	}
	if _, err := os.Stat(filepath.Join(dir, wantMetric)); err != nil {
		t.Errorf("expected metrics file %q to exist: %v", wantMetric, err)
	}
}

func TestPruneOldDayFiles(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "rimba")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	repoRoot := "/repo/pruning"
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", filepath.Base(repoRoot), hash)
	otherPrefix := "rimba-other-deadbeef"
	today := time.Now().Format("2006-01-02")

	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	oldDate := time.Now().AddDate(0, 0, -20).Format("2006-01-02")
	recentDate := time.Now().AddDate(0, 0, -5).Format("2006-01-02")

	oldLog := prefix + "-" + oldDate + ".log.jsonl"
	oldMetric := prefix + "-" + oldDate + ".metrics.jsonl"
	recentLog := prefix + "-" + recentDate + ".log.jsonl"
	todayLog := prefix + "-" + today + ".log.jsonl"
	badDateLog := prefix + "-garbage.log.jsonl"
	otherOldLog := otherPrefix + "-" + oldDate + ".log.jsonl"

	for _, name := range []string{oldLog, oldMetric, recentLog, todayLog, badDateLog, otherOldLog} {
		touch(name)
	}

	sink, err := newFileSinkAt(cacheDir, repoRoot, 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	_ = sink.Close()

	assertExists := func(name string, want bool) {
		t.Helper()
		_, err := os.Stat(filepath.Join(dir, name))
		exists := err == nil
		if exists != want {
			t.Errorf("file %q exists=%v, want %v", name, exists, want)
		}
	}

	assertExists(oldLog, false)
	assertExists(oldMetric, false)
	assertExists(recentLog, true)
	assertExists(todayLog, true)
	assertExists(badDateLog, true)
	assertExists(otherOldLog, true)
}

func TestNewFileSinkUserCacheDirError(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	if _, err := NewFileSink("/some/repo", 14); err == nil {
		t.Fatal("expected error when the user cache dir cannot be resolved")
	}
}

func TestNewFileSinkAtMkdirAllFails(t *testing.T) {
	cacheDir := t.TempDir()
	// Put a plain file where the "rimba" subdirectory needs to go, so MkdirAll fails.
	if err := os.WriteFile(filepath.Join(cacheDir, "rimba"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := newFileSinkAt(cacheDir, "/repo/mkdirfail", 14); err == nil {
		t.Fatal("expected error when the rimba cache path collides with a file")
	}
}

func TestNewFileSinkAtLogFileOpenFails(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "rimba")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	repoRoot := "/repo/logopenfail"
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", filepath.Base(repoRoot), hash)
	today := time.Now().Format("2006-01-02")
	// A directory at the log file's path makes os.OpenFile fail.
	if err := os.MkdirAll(filepath.Join(dir, prefix+"-"+today+".log.jsonl"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if _, err := newFileSinkAt(cacheDir, repoRoot, 14); err == nil {
		t.Fatal("expected error opening the log file where a directory exists")
	}
}

func TestNewFileSinkAtMetricFileOpenFails(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "rimba")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	repoRoot := "/repo/metricopenfail"
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	prefix := fmt.Sprintf("rimba-%s-%s", filepath.Base(repoRoot), hash)
	today := time.Now().Format("2006-01-02")
	// A directory at the metrics file's path makes the second os.OpenFile fail,
	// after the log file has already been opened successfully.
	if err := os.MkdirAll(filepath.Join(dir, prefix+"-"+today+".metrics.jsonl"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if _, err := newFileSinkAt(cacheDir, repoRoot, 14); err == nil {
		t.Fatal("expected error opening the metrics file where a directory exists")
	}
}

func TestAppendJSONLineMarshalError(t *testing.T) {
	cacheDir := t.TempDir()
	sink, err := newFileSinkAt(cacheDir, "/repo/marshalerr", 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	defer func() { _ = sink.Close() }()

	if err := sink.WriteLog(make(chan int)); err == nil {
		t.Error("expected a marshal error for an unsupported type")
	}
}

func TestAppendJSONLineWriteError(t *testing.T) {
	cacheDir := t.TempDir()
	sinkIface, err := newFileSinkAt(cacheDir, "/repo/writeerr", 14)
	if err != nil {
		t.Fatalf("newFileSinkAt: %v", err)
	}
	fs, ok := sinkIface.(*fileSink)
	if !ok {
		t.Fatalf("sinkIface = %T, want *fileSink", sinkIface)
	}
	if err := fs.logFile.Close(); err != nil {
		t.Fatalf("closing underlying log file early: %v", err)
	}

	if err := fs.WriteLog(CommandRecord{}); err == nil {
		t.Error("expected a write error after closing the underlying file")
	}
}

// TestListDayFilesMatchesGlobMetacharacterPrefix is the regression test for
// a real gap: a repo directory name containing a glob metacharacter (here,
// unmatched brackets) would silently fail to match its own files under
// filepath.Glob. ListDayFiles matches by literal string comparison instead,
// so it must find the file regardless.
func TestListDayFilesMatchesGlobMetacharacterPrefix(t *testing.T) {
	dir := t.TempDir()
	prefix := "rimba-my[repo]-deadbeef"
	today := time.Now().Format("2006-01-02")
	name := prefix + "-" + today + ".log.jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	matches := ListDayFiles(dir, prefix, ".log.jsonl")
	if len(matches) != 1 || filepath.Base(matches[0]) != name {
		t.Errorf("ListDayFiles(%q) = %v, want exactly [%q]", prefix, matches, name)
	}
}

// TestPruneOldDayFilesPrunesGlobMetacharacterPrefix confirms pruning itself
// (not just listing) still targets the correct files when the prefix
// contains glob metacharacters.
func TestPruneOldDayFilesPrunesGlobMetacharacterPrefix(t *testing.T) {
	dir := t.TempDir()
	prefix := "rimba-my[repo]-deadbeef"
	today := time.Now().Format("2006-01-02")
	oldDate := time.Now().AddDate(0, 0, -20).Format("2006-01-02")
	oldLog := prefix + "-" + oldDate + ".log.jsonl"
	todayLog := prefix + "-" + today + ".log.jsonl"
	for _, name := range []string{oldLog, todayLog} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	pruneOldDayFiles(dir, prefix, 14, today)

	if _, err := os.Stat(filepath.Join(dir, oldLog)); err == nil {
		t.Error("old day-file with a glob-metacharacter prefix was not pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, todayLog)); err != nil {
		t.Error("today's file with a glob-metacharacter prefix should survive")
	}
}

// TestListDayFilesMissingDirReturnsNil confirms the ReadDir-error path
// degrades gracefully (best-effort), matching the old filepath.Glob
// error-handling contract it replaced.
func TestListDayFilesMissingDirReturnsNil(t *testing.T) {
	matches := ListDayFiles(filepath.Join(t.TempDir(), "does-not-exist"), "rimba-x-deadbeef", ".log.jsonl")
	if matches != nil {
		t.Errorf("ListDayFiles on a missing dir = %v, want nil", matches)
	}
}

func TestPruneOldDayFilesRetentionDisabled(t *testing.T) {
	for _, retention := range []int{0, -1} {
		t.Run(fmt.Sprintf("retention=%d", retention), func(t *testing.T) {
			cacheDir := t.TempDir()
			dir := filepath.Join(cacheDir, "rimba")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}

			repoRoot := "/repo/disabled"
			sum := sha256.Sum256([]byte(repoRoot))
			hash := hex.EncodeToString(sum[:])[:8]
			prefix := fmt.Sprintf("rimba-%s-%s", filepath.Base(repoRoot), hash)
			oldDate := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
			oldLog := prefix + "-" + oldDate + ".log.jsonl"

			if err := os.WriteFile(filepath.Join(dir, oldLog), []byte("{}\n"), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			sink, err := newFileSinkAt(cacheDir, repoRoot, retention)
			if err != nil {
				t.Fatalf("newFileSinkAt: %v", err)
			}
			_ = sink.Close()

			if _, err := os.Stat(filepath.Join(dir, oldLog)); err != nil {
				t.Errorf("expected old file to survive when retention=%d, but it's gone: %v", retention, err)
			}
		})
	}
}
