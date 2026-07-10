package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lugassawan/rimba/testutil"
)

func TestDiffNameOnly(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		want    []string
		wantErr bool
	}{
		{
			name: "two files changed",
			out:  "file1.go\nfile2.go",
			want: []string{"file1.go", "file2.go"},
		},
		{
			name: "single file changed",
			out:  "main.go",
			want: []string{"main.go"},
		},
		{
			name: "no changes",
			out:  "",
			want: nil,
		},
		{
			name:    "git error",
			err:     errors.New("git diff failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured []string
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					captured = args
					return tt.out, tt.err
				},
			}

			got, err := DiffNameOnly(context.Background(), r, "main", "feature/x")
			if (err != nil) != tt.wantErr {
				t.Fatalf("DiffNameOnly error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				want := []string{CmdDiff, "--name-only", flagEndOfOptions, "main...feature/x"}
				if !slices.Equal(captured, want) {
					t.Errorf("args = %v, want %v", captured, want)
				}
				if len(got) != len(tt.want) {
					t.Fatalf("got %d files, want %d", len(got), len(tt.want))
				}
				for i, w := range tt.want {
					if got[i] != w {
						t.Errorf("files[%d] = %q, want %q", i, got[i], w)
					}
				}
			}
		})
	}
}

func TestDiffNameOnlyLeadingDashBranchName(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegrationGit)
	}

	repo := testutil.NewTestRepo(t)
	r := &ExecRunner{Dir: repo}

	// -diff-dash must be base, not branch, so the dash leads the combined "base...branch" arg.
	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-diff-dash", "HEAD")
	testutil.CreateFile(t, repo, "extra.txt", "content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "extra commit")

	files, err := DiffNameOnly(context.Background(), r, "-diff-dash", "main")
	if err != nil {
		t.Fatalf("DiffNameOnly with leading-dash branch: %v", err)
	}
	if !slices.Contains(files, "extra.txt") {
		t.Errorf("expected extra.txt in diff, got %v", files)
	}
}

func TestMergeTreeClean(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "abc123def456", nil // tree hash on success
		},
	}

	result, err := MergeTree(context.Background(), r, "main", "feature/x")
	if err != nil {
		t.Fatalf("MergeTree: %v", err)
	}
	if result.HasConflicts {
		t.Error("expected clean merge")
	}
}

func TestMergeTreeConflicts(t *testing.T) {
	conflictOutput := "abc123def456\nCONFLICT (content): Merge conflict in file1.go\nCONFLICT (content): Merge conflict in file2.go"
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return conflictOutput, errors.New("exit status 1")
		},
	}

	result, err := MergeTree(context.Background(), r, "main", "feature/x")
	if err != nil {
		t.Fatalf("MergeTree: %v", err)
	}
	if !result.HasConflicts {
		t.Fatal("expected conflicts")
	}
	if len(result.ConflictFiles) != 2 {
		t.Fatalf("got %d conflict files, want 2", len(result.ConflictFiles))
	}
	if result.ConflictFiles[0] != "file1.go" {
		t.Errorf("ConflictFiles[0] = %q, want %q", result.ConflictFiles[0], "file1.go")
	}
	if result.ConflictFiles[1] != "file2.go" {
		t.Errorf("ConflictFiles[1] = %q, want %q", result.ConflictFiles[1], "file2.go")
	}
}

func TestMergeTreeError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New("git merge-tree: not a valid object")
		},
	}

	_, err := MergeTree(context.Background(), r, "main", "nonexistent")
	if err == nil {
		t.Fatal("expected error from MergeTree")
	}
}

func TestMergeTreeLeadingDashBranchName(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegrationGit)
	}

	repo := testutil.NewTestRepo(t)
	r := &ExecRunner{Dir: repo}

	testutil.GitCmd(t, repo, "update-ref", "refs/heads/-merge-tree-dash", "HEAD")

	result, err := MergeTree(context.Background(), r, "main", "-merge-tree-dash")
	if err != nil {
		t.Fatalf("MergeTree with leading-dash branch: %v", err)
	}
	if result.HasConflicts {
		t.Error("expected clean merge against identical history")
	}
}

func TestMergeTreeArgs(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	_, _ = MergeTree(context.Background(), r, "main", "feature/x")
	want := []string{"merge-tree", "--write-tree", "--", "main", "feature/x"}
	if !slices.Equal(captured, want) {
		t.Errorf("args = %v, want %v", captured, want)
	}
}

func TestParseMergeTreeOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantConfl bool
		wantFiles []string
	}{
		{
			name:      "no conflicts",
			output:    "abc123def456",
			wantConfl: false,
		},
		{
			name:      "single conflict",
			output:    "abc123\nCONFLICT (content): Merge conflict in main.go",
			wantConfl: true,
			wantFiles: []string{"main.go"},
		},
		{
			name:      "multiple conflicts",
			output:    "abc123\nCONFLICT (content): Merge conflict in a.go\nCONFLICT (add/add): Merge conflict in b.go",
			wantConfl: true,
			wantFiles: []string{"a.go", "b.go"},
		},
		{
			name:      "empty output",
			output:    "",
			wantConfl: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMergeTreeOutput(tt.output)
			if result.HasConflicts != tt.wantConfl {
				t.Errorf("HasConflicts = %v, want %v", result.HasConflicts, tt.wantConfl)
			}
			if len(result.ConflictFiles) != len(tt.wantFiles) {
				t.Fatalf("got %d conflict files, want %d", len(result.ConflictFiles), len(tt.wantFiles))
			}
			for i, w := range tt.wantFiles {
				if result.ConflictFiles[i] != w {
					t.Errorf("ConflictFiles[%d] = %q, want %q", i, result.ConflictFiles[i], w)
				}
			}
		})
	}
}
