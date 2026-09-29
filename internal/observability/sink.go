package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const dayLayout = "2006-01-02"

// Sink is the narrow port domain code never sees directly — only Recorder
// depends on it. Swapping the file adapter for something else (SQLite, etc.)
// touches only this file and NewFileSink's caller.
type Sink interface {
	// WriteLog appends one JSONL record to the log stream.
	WriteLog(record any) error
	// WriteMetric appends one JSONL record to the metrics stream.
	WriteMetric(record any) error
	// Close releases the underlying file handles.
	Close() error
}

// fileSink is the default Sink: per-day append-only JSONL log and metrics files,
// rotated lazily on the first write after midnight so a long-lived owner can share it.
type fileSink struct {
	mu            sync.Mutex
	dir, prefix   string
	retentionDays int
	now           func() time.Time
	day           string
	logFile       *os.File // nil after Close
	metricFile    *os.File
}

// RepoPrefix returns repoRoot's day-file prefix. It's the single source of
// truth for the naming formula — NewFileSink and `rimba report`'s file
// discovery both derive their ListDayFiles prefix from it.
func RepoPrefix(repoRoot string) string {
	base := filepath.Base(repoRoot)
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	return fmt.Sprintf("rimba-%s-%s", base, hash)
}

// NewFileSink opens (creating if necessary) today's log and metrics JSONL
// files for repoRoot under the OS cache directory, and best-effort prunes
// day-files older than retentionDays (<= 0 disables). It rotates itself across midnight.
func NewFileSink(repoRoot string, retentionDays int) (Sink, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user cache dir: %w", err)
	}
	return newFileSinkAt(cacheDir, repoRoot, retentionDays, time.Now)
}

// ListDayFiles returns files under dir starting with prefix+"-" and ending
// with suffix, matched literally rather than via filepath.Glob, so a repo
// directory name containing a glob metacharacter ("[", "*", "?") still
// matches its own day-files correctly.
func ListDayFiles(dir, prefix, suffix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	want := prefix + "-"
	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, want) && strings.HasSuffix(name, suffix) {
			matches = append(matches, filepath.Join(dir, name))
		}
	}
	return matches
}

// WriteLog appends record to the log stream as one JSON line.
func (f *fileSink) WriteLog(record any) error {
	return f.appendLine(record, func() *os.File { return f.logFile })
}

// WriteMetric appends record to the metrics stream as one JSON line.
func (f *fileSink) WriteMetric(record any) error {
	return f.appendLine(record, func() *os.File { return f.metricFile })
}

// Close closes both underlying files, joining any errors from each. Later
// writes return os.ErrClosed and never reopen files.
func (f *fileSink) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logFile == nil {
		return nil
	}
	err := errors.Join(f.logFile.Close(), f.metricFile.Close())
	f.logFile, f.metricFile = nil, nil
	return err
}

// newFileSinkAt is NewFileSink with the cache-dir root and clock passed
// explicitly, so tests can use a temp dir and a fake clock.
func newFileSinkAt(cacheDir, repoRoot string, retentionDays int, now func() time.Time) (Sink, error) {
	dir := filepath.Join(cacheDir, "rimba")
	prefix := RepoPrefix(repoRoot)
	t := now()
	day := t.Format(dayLayout)

	logFile, metricFile, err := openDayFiles(dir, prefix, day)
	if err != nil {
		return nil, err
	}

	pruneOldDayFiles(dir, prefix, retentionDays, t)

	return &fileSink{
		dir: dir, prefix: prefix, retentionDays: retentionDays, now: now,
		day: day, logFile: logFile, metricFile: metricFile,
	}, nil
}

// pruneOldDayFiles best-effort deletes this repo's own day-files (matched by
// prefix) older than retentionDays relative to now. retentionDays <= 0
// disables pruning entirely — a config typo must never wipe everything.
// Today's own file is never deleted, regardless of retentionDays, via an
// unconditional guard independent of the age arithmetic below.
func pruneOldDayFiles(dir, prefix string, retentionDays int, now time.Time) {
	if retentionDays <= 0 {
		return
	}
	pruneSuffix(dir, prefix, ".log.jsonl", retentionDays, now)
	pruneSuffix(dir, prefix, ".metrics.jsonl", retentionDays, now)
}

// pruneSuffix deletes files under dir whose name literally starts with
// prefix+"-" and ends with suffix, and whose embedded YYYY-MM-DD date is
// older than retentionDays days before now. Unparseable dates and today's
// own date are always skipped.
func pruneSuffix(dir, prefix, suffix string, retentionDays int, now time.Time) {
	want := prefix + "-"
	today := now.Format(dayLayout)
	for _, match := range ListDayFiles(dir, prefix, suffix) {
		name := filepath.Base(match)
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, want), suffix)
		if dateStr == today {
			continue
		}
		date, err := time.Parse(dayLayout, dateStr)
		if err != nil {
			continue
		}
		if now.Sub(date) > time.Duration(retentionDays)*24*time.Hour {
			_ = os.Remove(match)
		}
	}
}

// openDayFiles opens (creating dir and files if necessary) the log and metrics
// files for day, both or neither. Recreating dir lets rotation self-heal.
func openDayFiles(dir, prefix, day string) (logFile, metricFile *os.File, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // observability cache dir holds no secrets; 0755 matches other rimba cache/log dirs
		return nil, nil, fmt.Errorf("failed to create observability cache dir: %w", err)
	}
	logFile, err = os.OpenFile(filepath.Join(dir, prefix+"-"+day+".log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // 0600 avoids the G302 permissive-mode warning; log lines can carry command args/stderr that may include secrets, so this is NOT "safe to expose", just owner-only
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open observability log file: %w", err)
	}
	metricFile, err = os.OpenFile(filepath.Join(dir, prefix+"-"+day+".metrics.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // same as the log file above
	if err != nil {
		_ = logFile.Close()
		return nil, nil, fmt.Errorf("failed to open observability metrics file: %w", err)
	}
	return logFile, metricFile, nil
}

// appendLine marshals record and appends it as one line to the file chosen by
// pick, which runs under f.mu after any day rotation.
func (f *fileSink) appendLine(record any, pick func() *os.File) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal observability record: %w", err)
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logFile == nil {
		return os.ErrClosed
	}
	f.rotateLocked()
	if _, err := pick().Write(data); err != nil {
		return fmt.Errorf("failed to write observability record: %w", err)
	}
	return nil
}

// rotateLocked switches to today's files when the day changed (caller holds f.mu).
// On failure the old handles stay in use and a later write retries.
func (f *fileSink) rotateLocked() {
	t := f.now()
	day := t.Format(dayLayout)
	if day == f.day {
		return
	}
	logFile, metricFile, err := openDayFiles(f.dir, f.prefix, day)
	if err != nil {
		return
	}
	_ = f.logFile.Close()
	_ = f.metricFile.Close()
	f.logFile, f.metricFile, f.day = logFile, metricFile, day
	pruneOldDayFiles(f.dir, f.prefix, f.retentionDays, t)
}
