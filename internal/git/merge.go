package git

import (
	"fmt"
	"strings"
)

// Merge runs `git merge [--no-ff] <branch>` inside the given directory.
func Merge(r Runner, dir, branch string, noFF bool) error {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	_, err := r.RunInDir(dir, args...)
	return err
}

// MergeInProgress reports whether a merge is in progress in dir (MERGE_HEAD exists).
// Returns (false, nil) when no merge is in progress, (true, nil) when one is,
// and (false, err) when the check itself fails (infrastructure error).
func MergeInProgress(r Runner, dir string) (bool, error) {
	_, err := r.RunInDir(dir, "rev-parse", "--verify", "-q", "MERGE_HEAD")
	if err == nil {
		return true, nil
	}
	// rev-parse exits non-zero when MERGE_HEAD doesn't exist; that is not an error.
	// Distinguish "ref not found" (exit 128 / message mentions MERGE_HEAD or missing-ref
	// text) from genuine infrastructure failures by checking the error text.
	s := err.Error()
	if strings.Contains(s, "MERGE_HEAD") ||
		strings.Contains(s, "unknown revision") ||
		strings.Contains(s, "Needed a single revision") {
		return false, nil
	}
	return false, fmt.Errorf("checking merge state: %w", err)
}

// MergeAbort runs `git merge --abort` in dir.
func MergeAbort(r Runner, dir string) error {
	_, err := r.RunInDir(dir, "merge", "--abort")
	return err
}
