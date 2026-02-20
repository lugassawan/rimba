package git

// Fetch runs `git fetch <remote>` to update remote tracking branches.
func Fetch(r Runner, remote string) error {
	_, err := r.Run("fetch", remote)
	return err
}

// Rebase runs `git rebase <branch>` inside the given directory.
func Rebase(r Runner, dir, branch string) error {
	_, err := r.RunInDir(dir, "rebase", branch)
	return err
}

// AbortRebase runs `git rebase --abort` inside the given directory.
func AbortRebase(r Runner, dir string) error {
	_, err := r.RunInDir(dir, "rebase", "--abort")
	return err
}

// MergeBase returns the merge-base commit of two refs.
func MergeBase(r Runner, ref1, ref2 string) (string, error) {
	return r.Run("merge-base", ref1, ref2)
}

// IsMergeBaseAncestor checks if ancestor is an ancestor of descendant
// using `git merge-base --is-ancestor`.
func IsMergeBaseAncestor(r Runner, ancestor, descendant string) bool {
	_, err := r.Run("merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil
}

// HasUpstream checks whether the current branch in dir has a remote tracking branch.
func HasUpstream(r Runner, dir string) bool {
	_, err := r.RunInDir(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return err == nil
}

// Push runs `git push` inside the given directory.
func Push(r Runner, dir string) error {
	_, err := r.RunInDir(dir, "push")
	return err
}

// PushForceWithLease runs `git push --force-with-lease` inside the given directory.
func PushForceWithLease(r Runner, dir string) error {
	_, err := r.RunInDir(dir, "push", "--force-with-lease")
	return err
}
