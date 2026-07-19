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

// fileSink is the default Sink implementation: two append-only JSONL files
// per day, one for the log stream and one for the metrics stream.
type fileSink struct {
	logMu      sync.Mutex
	logFile    *os.File
	metricMu   sync.Mutex
	metricFile *os.File
}

// RepoPrefix returns the day-file prefix ("rimba-<base>-<hash>") used for
// repoRoot's observability files: the repo's base directory name plus the
// first 8 hex characters of the SHA-256 of its absolute path, disambiguating
// same-named repos in different locations. This is the single source of
// truth for the naming formula — NewFileSink and `rimba report`'s file
// discovery both derive their glob patterns from it.
func RepoPrefix(repoRoot string) string {
	base := filepath.Base(repoRoot)
	sum := sha256.Sum256([]byte(repoRoot))
	hash := hex.EncodeToString(sum[:])[:8]
	return fmt.Sprintf("rimba-%s-%s", base, hash)
}

// NewFileSink opens (creating if necessary) today's log and metrics JSONL
// files for repoRoot under the OS cache directory, and best-effort prunes
// day-files older than retentionDays (<= 0 disables pruning).
func NewFileSink(repoRoot string, retentionDays int) (Sink, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user cache dir: %w", err)
	}
	return newFileSinkAt(cacheDir, repoRoot, retentionDays)
}

// WriteLog appends record to the log stream as one JSON line.
func (f *fileSink) WriteLog(record any) error {
	return appendJSONLine(&f.logMu, f.logFile, record)
}

// WriteMetric appends record to the metrics stream as one JSON line.
func (f *fileSink) WriteMetric(record any) error {
	return appendJSONLine(&f.metricMu, f.metricFile, record)
}

// Close closes both underlying files, joining any errors from each.
func (f *fileSink) Close() error {
	return errors.Join(f.logFile.Close(), f.metricFile.Close())
}

// newFileSinkAt is NewFileSink with the cache-dir root passed explicitly, so
// tests can point it at a temp dir instead of the real user cache dir.
func newFileSinkAt(cacheDir, repoRoot string, retentionDays int) (Sink, error) {
	dir := filepath.Join(cacheDir, "rimba")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // observability cache dir holds no secrets; 0755 matches other rimba cache/log dirs
		return nil, fmt.Errorf("failed to create observability cache dir: %w", err)
	}

	prefix := RepoPrefix(repoRoot)
	today := time.Now().Format("2006-01-02")

	logFile, err := os.OpenFile(filepath.Join(dir, prefix+"-"+today+".log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // observability log lines hold no secrets; 0644 matches other rimba log files
	if err != nil {
		return nil, fmt.Errorf("failed to open observability log file: %w", err)
	}
	metricFile, err := os.OpenFile(filepath.Join(dir, prefix+"-"+today+".metrics.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // same as the log file above
	if err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("failed to open observability metrics file: %w", err)
	}

	pruneOldDayFiles(dir, prefix, retentionDays, today)

	return &fileSink{logFile: logFile, metricFile: metricFile}, nil
}

// pruneOldDayFiles best-effort deletes this repo's own day-files (matched by
// prefix) older than retentionDays. retentionDays <= 0 disables pruning
// entirely — a config typo must never wipe everything. Today's own file is
// never deleted, regardless of retentionDays, via an unconditional guard
// independent of the age arithmetic below.
func pruneOldDayFiles(dir, prefix string, retentionDays int, today string) {
	if retentionDays <= 0 {
		return
	}
	pruneGlob(dir, prefix+"-*.log.jsonl", prefix, ".log.jsonl", retentionDays, today)
	pruneGlob(dir, prefix+"-*.metrics.jsonl", prefix, ".metrics.jsonl", retentionDays, today)
}

// pruneGlob deletes files matching pattern under dir whose embedded
// YYYY-MM-DD date (between prefix+"-" and suffix) is older than
// retentionDays days before today. Unparseable dates and today's own date
// are always skipped.
func pruneGlob(dir, pattern, prefix, suffix string, retentionDays int, today string) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return
	}
	for _, match := range matches {
		name := filepath.Base(match)
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix+"-"), suffix)
		if dateStr == today {
			continue
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if time.Since(date) > time.Duration(retentionDays)*24*time.Hour {
			_ = os.Remove(match)
		}
	}
}

// appendJSONLine marshals record and appends it, newline-terminated, to f
// under mu — the single write path shared by WriteLog and WriteMetric.
func appendJSONLine(mu *sync.Mutex, f *os.File, record any) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal observability record: %w", err)
	}
	data = append(data, '\n')

	mu.Lock()
	defer mu.Unlock()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write observability record: %w", err)
	}
	return nil
}
