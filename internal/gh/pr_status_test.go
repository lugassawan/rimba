package gh

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

const statusFailure = "FAILURE"

func TestQueryPRStatusNoOpenPR(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("[]\n"), nil
		},
	}

	got, err := QueryPRStatus(context.Background(), runner, "feature/none")
	if err != nil {
		t.Fatalf("QueryPRStatus() err = %v, want nil", err)
	}
	if (got != PRStatus{}) {
		t.Errorf("QueryPRStatus() = %+v, want zero value", got)
	}
}

func TestQueryPRStatusSuccessRollup(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte(`[{"number":412,"statusCheckRollup":[{"conclusion":"SUCCESS"},{"conclusion":"SUCCESS"}]}]`), nil
		},
	}

	got, err := QueryPRStatus(context.Background(), runner, "feature/auth")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := PRStatus{Number: 412, CIStatus: "SUCCESS"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestQueryPRStatusPendingRollup(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte(`[{"number":418,"statusCheckRollup":[{"conclusion":"SUCCESS"},{"status":"IN_PROGRESS"}]}]`), nil
		},
	}

	got, err := QueryPRStatus(context.Background(), runner, "fix/race")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.CIStatus != "PENDING" || got.Number != 418 {
		t.Errorf("got %+v, want PENDING #418", got)
	}
}

func TestQueryPRStatusFailureWins(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte(`[{"number":501,"statusCheckRollup":[{"conclusion":"SUCCESS"},{"conclusion":"FAILURE"},{"status":"IN_PROGRESS"}]}]`), nil
		},
	}

	got, err := QueryPRStatus(context.Background(), runner, "pr/501")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.CIStatus != statusFailure {
		t.Errorf("got %+v, want FAILURE", got)
	}
}

func TestQueryPRStatusEmptyRollup(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte(`[{"number":999,"statusCheckRollup":[]}]`), nil
		},
	}

	got, err := QueryPRStatus(context.Background(), runner, "some")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := PRStatus{Number: 999, CIStatus: ""}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestQueryPRStatusInvalidJSON(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("not json"), nil
		},
	}

	_, err := QueryPRStatus(context.Background(), runner, "x")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestQueryPRStatusRunnerError(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return nil, errors.New("exit 1")
		},
	}

	_, err := QueryPRStatus(context.Background(), runner, "x")
	if err == nil {
		t.Fatal("expected runner error, got nil")
	}
}

func TestQueryPRStatusArgsPassedToRunner(t *testing.T) {
	var captured []string
	runner := &mockRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			captured = args
			return []byte("[]"), nil
		},
	}

	if _, err := QueryPRStatus(context.Background(), runner, "feature/x"); err != nil {
		t.Fatalf("err = %v", err)
	}

	want := []string{"pr", "list", "--head", "feature/x", "--state", "open", "--json", "number,statusCheckRollup", "--limit", "1"}
	if !reflect.DeepEqual(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestQueryPRStatusActionRequiredMapsToFailure(t *testing.T) {
	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte(`[{"number":7,"statusCheckRollup":[{"conclusion":"ACTION_REQUIRED"}]}]`), nil
		},
	}

	got, _ := QueryPRStatus(context.Background(), runner, "x")
	if got.CIStatus != statusFailure {
		t.Errorf("ACTION_REQUIRED: got %s, want FAILURE", got.CIStatus)
	}
}
