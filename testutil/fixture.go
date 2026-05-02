package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// LoadFixture reads a file at relpath relative to the calling test file's source directory.
func LoadFixture(t *testing.T, relpath string) string {
	t.Helper()
	_, callerFile, _, _ := runtime.Caller(1)
	path := filepath.Join(filepath.Dir(callerFile), relpath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LoadFixture %s: %v", relpath, err)
	}
	return string(data)
}
