package trust

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/fileutil"
)

const (
	// FileName is the gitignored local trust record for a repo.
	FileName     = "trust.local.toml"
	storeVersion = 1
)

// Store is the content of the per-repo trust record.
type Store struct {
	Version    int    `toml:"version"`
	Hash       string `toml:"hash"`
	ApprovedAt string `toml:"approved_at"`
}

// Load reads the trust store from repoRoot. Returns (nil, nil) if absent.
func Load(repoRoot string) (*Store, error) {
	data, err := os.ReadFile(storePath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // nil,nil means "absent, no error" — matches config.loadRaw convention
		}
		return nil, fmt.Errorf("read trust store: %w", err)
	}
	var s Store
	if err := toml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse trust store: %w", err)
	}
	if s.Version > storeVersion {
		return nil, fmt.Errorf("trust store version %d is newer than supported (%d); upgrade rimba", s.Version, storeVersion)
	}
	return &s, nil
}

// IsTrusted reports whether the stored hash matches the given hash.
// Returns false (not error) when the trust file is absent or the hash differs.
func IsTrusted(repoRoot, hash string) (bool, error) {
	s, err := Load(repoRoot)
	if err != nil {
		return false, err
	}
	return s != nil && s.Hash == hash, nil
}

// Record persists approval of the given command-set hash for repoRoot.
// It ensures .rimba/ exists and registers trust.local.toml in .gitignore.
func Record(repoRoot, hash string) error {
	dir := filepath.Join(repoRoot, config.DirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create .rimba dir: %w", err)
	}

	// Best-effort gitignore registration — self-heals repos initialized
	// before this feature existed.
	entry := filepath.Join(config.DirName, FileName)
	if _, err := fileutil.EnsureGitignore(repoRoot, entry); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	s := Store{
		Version:    storeVersion,
		Hash:       hash,
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := toml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal trust store: %w", err)
	}
	if err := os.WriteFile(storePath(repoRoot), data, 0600); err != nil {
		return fmt.Errorf("write trust store: %w", err)
	}
	return nil
}

// GateNonInteractive is the non-interactive consent gate used by MCP handlers.
// It returns nil when there are no committed commands, the repo is already
// trusted, or RIMBA_TRUST_YES is set to a truthy value. Otherwise it returns
// an error with a remediation hint pointing to `rimba trust`.
func GateNonInteractive(repoRoot string, cfg *config.Config) error {
	if !HasCommands(cfg) {
		return nil
	}
	h := Hash(cfg)
	ok, err := IsTrusted(repoRoot, h)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if TrustYesFromEnv() {
		return Record(repoRoot, h)
	}
	return errhint.WithFix(
		errors.New("committed shell commands are not trusted for this repo"),
		"run 'rimba trust' to approve them, or set RIMBA_TRUST_YES=1 for CI",
	)
}

// TrustYesFromEnv reports whether RIMBA_TRUST_YES is set to a truthy value.
// It is the canonical definition of truthy for this environment variable;
// cmd/trust_gate.go delegates to it to avoid duplicate semantics.
func TrustYesFromEnv() bool {
	v, ok := os.LookupEnv("RIMBA_TRUST_YES")
	return ok && v != "" && v != "0"
}

func storePath(repoRoot string) string {
	return filepath.Join(repoRoot, config.DirName, FileName)
}
