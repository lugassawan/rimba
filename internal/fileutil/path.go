package fileutil

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrPathEscapes is returned by ContainedJoin when a name escapes the base directory.
var ErrPathEscapes = errors.New("path escapes base directory")

// ContainedJoin joins name onto base and verifies the result stays within base.
// A name that nets back inside base (e.g. "sub/../file") is allowed; one that
// escapes via ".." is rejected. base must be a cleaned directory path.
func ContainedJoin(base, name string) (string, error) {
	base = filepath.Clean(base)
	joined := filepath.Join(base, name)
	if joined != base && !strings.HasPrefix(joined, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q contains ..: %w", name, ErrPathEscapes)
	}
	return joined, nil
}
