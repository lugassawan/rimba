package gh

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeGh installs a `gh` script on PATH for the test's lifetime.
func writeFakeGh(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "gh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecRunnerRunStdoutOnlyIgnoresStderrNoise(t *testing.T) {
	writeFakeGh(t, `echo '{"number":42}'
echo "warning: a new release of gh is available" >&2
exit 0
`)

	out, err := Default(0).Run(context.Background(), "pr", "list")
	if err != nil {
		t.Fatalf("Run() err = %v, want nil", err)
	}

	var got struct{ Number int }
	if jsonErr := json.Unmarshal(out, &got); jsonErr != nil {
		t.Fatalf("Unmarshal(%q) = %v, want nil", out, jsonErr)
	}
	if got.Number != 42 {
		t.Errorf("Number = %d, want 42", got.Number)
	}
}

func TestExecRunnerRunStderrSurfacesOnFailure(t *testing.T) {
	writeFakeGh(t, `echo "error: authentication required" >&2
exit 1
`)

	_, err := Default(0).Run(context.Background(), "pr", "list")
	assertContains(t, err, "authentication required")
}
