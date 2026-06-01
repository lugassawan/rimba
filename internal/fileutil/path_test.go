package fileutil_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/fileutil"
)

func TestContainedJoin(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple file", input: "file.txt"},
		{name: "nested file", input: "sub/file.txt"},
		{name: "net-inside traversal", input: "sub/../file.txt"},
		{name: "escape one level", input: "../x", wantErr: true},
		{name: "escape two levels", input: "../../x", wantErr: true},
		{name: "escape after descent", input: "a/../../x", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fileutil.ContainedJoin(base, tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ContainedJoin(%q, %q) = %q, nil; want error", base, tc.input, got)
				}
				if !errors.Is(err, fileutil.ErrPathEscapes) {
					t.Fatalf("error %v is not ErrPathEscapes", err)
				}
				if !strings.Contains(err.Error(), "contains ..") {
					t.Fatalf("error %q does not contain %q", err.Error(), "contains ..")
				}
				return
			}
			if err != nil {
				t.Fatalf("ContainedJoin(%q, %q) unexpected error: %v", base, tc.input, err)
			}
			if !strings.HasPrefix(got, base) {
				t.Fatalf("result %q is not inside base %q", got, base)
			}
			if got != filepath.Join(base, tc.input) {
				t.Fatalf("ContainedJoin(%q, %q) = %q; want %q", base, tc.input, got, filepath.Join(base, tc.input))
			}
		})
	}
}
