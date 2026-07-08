package git

import (
	"context"
	"strings"
)

// ListIgnoredUntracked lists untracked files git ignores under dir, scoped
// to pathspecs. Existence is implied (untracked-but-present); tracked files
// are excluded — this is exactly the copy_files auto-detection use case.
func ListIgnoredUntracked(ctx context.Context, r Runner, dir string, pathspecs []string) ([]string, error) {
	args := append([]string{"ls-files", "--others", "--ignored", "--exclude-standard", "--"}, pathspecs...)
	out, err := r.RunInDir(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
