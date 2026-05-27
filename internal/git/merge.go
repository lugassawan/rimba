package git

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
func MergeInProgress(r Runner, dir string) bool {
	_, err := r.RunInDir(dir, "rev-parse", "--verify", "-q", "MERGE_HEAD")
	return err == nil
}

// MergeAbort runs `git merge --abort` in dir.
func MergeAbort(r Runner, dir string) error {
	_, err := r.RunInDir(dir, "merge", "--abort")
	return err
}
