package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	branchMain            = "main"
	branchMaster          = "master"
	userHook              = "echo 'user hook running'\n"
	fatalInstall          = "Install: %v"
	fatalReadHook         = "read hook: %v"
	fatalWriteHook        = "write hook: %v"
	errBeginMarkerRemoved = "BEGIN marker should be removed"
)

func postMergeBlock() string {
	return PostMergeBlock(branchMain)
}

// --- PostMergeBlock (post-merge) tests ---

func TestPostMergeBlock(t *testing.T) {
	block := PostMergeBlock(branchMain)

	if !strings.Contains(block, BeginMarker) {
		t.Error("block missing BEGIN marker")
	}
	if !strings.Contains(block, EndMarker) {
		t.Error("block missing END marker")
	}
	if !strings.Contains(block, `"main"`) {
		t.Error("block missing branch guard for main")
	}
	if !strings.Contains(block, "rimba clean --merged --force") {
		t.Error("block missing clean command")
	}
	if !strings.Contains(block, "command -v rimba") {
		t.Error("block missing rimba existence check")
	}
}

func TestPostMergeBlockCustomBranch(t *testing.T) {
	block := PostMergeBlock(branchMaster)

	if !strings.Contains(block, `"master"`) {
		t.Errorf("block should contain master branch guard, got:\n%s", block)
	}
}

// --- PreCommitBlock tests ---

func TestPreCommitBlock(t *testing.T) {
	block := PreCommitBlock()

	if !strings.Contains(block, BeginMarker) {
		t.Error("block missing BEGIN marker")
	}
	if !strings.Contains(block, EndMarker) {
		t.Error("block missing END marker")
	}
	if !strings.Contains(block, `"main"`) {
		t.Error("block missing guard for main")
	}
	if !strings.Contains(block, `"master"`) {
		t.Error("block missing guard for master")
	}
	if !strings.Contains(block, "exit 1") {
		t.Error("block missing exit 1")
	}
	if !strings.Contains(block, "rimba add") {
		t.Error("block missing hint to use rimba add")
	}
}

// --- Install tests (post-merge) ---

func TestInstallNewFile(t *testing.T) {
	dir := t.TempDir()

	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	hookPath := filepath.Join(dir, PostMergeHook)
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}

	s := string(content)
	if !strings.HasPrefix(s, shebang) {
		t.Error("hook file should start with shebang")
	}
	if !strings.Contains(s, BeginMarker) {
		t.Error("hook file missing BEGIN marker")
	}
	if !strings.Contains(s, EndMarker) {
		t.Error("hook file missing END marker")
	}

	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if info.Mode().Perm() != fileMode {
		t.Errorf("hook file mode = %o, want %o", info.Mode().Perm(), fileMode)
	}
}

func TestInstallAppendToExisting(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	existing := shebang + "\n\n" + userHook
	if err := os.WriteFile(hookPath, []byte(existing), fileMode); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}

	s := string(content)
	if !strings.Contains(s, userHook) {
		t.Error("existing user hook content should be preserved")
	}
	if !strings.Contains(s, BeginMarker) {
		t.Error("rimba block should be appended")
	}
}

func TestInstallAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()

	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	err := Install(dir, PostMergeHook, postMergeBlock())
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("second Install: got %v, want ErrAlreadyInstalled", err)
	}
}

func TestInstallCreatesHooksDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks")

	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := os.Stat(filepath.Join(dir, PostMergeHook)); err != nil {
		t.Fatalf("hook file should exist: %v", err)
	}
}

// --- Install tests (pre-commit) ---

func TestInstallPreCommitNewFile(t *testing.T) {
	dir := t.TempDir()

	if err := Install(dir, PreCommitHook, PreCommitBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	hookPath := filepath.Join(dir, PreCommitHook)
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}

	s := string(content)
	if !strings.HasPrefix(s, shebang) {
		t.Error("hook file should start with shebang")
	}
	if !strings.Contains(s, `"main"`) {
		t.Error("pre-commit hook missing main guard")
	}
	if !strings.Contains(s, `"master"`) {
		t.Error("pre-commit hook missing master guard")
	}
	if !strings.Contains(s, "exit 1") {
		t.Error("pre-commit hook missing exit 1")
	}
}

func TestInstallPreCommitAppendToExisting(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PreCommitHook)

	existing := shebang + "\n\n" + userHook
	if err := os.WriteFile(hookPath, []byte(existing), fileMode); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(dir, PreCommitHook, PreCommitBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}

	s := string(content)
	if !strings.Contains(s, userHook) {
		t.Error("existing user hook content should be preserved")
	}
	if !strings.Contains(s, "exit 1") {
		t.Error("pre-commit block should be appended")
	}
}

// --- Uninstall tests (post-merge) ---

func TestUninstallRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	existing := shebang + "\n\n" + userHook + "\n" + postMergeBlock() + "\n"
	if err := os.WriteFile(hookPath, []byte(existing), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	if err := Uninstall(dir, PostMergeHook); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf(fatalReadHook, err)
	}

	s := string(content)
	if strings.Contains(s, BeginMarker) {
		t.Error("rimba block should be removed")
	}
	if !strings.Contains(s, userHook) {
		t.Error("user hook content should be preserved")
	}
}

func TestUninstallRemovesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	content := shebang + "\n\n" + postMergeBlock() + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	if err := Uninstall(dir, PostMergeHook); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should be deleted when only rimba content remains")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()

	err := Uninstall(dir, PostMergeHook)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Uninstall: got %v, want ErrNotInstalled", err)
	}
}

func TestUninstallNoBlock(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	if err := os.WriteFile(hookPath, []byte(shebang+"\n"+userHook), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	err := Uninstall(dir, PostMergeHook)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Uninstall: got %v, want ErrNotInstalled", err)
	}
}

// --- Uninstall tests (pre-commit) ---

func TestUninstallPreCommit(t *testing.T) {
	dir := t.TempDir()

	if err := Install(dir, PreCommitHook, PreCommitBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if err := Uninstall(dir, PreCommitHook); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	hookPath := filepath.Join(dir, PreCommitHook)
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("pre-commit hook file should be deleted when only rimba content remains")
	}
}

// --- Check tests (post-merge) ---

func TestCheckInstalled(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	s := Check(dir, PostMergeHook)
	if !s.Installed {
		t.Error("expected Installed = true")
	}
	if s.HasOther {
		t.Error("expected HasOther = false")
	}
	if s.HookPath == "" {
		t.Error("expected non-empty HookPath")
	}
}

func TestCheckNotInstalled(t *testing.T) {
	dir := t.TempDir()

	s := Check(dir, PostMergeHook)
	if s.Installed {
		t.Error("expected Installed = false")
	}
}

func TestCheckHasOtherContent(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	content := shebang + "\n\n" + userHook + "\n" + postMergeBlock() + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	s := Check(dir, PostMergeHook)
	if !s.Installed {
		t.Error("expected Installed = true")
	}
	if !s.HasOther {
		t.Error("expected HasOther = true")
	}
}

// --- Check tests (pre-commit) ---

func TestCheckPreCommit(t *testing.T) {
	dir := t.TempDir()

	s := Check(dir, PreCommitHook)
	if s.Installed {
		t.Error("expected Installed = false before install")
	}

	if err := Install(dir, PreCommitHook, PreCommitBlock()); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	s = Check(dir, PreCommitHook)
	if !s.Installed {
		t.Error("expected Installed = true after install")
	}
}

// --- Both hooks independent ---

func TestBothHooksIndependent(t *testing.T) {
	dir := t.TempDir()

	// Install both hooks
	if err := Install(dir, PostMergeHook, postMergeBlock()); err != nil {
		t.Fatalf("install post-merge: %v", err)
	}
	if err := Install(dir, PreCommitHook, PreCommitBlock()); err != nil {
		t.Fatalf("install pre-commit: %v", err)
	}

	// Both should be installed
	if !Check(dir, PostMergeHook).Installed {
		t.Error("post-merge should be installed")
	}
	if !Check(dir, PreCommitHook).Installed {
		t.Error("pre-commit should be installed")
	}

	// Uninstall post-merge only
	if err := Uninstall(dir, PostMergeHook); err != nil {
		t.Fatalf("uninstall post-merge: %v", err)
	}

	// Post-merge should be gone, pre-commit should survive
	if Check(dir, PostMergeHook).Installed {
		t.Error("post-merge should be uninstalled")
	}
	if !Check(dir, PreCommitHook).Installed {
		t.Error("pre-commit should still be installed after uninstalling post-merge")
	}
}

// --- removeBlock tests ---

func TestRemoveBlock(t *testing.T) {
	block := postMergeBlock()
	content := shebang + "\n\n" + userHook + "\n" + block + "\n"

	result := removeBlock(content)
	if strings.Contains(result, BeginMarker) {
		t.Error(errBeginMarkerRemoved)
	}
	if strings.Contains(result, EndMarker) {
		t.Error("END marker should be removed")
	}
	if !strings.Contains(result, userHook) {
		t.Error("user content should be preserved")
	}
}

func TestRemoveBlockNoBeginMarker(t *testing.T) {
	content := shebang + "\n\n" + userHook
	result := removeBlock(content)
	if result != content {
		t.Errorf("removeBlock should return content unchanged when no markers present\ngot:  %q\nwant: %q", result, content)
	}
}

func TestRemoveBlockCorruptBeginOnly(t *testing.T) {
	// BEGIN at the very start of content, no END marker
	content := BeginMarker + "\nsome corrupt content\nmore stuff"
	result := removeBlock(content)
	if result != "" {
		t.Errorf("removeBlock should return empty string when BEGIN is at start and no END, got %q", result)
	}
}

func TestCheckHasOtherWithoutInstalled(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	// Hook file with user content but no rimba block
	content := shebang + "\n\n" + userHook
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	s := Check(dir, PostMergeHook)
	if s.Installed {
		t.Error("expected Installed = false")
	}
	if !s.HasOther {
		t.Error("expected HasOther = true")
	}
}

func TestUninstallReadError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	// Create a directory named "post-merge" instead of a file,
	// so os.ReadFile fails with a non-IsNotExist error.
	if err := os.Mkdir(hookPath, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	err := Uninstall(dir, PostMergeHook)
	if err == nil {
		t.Fatal("expected error when hook path is a directory")
	}
	if !strings.Contains(err.Error(), "read hook file") {
		t.Errorf("error = %q, want to contain 'read hook file'", err.Error())
	}
}

func TestRemoveBlockNoEndMarker(t *testing.T) {
	// Simulate corrupt file: BEGIN without END
	content := shebang + "\n\n" + userHook + "\n" + BeginMarker + "\nsome corrupt content"

	result := removeBlock(content)
	if strings.Contains(result, BeginMarker) {
		t.Error("BEGIN marker should be removed even without END")
	}
	if !strings.Contains(result, userHook) {
		t.Error("user content before block should be preserved")
	}
}

func TestInstallReadError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	// Create a directory named "post-merge" so os.ReadFile returns non-IsNotExist error
	if err := os.Mkdir(hookPath, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	err := Install(dir, PostMergeHook, postMergeBlock())
	if err == nil {
		t.Fatal("expected error when hook path is a directory")
	}
	if !strings.Contains(err.Error(), "read hook file") {
		t.Errorf("error = %q, want to contain 'read hook file'", err.Error())
	}
}

func TestInstallMkdirError(t *testing.T) {
	// Use a file path as the hooks directory — MkdirAll should fail
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	hooksDir := filepath.Join(blocker, "hooks")

	err := Install(hooksDir, PostMergeHook, postMergeBlock())
	if err == nil {
		t.Fatal("expected error from MkdirAll")
	}
	if !strings.Contains(err.Error(), "create hooks directory") {
		t.Errorf("error = %q, want to contain 'create hooks directory'", err.Error())
	}
}

func TestUninstallWriteError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	// Create hook with rimba block AND user content so Uninstall tries WriteFile
	content := shebang + "\n\n" + userHook + "\n" + postMergeBlock() + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	// Make the hook file read-only so WriteFile fails
	if err := os.Chmod(hookPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hookPath, 0644) })

	err := Uninstall(dir, PostMergeHook)
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if !strings.Contains(err.Error(), "write hook file") {
		t.Errorf("error = %q, want to contain 'write hook file'", err.Error())
	}
}

func TestRemoveBlockBeginOnlyAtStart(t *testing.T) {
	// BEGIN marker at very start of file with content before END
	content := BeginMarker + "\nrimba hook content\n" + EndMarker + "\n"
	result := removeBlock(content)
	if strings.Contains(result, BeginMarker) {
		t.Error(errBeginMarkerRemoved)
	}
	if strings.Contains(result, EndMarker) {
		t.Error("END marker should be removed")
	}
	// After removing the block at the start, result should be empty
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestRemoveBlockContentAfterEnd(t *testing.T) {
	// Rimba block at the start, user content after END
	block := postMergeBlock()
	afterContent := "echo 'runs after rimba'\n"
	content := shebang + "\n\n" + block + "\n" + afterContent

	result := removeBlock(content)
	if strings.Contains(result, BeginMarker) {
		t.Error(errBeginMarkerRemoved)
	}
	if !strings.Contains(result, afterContent) {
		t.Errorf("content after END should be preserved, got %q", result)
	}
	if !strings.HasPrefix(result, shebang) {
		t.Errorf("shebang should be preserved at start, got %q", result)
	}
}

func TestUninstallRemoveError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, PostMergeHook)

	// Create hook with only rimba content (shebang-only after removal → file deleted)
	content := shebang + "\n\n" + postMergeBlock() + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	// Make directory read-only so os.Remove fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := Uninstall(dir, PostMergeHook)
	if err == nil {
		t.Fatal("expected error when file removal fails")
	}
	if !strings.Contains(err.Error(), "remove hook file") {
		t.Errorf("error = %q, want to contain 'remove hook file'", err.Error())
	}
}

func TestInstallWriteError(t *testing.T) {
	dir := t.TempDir()

	// Make directory read-only so WriteFile fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	err := Install(dir, PostMergeHook, postMergeBlock())
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if !strings.Contains(err.Error(), "write hook file") {
		t.Errorf("error = %q, want to contain 'write hook file'", err.Error())
	}
}
