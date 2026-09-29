package git

import (
	"context"
	"strings"
)

// reflogSubjectMark prefixes each line so empty subjects survive the runner's output trimming.
const reflogSubjectMark = "@"

// BranchReflogSubjects returns branch's reflog subjects, newest first; empty if it has no reflog.
// Entries logged without a message yield an empty subject.
func BranchReflogSubjects(ctx context.Context, r Runner, branch string) ([]string, error) {
	out, err := r.Run(ctx, CmdLog, "-g", "--format="+reflogSubjectMark+"%gs", flagEndOfOptions, refsHeadsPrefix+branch)
	if err != nil {
		return nil, err
	}

	var subjects []string
	for line := range strings.SplitSeq(out, "\n") {
		if s, ok := strings.CutPrefix(strings.TrimSpace(line), reflogSubjectMark); ok {
			subjects = append(subjects, s)
		}
	}
	return subjects, nil
}
