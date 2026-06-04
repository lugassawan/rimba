package testutil

import (
	"os"
	"path/filepath"
	"runtime"
)

// fixtureT is the subset of *testing.T that LoadFixture needs, extracted so the
// failure path (t.Fatalf) can be exercised with a spy in tests.
type fixtureT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// LoadFixture reads a file at relpath relative to the calling test file's source directory.
func LoadFixture(t fixtureT, relpath string) string {
	t.Helper()
	_, callerFile, _, _ := runtime.Caller(1)
	path := filepath.Join(filepath.Dir(callerFile), relpath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LoadFixture %s: %v", relpath, err)
	}
	return string(data)
}
