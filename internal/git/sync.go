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
