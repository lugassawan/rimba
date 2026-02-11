package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/resolver"
)

func TestRenderWorktreeListEmpty(t *testing.T) {
	m := model{}
	out := m.renderWorktreeList()
	if !strings.Contains(out, "Rimba Worktrees") {
		t.Error("expected title in output")
	}
	if !strings.Contains(out, "No worktrees found") {
		t.Error("expected 'No worktrees found' message")
	}
}

func TestRenderWorktreeListWithItems(t *testing.T) {
	m := model{
		worktrees: []worktreeItem{
			{Task: "auth", Type: "feature", Branch: "feature/auth", Path: "/wt/auth", Status: resolver.WorktreeStatus{}},
			{Task: "fix-leak", Type: "bugfix", Branch: "bugfix/fix-leak", Path: "/wt/fix-leak", IsCurrent: true, Status: resolver.WorktreeStatus{Dirty: true}},
		},
		cursor: 0,
	}
	out := m.renderWorktreeList()

	if !strings.Contains(out, "auth") {
		t.Error("expected 'auth' task in output")
	}
	if !strings.Contains(out, "fix-leak") {
		t.Error("expected 'fix-leak' task in output")
	}
}

func TestRenderWorktreeListCursor(t *testing.T) {
	m := model{
		worktrees: []worktreeItem{
			{Task: "first", Type: "feature", Branch: "feature/first"},
			{Task: "second", Type: "bugfix", Branch: "bugfix/second"},
		},
		cursor: 1,
	}
	out := m.renderWorktreeList()

	// The output should contain both items.
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Error("expected both items in output")
	}
}

func TestRenderHelp(t *testing.T) {
	m := model{}
	out := m.renderHelp()

	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Error("expected 'Keyboard Shortcuts' title")
	}
	// Verify some shortcuts are listed.
	for _, key := range []string{"Navigate", "Add", "Remove", "Merge", "Sync", "Quit"} {
		if !strings.Contains(out, key) {
			t.Errorf("expected %q in help output", key)
		}
	}
}

func TestRenderConflictViewNilAnalysis(t *testing.T) {
	m := model{conflictAnalysis: nil}
	out := m.renderConflictView()

	if !strings.Contains(out, "Conflict Analysis") {
		t.Error("expected 'Conflict Analysis' title")
	}
	if !strings.Contains(out, "Running analysis") {
		t.Error("expected 'Running analysis...' message")
	}
}

func TestRenderConflictViewNoOverlaps(t *testing.T) {
	m := model{conflictAnalysis: &conflict.Analysis{}}
	out := m.renderConflictView()

	if !strings.Contains(out, "No file overlaps detected") {
		t.Error("expected 'No file overlaps detected' message")
	}
}

func TestRenderConflictViewWithOverlaps(t *testing.T) {
	m := model{
		conflictAnalysis: &conflict.Analysis{
			Overlaps: []conflict.FileOverlap{
				{File: testSharedFile, Branches: []string{testBranchA, testBranchB}},
			},
			Pairs: []conflict.PairConflict{
				{BranchA: testBranchA, BranchB: testBranchB, OverlapFiles: []string{testSharedFile}, HasConflict: true},
			},
		},
	}
	out := m.renderConflictView()

	if !strings.Contains(out, testSharedFile) {
		t.Error("expected 'shared.go' in output")
	}
	if !strings.Contains(out, testBranchA) {
		t.Error("expected 'feature/a' in output")
	}
	if !strings.Contains(out, "CONFLICT") {
		t.Error("expected 'CONFLICT' label in output")
	}
}

func TestRenderConflictViewWithPairsNoConflict(t *testing.T) {
	m := model{
		conflictAnalysis: &conflict.Analysis{
			Overlaps: []conflict.FileOverlap{
				{File: "common.go", Branches: []string{testBranchA, testBranchB}},
			},
			Pairs: []conflict.PairConflict{
				{BranchA: testBranchA, BranchB: testBranchB, OverlapFiles: []string{"common.go"}, HasConflict: false},
			},
		},
	}
	out := m.renderConflictView()

	if strings.Contains(out, "CONFLICT") {
		t.Error("should not contain 'CONFLICT' label when HasConflict is false")
	}
}

func TestRenderExecViewEmpty(t *testing.T) {
	m := model{execOutput: ""}
	out := m.renderExecView()

	if !strings.Contains(out, "Exec Results") {
		t.Error("expected 'Exec Results' title")
	}
	if !strings.Contains(out, "No exec output yet") {
		t.Error("expected empty state message")
	}
}

func TestRenderExecViewWithOutput(t *testing.T) {
	m := model{execOutput: "test output line 1\ntest output line 2"}
	out := m.renderExecView()

	if !strings.Contains(out, "test output line 1") {
		t.Error("expected exec output in view")
	}
}

func TestViewSwitching(t *testing.T) {
	m := model{
		worktrees:        []worktreeItem{{Task: testTaskA, Type: "feature", Branch: testBranchTaskA}},
		conflictAnalysis: &conflict.Analysis{},
		execOutput:       "exec result",
	}

	// Default is list view.
	out := m.View()
	if !strings.Contains(out, "Rimba Worktrees") {
		t.Error("expected list view by default")
	}

	// Help view.
	m.view = viewHelp
	out = m.View()
	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Error("expected help view")
	}

	// Conflict view.
	m.view = viewConflict
	out = m.View()
	if !strings.Contains(out, "Conflict Analysis") {
		t.Error("expected conflict view")
	}

	// Exec view.
	m.view = viewExec
	out = m.View()
	if !strings.Contains(out, "Exec Results") {
		t.Error("expected exec view")
	}
}

func TestViewStatusBar(t *testing.T) {
	m := model{statusMsg: "3 worktree(s)"}
	out := m.View()
	if !strings.Contains(out, "3 worktree(s)") {
		t.Error("expected status message in output")
	}
}

func TestViewErrorDisplay(t *testing.T) {
	m := model{err: errTest}
	out := m.View()
	if !strings.Contains(out, "test error") {
		t.Error("expected error in output")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestViewHelpFooter(t *testing.T) {
	m := model{}
	out := m.View()
	if !strings.Contains(out, "? help") {
		t.Error("expected help footer")
	}
	if !strings.Contains(out, "q quit") {
		t.Error("expected quit hint in footer")
	}
}

func TestNewModel(t *testing.T) {
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	m := New(nil, cfg)
	if m.cfg != cfg {
		t.Error("expected config to be set")
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := model{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	um := updated.(model)
	if um.width != 100 || um.height != 50 {
		t.Errorf("size = %dx%d, want 100x50", um.width, um.height)
	}
}

func TestUpdateWorktreesLoaded(t *testing.T) {
	m := model{}
	items := []worktreeItem{
		{Task: "a", Branch: testBranchA},
		{Task: "b", Branch: "bugfix/b"},
	}
	updated, _ := m.Update(worktreesLoadedMsg{items: items})
	um := updated.(model)
	if len(um.worktrees) != 2 {
		t.Errorf("got %d worktrees, want 2", len(um.worktrees))
	}
	if um.statusMsg != "2 worktree(s)" {
		t.Errorf("statusMsg = %q, want %q", um.statusMsg, "2 worktree(s)")
	}
}

func TestUpdateWorktreesLoadedError(t *testing.T) {
	m := model{}
	updated, _ := m.Update(worktreesLoadedMsg{err: errTest})
	um := updated.(model)
	if um.err == nil {
		t.Error("expected error to be set")
	}
}

func TestUpdateWorktreesLoadedCursorAdjust(t *testing.T) {
	m := model{cursor: 5}
	items := []worktreeItem{{Task: "a"}}
	updated, _ := m.Update(worktreesLoadedMsg{items: items})
	um := updated.(model)
	if um.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (adjusted to last)", um.cursor)
	}
}

func TestUpdateOperationDone(t *testing.T) {
	m := model{}
	updated, _ := m.Update(operationDoneMsg{msg: msgSyncedTaskA})
	um := updated.(model)
	if um.statusMsg != msgSyncedTaskA {
		t.Errorf("statusMsg = %q, want %q", um.statusMsg, msgSyncedTaskA)
	}
}

func TestUpdateOperationDoneError(t *testing.T) {
	m := model{}
	updated, _ := m.Update(operationDoneMsg{err: errTest})
	um := updated.(model)
	if !strings.Contains(um.statusMsg, "Error") {
		t.Errorf("statusMsg = %q, want it to contain 'Error'", um.statusMsg)
	}
}

func TestUpdateConflictDone(t *testing.T) {
	m := model{}
	analysis := &conflict.Analysis{}
	updated, _ := m.Update(conflictDoneMsg{analysis: analysis})
	um := updated.(model)
	if um.view != viewConflict {
		t.Errorf("view = %d, want %d (viewConflict)", um.view, viewConflict)
	}
	if um.conflictAnalysis != analysis {
		t.Error("expected analysis to be set")
	}
}

func TestUpdateConflictDoneError(t *testing.T) {
	m := model{}
	updated, _ := m.Update(conflictDoneMsg{err: errTest})
	um := updated.(model)
	if !strings.Contains(um.statusMsg, "Conflict check error") {
		t.Errorf("statusMsg = %q, want it to contain 'Conflict check error'", um.statusMsg)
	}
}

func TestHandleKeyQuit(t *testing.T) {
	m := model{}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestHandleKeyHelpToggle(t *testing.T) {
	m := model{view: viewList}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	um := updated.(model)
	if um.view != viewHelp {
		t.Errorf("view = %d, want %d (viewHelp)", um.view, viewHelp)
	}

	// Toggle back.
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	um = updated.(model)
	if um.view != viewList {
		t.Errorf("view = %d, want %d (viewList)", um.view, viewList)
	}
}

func TestHandleKeyEscape(t *testing.T) {
	m := model{view: viewHelp, err: errTest}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := updated.(model)
	if um.view != viewList {
		t.Errorf("view = %d, want %d (viewList)", um.view, viewList)
	}
	if um.err != nil {
		t.Error("expected error to be cleared")
	}
}

func TestHandleListKeyUpDown(t *testing.T) {
	m := model{
		view:      viewList,
		worktrees: []worktreeItem{{Task: "a"}, {Task: "b"}, {Task: "c"}},
		cursor:    1,
	}

	// Move up.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	um := updated.(model)
	if um.cursor != 0 {
		t.Errorf("cursor = %d, want 0", um.cursor)
	}

	// Move up again (should stay at 0).
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyUp})
	um = updated.(model)
	if um.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (at top)", um.cursor)
	}

	// Move down.
	um.cursor = 1
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyDown})
	um = updated.(model)
	if um.cursor != 2 {
		t.Errorf("cursor = %d, want 2", um.cursor)
	}

	// Move down past end (should stay at 2).
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyDown})
	um = updated.(model)
	if um.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (at bottom)", um.cursor)
	}
}

func TestHandleListKeyOnEmptyList(t *testing.T) {
	m := model{view: viewList, worktrees: nil, cursor: 0}

	// Sync with empty list should not panic.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = updated.(model)
	if cmd != nil {
		t.Error("expected no command on empty list")
	}
}

func TestHandleKeyInNonListView(t *testing.T) {
	m := model{
		view:      viewConflict,
		worktrees: []worktreeItem{{Task: "a"}},
		cursor:    0,
	}

	// Arrow keys should be ignored in non-list view (except quit/help/escape).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	um := updated.(model)
	if um.cursor != 0 {
		t.Errorf("cursor should not change in non-list view")
	}
}

const (
	testPorcelainOutput  = "worktree /repo\nHEAD 0000000\nbranch refs/heads/main\n\nworktree /wt/feature/task\nHEAD abc1234\nbranch refs/heads/feature/task\n\n"
	fmtExpectedOpDoneMsg = "expected operationDoneMsg, got %T"
	errExpected          = "expected error"
	testSharedFile       = "shared.go"
	testBranchA          = "feature/a"
	testBranchB          = "feature/b"
	testTaskA            = "task-a"
	testBranchTaskA      = "feature/task-a"
	msgSyncedTaskA       = "Synced task-a"
	fmtExpectedWtLoaded  = "expected worktreesLoadedMsg, got %T"
	fmtUnexpectedErr     = "unexpected error: %v"
)

// mockRunner implements git.Runner for TUI tests.
type mockRunner struct {
	runFn      func(args ...string) (string, error)
	runInDirFn func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	if m.runFn != nil {
		return m.runFn(args...)
	}
	return "", nil
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	if m.runInDirFn != nil {
		return m.runInDirFn(dir, args...)
	}
	return "", nil
}

func TestInit(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	m := New(r, cfg)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a command")
	}
	// Execute the command to exercise loadWorktrees.
	msg := cmd()
	if _, ok := msg.(worktreesLoadedMsg); !ok {
		t.Errorf(fmtExpectedWtLoaded, msg)
	}
}

func TestLoadWorktreesError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errTest
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := loadWorktrees(r, cfg)
	msg := cmd()
	loaded, ok := msg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf(fmtExpectedWtLoaded, msg)
	}
	if loaded.err == nil {
		t.Error("expected error in loaded message")
	}
}

func TestLoadWorktreesWithEntries(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return testPorcelainOutput, nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := loadWorktrees(r, cfg)
	msg := cmd()
	loaded, ok := msg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf(fmtExpectedWtLoaded, msg)
	}
	if loaded.err != nil {
		t.Fatalf(fmtUnexpectedErr, loaded.err)
	}
	// Should filter out main, leaving only the feature worktree.
	if len(loaded.items) != 1 {
		t.Fatalf("got %d items, want 1", len(loaded.items))
	}
	if loaded.items[0].Task != "task" {
		t.Errorf("Task = %q, want %q", loaded.items[0].Task, "task")
	}
}

func TestSyncWorktreeCmd(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return testPorcelainOutput, nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := syncWorktreeCmd(r, cfg, "task")
	msg := cmd()
	opDone, ok := msg.(operationDoneMsg)
	if !ok {
		t.Fatalf(fmtExpectedOpDoneMsg, msg)
	}
	if opDone.err != nil {
		t.Fatalf(fmtUnexpectedErr, opDone.err)
	}
	if !strings.Contains(opDone.msg, "Synced") {
		t.Errorf("msg = %q, want it to contain 'Synced'", opDone.msg)
	}
}

func TestSyncWorktreeCmdError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errTest
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := syncWorktreeCmd(r, cfg, "nonexistent")
	msg := cmd()
	opDone, ok := msg.(operationDoneMsg)
	if !ok {
		t.Fatalf(fmtExpectedOpDoneMsg, msg)
	}
	if opDone.err == nil {
		t.Error("expected error in operation")
	}
}

func TestRemoveWorktreeCmd(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return testPorcelainOutput, nil
		},
	}
	cmd := removeWorktreeCmd(r, "task")
	msg := cmd()
	opDone, ok := msg.(operationDoneMsg)
	if !ok {
		t.Fatalf(fmtExpectedOpDoneMsg, msg)
	}
	if opDone.err != nil {
		t.Fatalf(fmtUnexpectedErr, opDone.err)
	}
	if !strings.Contains(opDone.msg, "Removed") {
		t.Errorf("msg = %q, want it to contain 'Removed'", opDone.msg)
	}
}

func TestRemoveWorktreeCmdError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errTest
		},
	}
	cmd := removeWorktreeCmd(r, "nonexistent")
	msg := cmd()
	opDone := msg.(operationDoneMsg)
	if opDone.err == nil {
		t.Error(errExpected)
	}
}

func TestMergeWorktreeCmd(t *testing.T) {
	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "rev-parse --show-toplevel") {
				return "/repo", nil
			}
			return testPorcelainOutput, nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := mergeWorktreeCmd(r, cfg, "task")
	msg := cmd()
	opDone, ok := msg.(operationDoneMsg)
	if !ok {
		t.Fatalf(fmtExpectedOpDoneMsg, msg)
	}
	if opDone.err != nil {
		t.Fatalf(fmtUnexpectedErr, opDone.err)
	}
	if !strings.Contains(opDone.msg, "Merged") {
		t.Errorf("msg = %q, want it to contain 'Merged'", opDone.msg)
	}
}

func TestMergeWorktreeCmdError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errTest
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := mergeWorktreeCmd(r, cfg, "nonexistent")
	msg := cmd()
	opDone := msg.(operationDoneMsg)
	if opDone.err == nil {
		t.Error(errExpected)
	}
}

func TestConflictCheckCmd(t *testing.T) {
	// Need at least 2 branches for conflict analysis.
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return testPorcelainOutput, nil
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := conflictCheckCmd(r, cfg)
	msg := cmd()
	done, ok := msg.(conflictDoneMsg)
	if !ok {
		t.Fatalf("expected conflictDoneMsg, got %T", msg)
	}
	// With fewer than 2 branches, should return empty analysis.
	if done.err != nil {
		t.Fatalf(fmtUnexpectedErr, done.err)
	}
	if done.analysis == nil {
		t.Error("expected non-nil analysis")
	}
}

func TestConflictCheckCmdError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errTest
		},
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	cmd := conflictCheckCmd(r, cfg)
	msg := cmd()
	done := msg.(conflictDoneMsg)
	if done.err == nil {
		t.Error(errExpected)
	}
}

func assertListKeyAction(t *testing.T, key rune, wantStatus string, needsCfg bool) {
	t.Helper()
	r := &mockRunner{
		runFn:      func(_ ...string) (string, error) { return "", nil },
		runInDirFn: func(_ string, _ ...string) (string, error) { return "", nil },
	}
	var cfg *config.Config
	if needsCfg {
		cfg = &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	}
	m := model{
		view:      viewList,
		runner:    r,
		cfg:       cfg,
		worktrees: []worktreeItem{{Task: testTaskA, Branch: testBranchTaskA}},
		cursor:    0,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	um := updated.(model)
	if !strings.Contains(um.statusMsg, wantStatus) {
		t.Errorf("statusMsg = %q, want it to contain %q", um.statusMsg, wantStatus)
	}
	if cmd == nil {
		t.Errorf("expected command for %q action", wantStatus)
	}
}

func TestHandleListKeySyncAction(t *testing.T) {
	assertListKeyAction(t, 's', "Syncing", true)
}

func TestHandleListKeyRemoveAction(t *testing.T) {
	assertListKeyAction(t, 'd', "Removing", false)
}

func TestHandleListKeyMergeAction(t *testing.T) {
	assertListKeyAction(t, 'm', "Merging", true)
}

func TestHandleListKeyConflictAction(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) { return "", nil },
	}
	cfg := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
	m := model{
		view:      viewList,
		runner:    r,
		cfg:       cfg,
		worktrees: []worktreeItem{{Task: testTaskA, Branch: testBranchTaskA}},
		cursor:    0,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	um := updated.(model)
	if !strings.Contains(um.statusMsg, "conflict check") {
		t.Errorf("statusMsg = %q, want it to contain 'conflict check'", um.statusMsg)
	}
	if cmd == nil {
		t.Error("expected command for conflict action")
	}
}
