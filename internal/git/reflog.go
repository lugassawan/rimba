package git

import (
	"context"
	"strings"
)

// BranchReflogSubjects returns branch's reflog subjects, newest first; empty if it has no reflog.
func BranchReflogSubjects(ctx context.Context, r Runner, branch string) ([]string, error) {
	out, err := r.Run(ctx, CmdLog, "-g", "--format=%gs", flagEndOfOptions, refsHeadsPrefix+branch)
	if err != nil {
		return nil, err
	}

	var subjects []string
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects, nil
}
