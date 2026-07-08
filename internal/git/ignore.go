package git

import (
	"context"
	"strings"
)

// ListIgnoredUntracked lists untracked files git ignores under dir, scoped
// to pathspecs. An empty pathspecs returns no paths, not every ignored file.
func ListIgnoredUntracked(ctx context.Context, r Runner, dir string, pathspecs []string) ([]string, error) {
	if len(pathspecs) == 0 {
		return nil, nil
	}
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
