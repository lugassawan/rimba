package git

import "strings"

// DiffNameOnly returns files changed between base and branch using three-dot diff.
// The three-dot notation (base...branch) shows changes on branch since it diverged from base.
func DiffNameOnly(r Runner, base, branch string) ([]string, error) {
	out, err := r.Run("diff", "--name-only", base+"..."+branch)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// MergeTreeResult holds output of git merge-tree --write-tree.
type MergeTreeResult struct {
	HasConflicts  bool
	ConflictFiles []string
}

// MergeTree runs git merge-tree --write-tree to simulate a merge without
// touching the working tree. Requires git 2.38+.
// Exit code 0 = clean merge, exit code 1 = conflicts, other = error.
func MergeTree(r Runner, branch1, branch2 string) (MergeTreeResult, error) {
	out, err := r.Run("merge-tree", "--write-tree", branch1, branch2)
	if err != nil {
		// git merge-tree exits 1 for both conflicts and errors.
		// We distinguish by checking whether the output contains CONFLICT lines.
		result := parseMergeTreeOutput(out)
		if result.HasConflicts {
			return result, nil
		}
		return MergeTreeResult{}, err
	}
	return MergeTreeResult{}, nil
}

// parseMergeTreeOutput extracts conflict information from git merge-tree output.
// When conflicts exist, the output contains lines like:
//
//	CONFLICT (content): Merge conflict in <file>
func parseMergeTreeOutput(output string) MergeTreeResult {
	var result MergeTreeResult
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "CONFLICT") {
			result.HasConflicts = true
			if idx := strings.LastIndex(line, " in "); idx >= 0 {
				result.ConflictFiles = append(result.ConflictFiles, line[idx+4:])
			}
		}
	}
	return result
}
