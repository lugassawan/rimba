package git

import (
	"errors"
	"testing"
)

const (
	testBranchA = "feature/a"
	testBranchB = "feature/b"
)

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("[%d] = %q, want %q", i, g, want[i])
		}
	}
}

func TestDiffNameOnly(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		err     error
		want    []string
		wantErr bool
	}{
		{
			name: "two files",
			out:  "file1.go\nfile2.go\n",
			want: []string{"file1.go", "file2.go"},
		},
		{
			name: "empty output",
			out:  "",
			want: nil,
		},
		{
			name: "whitespace only",
			out:  "   \n\n  ",
			want: nil,
		},
		{
			name:    "error",
			err:     errors.New("git failed"),
			wantErr: true,
		},
		{
			name: "single file",
			out:  "main.go\n",
			want: []string{"main.go"},
		},
		{
			name: "trailing whitespace trimmed",
			out:  "  a.go  \n  b.go  \n",
			want: []string{"a.go", "b.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ ...string) (string, error) {
					return tt.out, tt.err
				},
			}

			files, err := DiffNameOnly(r, "main", testBranchA)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertStringSlice(t, files, tt.want)
		})
	}
}

func TestDiffNameOnlyVerifiesRef(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	_, _ = DiffNameOnly(r, "main", "feature/x")
	// Should use three-dot notation.
	if len(captured) < 3 {
		t.Fatalf("expected at least 3 args, got %v", captured)
	}
	ref := captured[2]
	if ref != "main...feature/x" {
		t.Errorf("ref = %q, want %q", ref, "main...feature/x")
	}
}

func TestMergeTree(t *testing.T) {
	tests := []struct {
		name         string
		out          string
		err          error
		wantConflict bool
		wantErr      bool
	}{
		{
			name:         "no conflict",
			out:          "abc123",
			wantConflict: false,
		},
		{
			name:         "conflict detected",
			out:          "CONFLICT (content): Merge conflict in file.go",
			err:          errors.New("exit status 1"),
			wantConflict: true,
		},
		{
			name:    "error without conflict",
			out:     "some other error",
			err:     errors.New("git failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ ...string) (string, error) {
					return tt.out, tt.err
				},
			}

			hasConflict, err := MergeTree(r, "main", testBranchA, testBranchB)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hasConflict != tt.wantConflict {
				t.Errorf("hasConflict = %v, want %v", hasConflict, tt.wantConflict)
			}
		})
	}
}

func TestMergeTreeVerifiesArgs(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	_, _ = MergeTree(r, "main", testBranchA, testBranchB)
	if len(captured) < 4 {
		t.Fatalf("expected at least 4 args, got %v", captured)
	}
	if captured[0] != "merge-tree" {
		t.Errorf("arg[0] = %q, want %q", captured[0], "merge-tree")
	}
	if captured[1] != "--write-tree" {
		t.Errorf("arg[1] = %q, want %q", captured[1], "--write-tree")
	}
	if captured[2] != "--merge-base=main" {
		t.Errorf("arg[2] = %q, want %q", captured[2], "--merge-base=main")
	}
}
