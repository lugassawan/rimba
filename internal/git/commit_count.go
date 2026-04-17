package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CommitCountSince returns the number of commits on branch with a committer
// date newer than time.Now() - since. It wraps `git rev-list --count
// --since=<unix-ts> <branch>`, using a unix timestamp to avoid locale/TZ
// ambiguity in the --since parser.
//
// Note: git's --since stops walking the first-parent chain at the first
// commit older than the cutoff, so a backdated ancestor hides newer
// commits behind it. For the --detail "recent velocity" use case this is
// the intended behavior: it measures how much the branch tip has moved.
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
