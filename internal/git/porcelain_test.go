package git

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyPorcelainDeletions(t *testing.T) {
	tests := []struct {
		name        string
		out         string
		err         error
		wantDeleted int
		wantOther   int
		wantAll     bool
		wantErr     bool
	}{
		{
			name:        "all_unstaged_deletions",
			out:         " D file1.txt\n D file2.txt\n D file3.txt\n",
			wantDeleted: 3,
			wantOther:   0,
			wantAll:     true,
		},
		{
			name:        "mixed_deletion_and_modification",
			out:         " D deleted.txt\n M modified.txt\n",
			wantDeleted: 1,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name:        "staged_deletion",
			out:         "D  staged-deletion.txt\n",
			wantDeleted: 0,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name:        "untracked_file",
			out:         "?? untracked.txt\n",
			wantDeleted: 0,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name:        "conflict",
			out:         "DU conflict.txt\n",
			wantDeleted: 0,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name:        "empty_status",
			out:         "",
			wantDeleted: 0,
			wantOther:   0,
			wantAll:     false,
		},
		{
			name:    "run_in_dir_error_propagated",
			err:     errors.New(errNotARepo),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				runInDir: func(_ string, _ ...string) (string, error) {
					return tt.out, tt.err
				},
			}

			status, err := ClassifyPorcelainDeletions(context.Background(), r, fakeDir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error from RunInDir failure")
				}
				assertContains(t, err, errNotARepo)
				return
			}
			if err != nil {
				t.Fatalf("ClassifyPorcelainDeletions: %v", err)
			}
			if status.Deleted != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", status.Deleted, tt.wantDeleted)
			}
			if status.Other != tt.wantOther {
				t.Errorf("Other = %d, want %d", status.Other, tt.wantOther)
			}
			if status.AllDeletions() != tt.wantAll {
				t.Errorf("AllDeletions() = %v, want %v", status.AllDeletions(), tt.wantAll)
			}
		})
	}
}
