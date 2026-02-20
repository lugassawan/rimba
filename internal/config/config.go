package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// FileName is the legacy config file name used by rimba.
const FileName = ".rimba.toml"

// Directory-based config layout constants.
const (
	DirName   = ".rimba"
	TeamFile  = "settings.toml"
	LocalFile = "settings.local.toml"
)

type Config struct {
	WorktreeDir   string   `toml:"worktree_dir"`
	DefaultSource string   `toml:"default_source"`
	CopyFiles     []string `toml:"copy_files"`
	PostCreate    []string `toml:"post_create,omitempty"`

	Deps *DepsConfig       `toml:"deps,omitempty"`
	Open map[string]string `toml:"open,omitempty"`
}

// DepsConfig holds optional dependency management settings.
type DepsConfig struct {
	AutoDetect *bool          `toml:"auto_detect,omitempty"`
	Modules    []ModuleConfig `toml:"modules,omitempty"`
}

// ModuleConfig defines a manually configured dependency module.
type ModuleConfig struct {
	Dir      string `toml:"dir"`
	Lockfile string `toml:"lockfile"`
	Install  string `toml:"install"`
	WorkDir  string `toml:"work_dir,omitempty"`
}

// Validation error messages for required config fields.
const (
	ErrMsgEmptyWorktreeDir   = "worktree_dir must not be empty"
	ErrMsgEmptyDefaultSource = "default_source must not be empty"
)

// IsAutoDetectDeps returns whether automatic dependency detection is enabled.
// Defaults to true when Deps or AutoDetect is not configured.
func (c *Config) IsAutoDetectDeps() bool {
	if c.Deps == nil || c.Deps.AutoDetect == nil {
		return true
	}
	return *c.Deps.AutoDetect
}

// Validate checks that required config fields are present.
func (c *Config) Validate() error {
	var errs []error
	if c.WorktreeDir == "" {
		errs = append(errs, errors.New(ErrMsgEmptyWorktreeDir))
	}
	if c.DefaultSource == "" {
		errs = append(errs, errors.New(ErrMsgEmptyDefaultSource))
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid config: %w", errors.Join(errs...))
	}
	return nil
}

type ctxKey struct{}

func DefaultConfig(repoName, defaultBranch string) *Config {
	return &Config{
		WorktreeDir:   "../" + repoName + "-worktrees",
		DefaultSource: defaultBranch,
		CopyFiles:     []string{".env", ".env.local", ".envrc", ".tool-versions"},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config not found: %w (run 'rimba init' first)", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Merge combines a team config with a local override config.
// Scalars: local wins if non-zero. Slices/maps: local replaces team if non-nil.
func Merge(team, local *Config) *Config {
	if local == nil {
		return team
	}

	merged := *team // shallow copy

	if local.WorktreeDir != "" {
		merged.WorktreeDir = local.WorktreeDir
	}
	if local.DefaultSource != "" {
		merged.DefaultSource = local.DefaultSource
	}
	if local.CopyFiles != nil {
		merged.CopyFiles = local.CopyFiles
	}
	if local.PostCreate != nil {
		merged.PostCreate = local.PostCreate
	}
	if local.Deps != nil {
		merged.Deps = local.Deps
	}
	if local.Open != nil {
		merged.Open = local.Open
	}

	return &merged
}

// LoadDir loads the team config (required) and optional local override from
// a .rimba/ directory, merges them, and validates the result.
func LoadDir(dirPath string) (*Config, error) {
	team, err := loadRaw(filepath.Join(dirPath, TeamFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read team config: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("config not found: %s does not exist (run 'rimba init' first)", filepath.Join(dirPath, TeamFile))
	}

	local, err := loadRaw(filepath.Join(dirPath, LocalFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read local config: %w", err)
	}

	cfg := Merge(team, local)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Resolve loads config by checking for the .rimba/ directory first,
// then falling back to the legacy .rimba.toml file.
func Resolve(repoRoot string) (*Config, error) {
	dirPath := filepath.Join(repoRoot, DirName)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return LoadDir(dirPath)
	}

	return Load(filepath.Join(repoRoot, FileName))
}

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, ctxKey{}, cfg)
}

func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(ctxKey{}).(*Config)
	return cfg
}

// loadRaw reads a TOML config file without validation.
// Returns (nil, nil) if the file does not exist.
func loadRaw(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // nil,nil means "file absent, no error" — callers check for nil Config
		}
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", filepath.Base(path), err)
	}
	return &cfg, nil
}
