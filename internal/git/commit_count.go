package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CommitCountSince counts commits on branch within the last `since`
// duration, via `git rev-list --count --since=<unix-ts>`.
//
// git's --since stops at the first commit older than the cutoff, so an
// older ancestor hides anything beyond it. That matches what we want
// here: recent activity on the tip, not total commits in history.
func CommitCountSince(r Runner, branch string, since time.Duration) (int, error) {
	cutoff := time.Now().Add(-since).Unix()
	out, err := r.Run(
		"rev-list", "--count",
		fmt.Sprintf("--since=%d", cutoff),
		branch,
	)
	if err != nil {
		return 0, fmt.Errorf("commit count since for %s: %w", branch, err)
	}

	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("parse commit count %q: %w", out, err)
	}
	return n, nil
}
