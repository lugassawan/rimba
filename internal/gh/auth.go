package gh

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/errhint"
)

// CheckAuth returns nil if `gh` is installed and authenticated.
// Otherwise it returns a wrapped error with a "To fix:" hint.
func CheckAuth(ctx context.Context, runner Runner) error {
	if !IsAvailable() {
		return errhint.WithFix(
			errors.New("gh CLI not found on PATH"),
			"install from https://cli.github.com and run: gh auth login",
		)
	}
	if _, err := runner.Run(ctx, "auth", "status"); err != nil {
		return errhint.WithFix(
			errors.New("gh is not authenticated"),
			"run: gh auth login",
		)
	}
	return nil
}
