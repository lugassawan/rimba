package gh

import (
	"context"
	"encoding/json"
	"fmt"
)

// PRStatus is the open PR for a branch. Zero value means no open PR.
type PRStatus struct {
	Number   int
	CIStatus string
}

type prCheck struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type prListEntry struct {
	Number            int       `json:"number"`
	StatusCheckRollup []prCheck `json:"statusCheckRollup"`
}

// QueryPRStatus returns the open PR and CI rollup for branch.
// No open PR returns the zero PRStatus with a nil error.
func QueryPRStatus(ctx context.Context, r Runner, branch string) (PRStatus, error) {
	out, err := r.Run(ctx, "pr", "list", "--head", branch, "--state", "open", "--json", "number,statusCheckRollup", "--limit", "1")
	if err != nil {
		return PRStatus{}, err
	}
	var entries []prListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return PRStatus{}, fmt.Errorf("parse gh pr list: %w", err)
	}
	if len(entries) == 0 {
		return PRStatus{}, nil
	}
	e := entries[0]
	return PRStatus{Number: e.Number, CIStatus: reduceRollup(e.StatusCheckRollup)}, nil
}

func reduceRollup(checks []prCheck) string {
	if len(checks) == 0 {
		return ""
	}
	failure := map[string]bool{
		"FAILURE": true, "ERROR": true, "TIMED_OUT": true,
		"CANCELLED": true, "ACTION_REQUIRED": true,
	}
	pending := map[string]bool{
		"IN_PROGRESS": true, "QUEUED": true, "WAITING": true, "PENDING": true,
	}
	sawPending := false
	for _, c := range checks {
		if failure[c.Conclusion] {
			return "FAILURE"
		}
		if pending[c.Status] {
			sawPending = true
		}
	}
	if sawPending {
		return "PENDING"
	}
	return "SUCCESS"
}
