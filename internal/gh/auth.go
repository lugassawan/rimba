package gh

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/errhint"
)

// CheckAuth verifies that `gh` is installed and authenticated.
// Returns nil if `gh auth status` succeeds; otherwise returns an
// errhint.WithFix-wrapped error with guidance for the user.
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
