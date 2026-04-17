package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func ptr[T any](v T) *T { return &v }

func TestSortBySizeDesc(t *testing.T) {
	results := []statusEntry{
		{entry: git.WorktreeEntry{Branch: "small"}, sizeBytes: ptr(int64(100))},
		{entry: git.WorktreeEntry{Branch: "errored"}, sizeBytes: nil},
		{entry: git.WorktreeEntry{Branch: "big"}, sizeBytes: ptr(int64(5000))},
		{entry: git.WorktreeEntry{Branch: "medium"}, sizeBytes: ptr(int64(1000))},
		{entry: git.WorktreeEntry{Branch: "errored2"}, sizeBytes: nil},
	}
	sortBySizeDesc(results)
	wantOrder := []string{"big", "medium", "small", "errored", "errored2"}
	for i, w := range wantOrder {
		if results[i].entry.Branch != w {
			t.Errorf("index %d: got %q, want %q (full: %+v)", i, results[i].entry.Branch, w, results)
		}
	}
}

func TestSortBySizeDescAllNil(t *testing.T) {
	// Two nil-sized entries: order is preserved (stable).
	results := []statusEntry{
		{entry: git.WorktreeEntry{Branch: "a"}},
		{entry: git.WorktreeEntry{Branch: "b"}},
	}
	sortBySizeDesc(results)
	if results[0].entry.Branch != "a" || results[1].entry.Branch != "b" {
		t.Errorf("expected stable order [a, b], got [%s, %s]",
			results[0].entry.Branch, results[1].entry.Branch)
	}
}

func TestFormatDiskLine(t *testing.T) {
	p := termcolor.NewPainter(true) // no color

	fp := &operations.DiskFootprint{MainBytes: 1024 * 1024, WorktreesBytes: 2048, TotalBytes: 1024*1024 + 2048}
	got := formatDiskLine(p, fp)
	if !strings.Contains(got, "Disk:") || !strings.Contains(got, "main:") || !strings.Contains(got, "worktrees:") {
		t.Errorf("formatDiskLine = %q, missing required fragments", got)
	}

	// Error path: main: fragment must be omitted.
	fpErr := &operations.DiskFootprint{MainErr: errors.New("boom"), WorktreesBytes: 512, TotalBytes: 512}
	got = formatDiskLine(p, fpErr)
	if strings.Contains(got, "main:") {
		t.Errorf("formatDiskLine on mainErr = %q, expected no 'main:' fragment", got)
	}
	if !strings.Contains(got, "worktrees:") {
		t.Errorf("formatDiskLine on mainErr = %q, expected 'worktrees:' fragment", got)
	}
}

func TestFormatSizeCell(t *testing.T) {
	p := termcolor.NewPainter(true)

	if got := formatSizeCell(statusEntry{sizeBytes: nil}, p); got != "?" {
		t.Errorf("nil sizeBytes cell = %q, want '?'", got)
	}
	if got := formatSizeCell(statusEntry{sizeBytes: ptr(int64(2048))}, p); got != "2.0KB" {
		t.Errorf("2048-byte cell = %q, want '2.0KB'", got)
	}
}

func TestFormatRecentCell(t *testing.T) {
	p := termcolor.NewPainter(true)

	if got := formatRecentCell(statusEntry{recent7D: nil}, p); got != "?" {
		t.Errorf("nil recent7D cell = %q, want '?'", got)
	}
	if got := formatRecentCell(statusEntry{recent7D: ptr(42)}, p); got != "42" {
		t.Errorf("recent7D=42 cell = %q, want '42'", got)
	}
}

// detailFixture spins up a mocked Runner backed by real temp dirs suitable
// for exercising the full --detail code path end-to-end.
type detailFixture struct {
	mainPath, wtBigPath, wtSmallPath string
	restore                          func()
}

func setupDetailFixture(t *testing.T) detailFixture {
	t.Helper()
	f := detailFixture{
		mainPath:    t.TempDir(),
		wtBigPath:   t.TempDir(),
		wtSmallPath: t.TempDir(),
	}
	// Inflate big worktree so the sort is deterministic.
	if err := os.WriteFile(filepath.Join(f.wtBigPath, "blob"), make([]byte, 8*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.wtSmallPath, "tiny"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	worktreeList := fmt.Sprintf(
		"worktree %s\nHEAD abc\nbranch refs/heads/main\n\n"+
			"worktree %s\nHEAD def\nbranch refs/heads/feature/big\n\n"+
			"worktree %s\nHEAD ghi\nbranch refs/heads/feature/small\n\n",
		f.mainPath, f.wtBigPath, f.wtSmallPath,
	)

	f.restore = overrideNewRunner(&mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return f.mainPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return worktreeList, nil
			case args[0] == cmdLog:
				return "1700000000\tcommit msg", nil
			case args[0] == cmdRevList && len(args) >= 2 && args[1] == "--count":
				if strings.Contains(args[len(args)-1], "big") {
					return "9", nil
				}
				return "3", nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if args[0] == cmdStatus {
				return "", nil
			}
			if args[0] == cmdRevList {
				return aheadBehindZero, nil
			}
			return "", nil
		},
	})
	return f
}

func newDetailCmd(t *testing.T, jsonOut bool) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd, buf := newTestCmd()
	cmd.Flags().Int(flagStaleDays, 14, "")
	cmd.Flags().Bool(flagDetail, true, "")
	_ = cmd.Flags().Set(flagDetail, "true")
	if jsonOut {
		_ = cmd.Flags().Set(flagJSON, "true")
	}
	return cmd, buf
}

// TestStatusDetailTable exercises --detail table output: new columns, the
// Disk: summary line, and size-desc sort order.
func TestStatusDetailTable(t *testing.T) {
	f := setupDetailFixture(t)
	defer f.restore()

	cmd, buf := newDetailCmd(t, false)
	if err := statusCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("statusCmd.RunE --detail: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"SIZE", "7D", "Disk:", "main:", "worktrees:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in --detail output, got:\n%s", want, out)
		}
	}
	bigIdx := strings.Index(out, "feature/big")
	smallIdx := strings.Index(out, "feature/small")
	if bigIdx < 0 || smallIdx < 0 || bigIdx >= smallIdx {
		t.Errorf("expected feature/big before feature/small; big=%d small=%d\n%s",
			bigIdx, smallIdx, out)
	}
}

// TestStatusDetailJSON exercises --detail --json: new per-item fields and
// the top-level disk object are present.
func TestStatusDetailJSON(t *testing.T) {
	f := setupDetailFixture(t)
	defer f.restore()

	cmd, buf := newDetailCmd(t, true)
	if err := statusCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("statusCmd.RunE --detail --json: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	assertDetailJSONShape(t, data)
}

func assertDetailJSONShape(t *testing.T, data map[string]any) {
	t.Helper()

	disk, ok := data["disk"].(map[string]any)
	if !ok {
		t.Fatalf("missing disk key in JSON; full data: %+v", data)
	}
	for _, key := range []string{"total_bytes", "main_bytes", "worktrees_bytes"} {
		if _, present := disk[key]; !present {
			t.Errorf("disk.%s missing from JSON: %+v", key, disk)
		}
	}

	worktrees, _ := data["worktrees"].([]any)
	if len(worktrees) != 2 {
		t.Fatalf("worktrees length = %d, want 2", len(worktrees))
	}
	first, _ := worktrees[0].(map[string]any)
	if _, ok := first["size_bytes"]; !ok {
		t.Errorf("worktrees[0] missing size_bytes: %+v", first)
	}
	if _, ok := first["recent_7d"]; !ok {
		t.Errorf("worktrees[0] missing recent_7d: %+v", first)
	}
}
