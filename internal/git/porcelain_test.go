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
			name: "all_unstaged_deletions",
			out: "1 .D N... 100644 100644 000000 " +
				"aaaa1111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaa2222aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa file1.txt\n" +
				"1 .D N... 100644 100644 000000 " +
				"bbbb1111bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb bbbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb file2.txt\n" +
				"1 .D N... 100644 100644 000000 " +
				"cccc1111cccccccccccccccccccccccccccccccc cccc2222cccccccccccccccccccccccccccccccc file3.txt\n",
			wantDeleted: 3,
			wantOther:   0,
			wantAll:     true,
		},
		{
			name: "mixed_deletion_and_modification",
			out: "1 .D N... 100644 100644 000000 " +
				"aaaa1111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaa2222aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa deleted.txt\n" +
				"1 .M N... 100644 100644 100644 " +
				"bbbb1111bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb bbbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb modified.txt\n",
			wantDeleted: 1,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name: "staged_deletion",
			out: "1 D. N... 100644 000000 000000 " +
				"aaaa1111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 0000000000000000000000000000000000000000 staged-deletion.txt\n",
			wantDeleted: 0,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name:        "untracked_file",
			out:         "? untracked.txt\n",
			wantDeleted: 0,
			wantOther:   1,
			wantAll:     false,
		},
		{
			name: "conflict",
			out: "u UU N... 100644 100644 100644 100644 " +
				"aaaa1111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa bbbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb " +
				"cccc3333cccccccccccccccccccccccccccccccc conflict.txt\n",
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
			// Guards the real bug this format switch fixed: RunInDir's shared
			// strings.TrimSpace strips a leading space off the very first line
			// of raw output. v1's " D file.txt" would corrupt to "D file.txt"
			// and silently misclassify as "Other" — v2's "1 .D ..." starts
			// with a non-space char, so front-trimming is a no-op here.
			name: "single_deletion_front_trimmed_like_real_runner",
			out: "1 .D N... 100644 100644 000000 " +
				"aaaa1111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa aaaa2222aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa solo.txt",
			wantDeleted: 1,
			wantOther:   0,
			wantAll:     true,
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
