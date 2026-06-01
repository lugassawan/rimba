package fileutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ContainedJoin joins name onto base and verifies the result stays within base.
// A name that nets back inside base (e.g. "sub/../file") is allowed; one that
// escapes via ".." is rejected. base must be a cleaned directory path.
func ContainedJoin(base, name string) (string, error) {
	base = filepath.Clean(base)
	joined := filepath.Join(base, name)
	if joined != base && !strings.HasPrefix(joined, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q contains .. and escapes the base directory", name)
	}
	return joined, nil
}
