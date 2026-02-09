package deps

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// ModuleWithHash pairs a Module with its lockfile hash.
type ModuleWithHash struct {
	Module Module
	Hash   string
}

// HashLockfile computes the SHA-256 hex digest of a lockfile.
// Returns an empty string if the file doesn't exist.
func HashLockfile(worktreePath, lockfile string) (string, error) {
	data, err := os.ReadFile(filepath.Join(worktreePath, lockfile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// HashModules computes lockfile hashes for all given modules.
func HashModules(worktreePath string, modules []Module) ([]ModuleWithHash, error) {
	result := make([]ModuleWithHash, 0, len(modules))
	for _, m := range modules {
		h, err := HashLockfile(worktreePath, m.Lockfile)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", m.Lockfile, err)
		}
		result = append(result, ModuleWithHash{Module: m, Hash: h})
	}
	return result, nil
}
