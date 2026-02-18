package operations

import "errors"

const (
	branchMain         = "main"
	branchFeature      = "feature/login"
	branchBugfixTypo   = "bugfix/typo"
	pathWtFeatureLogin = "/wt/feature-login"
	errNotARepo        = "not a git repo"
)

var errGitFailed = errors.New("git failed")

// mockRunner implements git.Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	return m.runInDir(dir, args...)
}

// noopRunInDir is a default runInDir that returns empty output.
func noopRunInDir(_ string, _ ...string) (string, error) {
	return "", nil
}
