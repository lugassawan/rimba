package gitref

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrUnsafeRefName is the sentinel returned by Validate for unsafe ref names.
var ErrUnsafeRefName = errors.New("unsafe git ref name")

// refNameRe matches ref names that are safe to use as git branch/source arguments.
var refNameRe = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

// Validate returns ErrUnsafeRefName (wrapped) if name is unsafe for use as a
// git branch name or source argument. An empty name is not validated here —
// callers that treat empty as "use default" should check before calling.
func Validate(name string) error {
	switch {
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("%w %q (leading dash)", ErrUnsafeRefName, name)
	case strings.Contains(name, ".."):
		return fmt.Errorf("%w %q (contains ..)", ErrUnsafeRefName, name)
	case strings.HasPrefix(name, "/"):
		return fmt.Errorf("%w %q (leading slash)", ErrUnsafeRefName, name)
	case !refNameRe.MatchString(name):
		return fmt.Errorf("%w %q", ErrUnsafeRefName, name)
	}
	return nil
}
