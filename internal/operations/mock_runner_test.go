package operations

import (
	"context"
	"errors"
)

const (
	branchMain            = "main"
	branchFeature         = "feature/login"
	branchBugfixTypo      = "bugfix/typo"
	pathWtFeatureLogin    = "/wt/feature-login"
	errNotARepo           = "not a git repo"
	gitCmdRevList         = "rev-list"
	pathMainRepo          = "/repo"
	refsRemotesOriginMain = "refs/remotes/origin/main"
)

var errGitFailed = errors.New("git failed")

// mockRunner implements git.Runner with configurable closures for testing.
type mockRunner struct {
	run      func(args ...string) (string, error)
	runInDir func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(_ context.Context, args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(_ context.Context, dir string, args ...string) (string, error) {
	return m.runInDir(dir, args...)
}

// noopRunInDir is a default runInDir that returns empty output.
func noopRunInDir(_ string, _ ...string) (string, error) {
	return "", nil
}

// ctxAwareMockRunner implements git.Runner, forwarding the context to run so
// tests can assert which context a given call actually received.
type ctxAwareMockRunner struct {
	run func(ctx context.Context, args ...string) (string, error)
}

func (m *ctxAwareMockRunner) Run(ctx context.Context, args ...string) (string, error) {
	return m.run(ctx, args...)
}

func (m *ctxAwareMockRunner) RunInDir(ctx context.Context, _ string, args ...string) (string, error) {
	return m.run(ctx, args...)
}
