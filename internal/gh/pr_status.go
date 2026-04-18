package gh

import (
	"context"
	"encoding/json"
	"fmt"
)

// CIStatus is the aggregated CI rollup state for a PR.
type CIStatus string

// Rollup states reduceRollup can produce. An empty CIStatus means
// "no checks reported" (distinct from "no PR").
const (
	CIStatusSuccess CIStatus = "SUCCESS"
	CIStatusPending CIStatus = "PENDING"
	CIStatusFailure CIStatus = "FAILURE"
)

// PRStatus is the open PR for a branch. Zero value means no open PR.
type PRStatus struct {
	Number   int
	CIStatus CIStatus
}

type prCheck struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type prListEntry struct {
	Number            int       `json:"number"`
	StatusCheckRollup []prCheck `json:"statusCheckRollup"`
}

// Raw gh rollup states grouped by how reduceRollup collapses them.
var (
	failureConclusions = map[string]bool{
		"FAILURE":         true,
		"ERROR":           true,
		"TIMED_OUT":       true,
		"CANCELLED":       true,
		"ACTION_REQUIRED": true,
	}
	pendingStatuses = map[string]bool{
		"IN_PROGRESS": true,
		"QUEUED":      true,
		"WAITING":     true,
		"PENDING":     true,
	}
)

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

func reduceRollup(checks []prCheck) CIStatus {
	if len(checks) == 0 {
		return ""
	}
	sawPending := false
	for _, c := range checks {
		if failureConclusions[c.Conclusion] {
			return CIStatusFailure
		}
		if pendingStatuses[c.Status] {
			sawPending = true
		}
	}
	if sawPending {
		return CIStatusPending
	}
	return CIStatusSuccess
}
