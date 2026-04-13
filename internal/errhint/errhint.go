// Package errhint wraps errors with actionable "To fix:" hints.
package errhint

import "fmt"

// WithFix wraps err with a "To fix:" hint. Returns nil if err is nil.
func WithFix(err error, fix string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w\nTo fix: %s", err, fix)
}
