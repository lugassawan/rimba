package gh

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

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

func TestExecRunnerRunFailureFallsBackToStdoutWhenStderrEmpty(t *testing.T) {
	writeFakeGh(t, `echo "rate limit exceeded"
exit 1
`)

	_, err := Default(0).Run(context.Background(), "pr", "list")
	assertContains(t, err, "rate limit exceeded")
}

func TestExecRunnerRunFailureConcatenatesStderrAndStdout(t *testing.T) {
	writeFakeGh(t, `echo "partial output before the crash"
echo "error: unexpected EOF" >&2
exit 1
`)

	_, err := Default(0).Run(context.Background(), "pr", "list")
	assertContains(t, err, "error: unexpected EOF")
	assertContains(t, err, "partial output before the crash")
}

func TestExecRunnerRunSetsNoUpdateNotifierEnv(t *testing.T) {
	writeFakeGh(t, `echo "{\"notifier\":\"$GH_NO_UPDATE_NOTIFIER\"}"
exit 0
`)

	out, err := Default(0).Run(context.Background(), "pr", "list")
	if err != nil {
		t.Fatalf("Run() err = %v, want nil", err)
	}
	if want := `"notifier":"1"`; !strings.Contains(string(out), want) {
		t.Errorf("out = %q, want it to contain %q", out, want)
	}
}
