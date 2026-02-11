package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	branchMain     = "main"
	branchMaster   = "master"
	userHook       = "echo 'user hook running'\n"
	fatalInstall   = "Install: %v"
	fatalReadHook  = "read hook: %v"
	fatalWriteHook = "write hook: %v"
)

func TestHookBlock(t *testing.T) {
	block := HookBlock(branchMain)

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

func TestHookBlockCustomBranch(t *testing.T) {
	block := HookBlock(branchMaster)

	if !strings.Contains(block, `"master"`) {
		t.Errorf("block should contain master branch guard, got:\n%s", block)
	}
}

func TestInstallNewFile(t *testing.T) {
	dir := t.TempDir()

	if err := Install(dir, branchMain); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	hookPath := filepath.Join(dir, HookName)
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
	hookPath := filepath.Join(dir, HookName)

	existing := shebang + "\n\n" + userHook
	if err := os.WriteFile(hookPath, []byte(existing), fileMode); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	if err := Install(dir, branchMain); err != nil {
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

	if err := Install(dir, branchMain); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	err := Install(dir, branchMain)
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("second Install: got %v, want ErrAlreadyInstalled", err)
	}
}

func TestInstallCreatesHooksDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks")

	if err := Install(dir, branchMain); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := os.Stat(filepath.Join(dir, HookName)); err != nil {
		t.Fatalf("hook file should exist: %v", err)
	}
}

func TestUninstallRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, HookName)

	existing := shebang + "\n\n" + userHook + "\n" + HookBlock(branchMain) + "\n"
	if err := os.WriteFile(hookPath, []byte(existing), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	if err := Uninstall(dir); err != nil {
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
	hookPath := filepath.Join(dir, HookName)

	content := shebang + "\n\n" + HookBlock(branchMain) + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	if err := Uninstall(dir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should be deleted when only rimba content remains")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()

	err := Uninstall(dir)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Uninstall: got %v, want ErrNotInstalled", err)
	}
}

func TestUninstallNoBlock(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, HookName)

	if err := os.WriteFile(hookPath, []byte(shebang+"\n"+userHook), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	err := Uninstall(dir)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Uninstall: got %v, want ErrNotInstalled", err)
	}
}

func TestCheckInstalled(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir, branchMain); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	s := Check(dir)
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

	s := Check(dir)
	if s.Installed {
		t.Error("expected Installed = false")
	}
}

func TestCheckHasOtherContent(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, HookName)

	content := shebang + "\n\n" + userHook + "\n" + HookBlock(branchMain) + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	s := Check(dir)
	if !s.Installed {
		t.Error("expected Installed = true")
	}
	if !s.HasOther {
		t.Error("expected HasOther = true")
	}
}

func TestRemoveBlock(t *testing.T) {
	block := HookBlock(branchMain)
	content := shebang + "\n\n" + userHook + "\n" + block + "\n"

	result := removeBlock(content)
	if strings.Contains(result, BeginMarker) {
		t.Error("BEGIN marker should be removed")
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
	hookPath := filepath.Join(dir, HookName)

	// Hook file with user content but no rimba block
	content := shebang + "\n\n" + userHook
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	s := Check(dir)
	if s.Installed {
		t.Error("expected Installed = false")
	}
	if !s.HasOther {
		t.Error("expected HasOther = true")
	}
}

func TestUninstallReadError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, HookName)

	// Create a directory named "post-merge" instead of a file,
	// so os.ReadFile fails with a non-IsNotExist error.
	if err := os.Mkdir(hookPath, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	err := Uninstall(dir)
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
	hookPath := filepath.Join(dir, HookName)

	// Create a directory named "post-merge" so os.ReadFile returns non-IsNotExist error
	if err := os.Mkdir(hookPath, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	err := Install(dir, branchMain)
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

	err := Install(hooksDir, branchMain)
	if err == nil {
		t.Fatal("expected error from MkdirAll")
	}
	if !strings.Contains(err.Error(), "create hooks directory") {
		t.Errorf("error = %q, want to contain 'create hooks directory'", err.Error())
	}
}

func TestUninstallWriteError(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, HookName)

	// Create hook with rimba block AND user content so Uninstall tries WriteFile
	content := shebang + "\n\n" + userHook + "\n" + HookBlock(branchMain) + "\n"
	if err := os.WriteFile(hookPath, []byte(content), fileMode); err != nil {
		t.Fatalf(fatalWriteHook, err)
	}

	// Make the hook file read-only so WriteFile fails
	if err := os.Chmod(hookPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hookPath, 0644) })

	err := Uninstall(dir)
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if !strings.Contains(err.Error(), "write hook file") {
		t.Errorf("error = %q, want to contain 'write hook file'", err.Error())
	}
}
