package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ghStubScript is a fake `gh` that answers `auth status` and `pr list`
// from files in $GH_STUB_DIR. An `unauth` file flips auth to exit 1.
const ghStubScript = `#!/bin/sh
case "$1" in
  auth)
    if [ -f "$GH_STUB_DIR/unauth" ]; then
      echo "not logged in" >&2
      exit 1
    fi
    echo "Logged in"
    exit 0
    ;;
  pr)
    branch="$4"
    f="$GH_STUB_DIR/pr_$branch.json"
    if [ -f "$f" ]; then
      cat "$f"
    else
      echo "[]"
    fi
    exit 0
    ;;
esac
echo "unexpected gh invocation: $*" >&2
exit 2
`

// stubGh installs ghStubScript as `gh` on a fresh dir and returns a PATH
// env entry that prepends it.
func stubGh(t *testing.T) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(ghStubScript), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	env = []string{"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")}
	return dir, env
}

// writeStubPR writes the JSON response for one branch. Slashes in branch
// names become subdirectories under stubDir.
func writeStubPR(t *testing.T, stubDir, branch, content string) {
	t.Helper()
	p := filepath.Join(stubDir, "pr_"+branch+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write stub: %v", err)
	}
}

func TestListFullWithGhStubShowsPRAndCI(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "feat-auth")
	rimbaSuccess(t, repo, "add", "feat-pending")
	rimbaSuccess(t, repo, "add", "feat-failing")
	rimbaSuccess(t, repo, "add", "feat-nopr")

	stubDir, env := stubGh(t)
	env = append(env, "GH_STUB_DIR="+stubDir)

	writeStubPR(t, stubDir, "feature/feat-auth",
		`[{"number":412,"statusCheckRollup":[{"conclusion":"SUCCESS"}]}]`)
	writeStubPR(t, stubDir, "feature/feat-pending",
		`[{"number":418,"statusCheckRollup":[{"conclusion":"SUCCESS"},{"status":"IN_PROGRESS"}]}]`)
	writeStubPR(t, stubDir, "feature/feat-failing",
		`[{"number":501,"statusCheckRollup":[{"conclusion":"FAILURE"}]}]`)

	r := rimbaWithEnv(t, repo, env, "list", "--full")
	if r.ExitCode != 0 {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	for _, want := range []string{"PR", "CI", "#412", "#418", "●", "#501", "✗"} {
		assertContains(t, r.Stdout, want)
	}
	if !strings.Contains(r.Stdout, "feat-nopr") {
		t.Errorf("expected feat-nopr row in:\n%s", r.Stdout)
	}
}

func TestListFullWithGhStubJSONEmitsPRFields(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "feat-json")

	stubDir, env := stubGh(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	writeStubPR(t, stubDir, "feature/feat-json",
		`[{"number":777,"statusCheckRollup":[{"conclusion":"SUCCESS"}]}]`)

	r := rimbaWithEnv(t, repo, env, "list", "--full", "--json")
	if r.ExitCode != 0 {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	var payload struct {
		Data []struct {
			Task     string  `json:"task"`
			PRNumber *int    `json:"pr_number"`
			CIStatus *string `json:"ci_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, r.Stdout)
	}

	var match bool
	for _, row := range payload.Data {
		if row.Task == "feat-json" {
			match = true
			if row.PRNumber == nil || *row.PRNumber != 777 {
				t.Errorf("pr_number = %v, want 777", row.PRNumber)
			}
			if row.CIStatus == nil || *row.CIStatus != "SUCCESS" {
				t.Errorf("ci_status = %v, want SUCCESS", row.CIStatus)
			}
		}
	}
	if !match {
		t.Errorf("feat-json row missing; payload: %+v", payload)
	}
}

func TestListFullWarnsWhenGhUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "feat-unauth")

	stubDir, env := stubGh(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	if err := os.WriteFile(filepath.Join(stubDir, "unauth"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	r := rimbaWithEnv(t, repo, env, "list", "--full")
	if r.ExitCode != 0 {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	assertContains(t, r.Stdout, "gh unavailable")
	for _, want := range []string{"PR", "CI", "feat-unauth"} {
		assertContains(t, r.Stdout, want)
	}
}

func TestListFullJSONOmitsCIStatusWhenNoChecks(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "feat-nochecks")

	stubDir, env := stubGh(t)
	env = append(env, "GH_STUB_DIR="+stubDir)
	writeStubPR(t, stubDir, "feature/feat-nochecks",
		`[{"number":42,"statusCheckRollup":[]}]`)

	r := rimbaWithEnv(t, repo, env, "list", "--full", "--json")
	if r.ExitCode != 0 {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

	if strings.Contains(r.Stdout, `"ci_status"`) {
		t.Errorf("expected ci_status omitted when rollup empty:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stdout, `"pr_number": 42`) {
		t.Errorf("expected pr_number 42 in output:\n%s", r.Stdout)
	}
}

func TestListJSONOmitsPRFieldsWithoutFullFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "no-full-task")

	r := rimbaSuccess(t, repo, "list", "--json")
	if strings.Contains(r.Stdout, "pr_number") {
		t.Errorf("pr_number should be absent without --full:\n%s", r.Stdout)
	}
	if strings.Contains(r.Stdout, "ci_status") {
		t.Errorf("ci_status should be absent without --full:\n%s", r.Stdout)
	}
}
