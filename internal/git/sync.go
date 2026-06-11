package git

import "context"

// FetchArgs configures Fetch. The zero value performs a plain `git fetch <remote>`.
type FetchArgs struct {
	Prune bool // append --prune: drop remote-tracking refs whose upstream branch was deleted
}

// Fetch runs `git fetch <remote>`, applying any options in args, to update
// remote-tracking branches.
func Fetch(ctx context.Context, r Runner, remote string, args FetchArgs) error {
	gitArgs := []string{"fetch"}
	if args.Prune {
		gitArgs = append(gitArgs, "--prune")
	}
	gitArgs = append(gitArgs, remote)
	_, err := r.Run(ctx, gitArgs...)
	return err
}

// Rebase runs `git rebase <branch>` inside the given directory.
func Rebase(ctx context.Context, r Runner, dir, branch string) error {
	_, err := r.RunInDir(ctx, dir, "rebase", branch)
	return err
}

// AbortRebase runs `git rebase --abort` inside the given directory.
// Intentionally non-cancellable: rebase recovery must succeed even after Ctrl-C.
func AbortRebase(r Runner, dir string) error {
	_, err := r.RunInDir(context.Background(), dir, "rebase", "--abort")
	return err
}

// MergeBase returns the merge-base commit of two refs.
func MergeBase(ctx context.Context, r Runner, ref1, ref2 string) (string, error) {
	return r.Run(ctx, cmdMergeBase, ref1, ref2)
}

// IsMergeBaseAncestor checks if ancestor is an ancestor of descendant
// using `git merge-base --is-ancestor`.
func IsMergeBaseAncestor(ctx context.Context, r Runner, ancestor, descendant string) bool {
	_, err := r.Run(ctx, cmdMergeBase, "--is-ancestor", ancestor, descendant)
	return err == nil
}

// HasUpstream checks whether the current branch in dir has a remote tracking branch.
func HasUpstream(ctx context.Context, r Runner, dir string) bool {
	_, err := r.RunInDir(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return err == nil
}

// Push runs `git push` inside the given directory.
func Push(ctx context.Context, r Runner, dir string) error {
	_, err := r.RunInDir(ctx, dir, "push")
	return err
}

// PushForceWithLease runs `git push --force-with-lease` inside the given directory.
func PushForceWithLease(ctx context.Context, r Runner, dir string) error {
	_, err := r.RunInDir(ctx, dir, "push", "--force-with-lease")
	return err
}
