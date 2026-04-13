package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/lugassawan/rimba/internal/errhint"
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
	WorktreeDir   string   `toml:"worktree_dir,omitempty"`
	DefaultSource string   `toml:"default_source,omitempty"`
	CopyFiles     []string `toml:"copy_files"`
	PostCreate    []string `toml:"post_create,omitempty"`

	Deps *DepsConfig       `toml:"deps,omitempty"`
	Open map[string]string `toml:"open,omitempty"`
}

// DepsConfig holds optional dependency management settings.
type DepsConfig struct {
	AutoDetect *bool          `toml:"auto_detect,omitempty"`
	Modules    []ModuleConfig `toml:"modules,omitempty"`
	// Concurrency caps parallel module installs. 0 = auto.
	Concurrency int `toml:"concurrency,omitempty"`
}

// ModuleConfig defines a manually configured dependency module.
type ModuleConfig struct {
	Dir      string `toml:"dir"`
	Lockfile string `toml:"lockfile"`
	Install  string `toml:"install"`
	WorkDir  string `toml:"work_dir,omitempty"`
}

// IsAutoDetectDeps returns whether automatic dependency detection is enabled.
// Defaults to true when Deps or AutoDetect is not configured.
func (c *Config) IsAutoDetectDeps() bool {
	if c.Deps == nil || c.Deps.AutoDetect == nil {
		return true
	}
	return *c.Deps.AutoDetect
}

// DepsConcurrency returns the configured install concurrency, or 0 if unset.
func (c *Config) DepsConcurrency() int {
	if c.Deps == nil {
		return 0
	}
	return c.Deps.Concurrency
}

// DefaultWorktreeDir returns the conventional worktree directory path for a repo.
func DefaultWorktreeDir(repoName string) string {
	return "../" + repoName + "-worktrees"
}

// DefaultCopyFiles returns the default list of files copied to new worktrees.
func DefaultCopyFiles() []string {
	return []string{".env", ".env.local", ".envrc", ".tool-versions"}
}

// FillDefaults fills missing config fields with auto-derived values.
// repoName is used for WorktreeDir; defaultBranch is used for DefaultSource.
func (c *Config) FillDefaults(repoName, defaultBranch string) {
	if c.WorktreeDir == "" {
		c.WorktreeDir = DefaultWorktreeDir(repoName)
	}
	if c.DefaultSource == "" {
		c.DefaultSource = defaultBranch
	}
	if c.CopyFiles == nil {
		c.CopyFiles = DefaultCopyFiles()
	}
}

// Validate checks invariants on the loaded config and returns a joined error
// describing all issues at once, or nil if the config is valid. It performs no
// I/O and is safe to call after Resolve and FillDefaults. Each issue names the
// offending field or section so users can fix them in one pass.
func (c *Config) Validate() error {
	var errs []error
	errs = appendIf(errs, validateWorktreeDir(c.WorktreeDir)...)
	errs = appendIf(errs, validateDeps(c.Deps)...)
	errs = appendIf(errs, validateOpen(c.Open)...)
	return errors.Join(errs...)
}

type ctxKey struct{}

func DefaultConfig(repoName, defaultBranch string) *Config {
	cfg := &Config{}
	cfg.FillDefaults(repoName, defaultBranch)
	return cfg
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errhint.WithFix(
			fmt.Errorf("config not found: %w", err),
			"run 'rimba init' to create a default .rimba/settings.toml",
		)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, errhint.WithFix(
			fmt.Errorf("invalid config: %w", err),
			"fix the TOML syntax in "+path,
		)
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
		return nil, errhint.WithFix(
			fmt.Errorf("config not found: %s does not exist", filepath.Join(dirPath, TeamFile)),
			"run 'rimba init' to create a default .rimba/settings.toml",
		)
	}

	local, err := loadRaw(filepath.Join(dirPath, LocalFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read local config: %w", err)
	}

	return Merge(team, local), nil
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
		return nil, errhint.WithFix(
			fmt.Errorf("invalid config %s: %w", filepath.Base(path), err),
			"fix the TOML syntax in "+path,
		)
	}
	return &cfg, nil
}

// appendIf appends only non-nil errors. Keeps Validate concise.
func appendIf(dst []error, src ...error) []error {
	for _, e := range src {
		if e != nil {
			dst = append(dst, e)
		}
	}
	return dst
}

// validateWorktreeDir enforces that WorktreeDir is relative. Absolute paths
// break resolution because worktree paths are joined against the repo root.
func validateWorktreeDir(dir string) []error {
	if dir != "" && filepath.IsAbs(dir) {
		return []error{fmt.Errorf("config: worktree_dir must be relative, got %q", dir)}
	}
	return nil
}

// validateDeps checks that each module has a non-empty, unique Dir and a
// non-empty Install command. Empty Dir collides in the downstream map keyed
// by Dir; empty Install is a silent no-op at install time.
func validateDeps(deps *DepsConfig) []error {
	if deps == nil {
		return nil
	}
	var errs []error
	seenDirs := make(map[string]bool, len(deps.Modules))
	for i, m := range deps.Modules {
		errs = append(errs, validateModuleDir(i, m.Dir, seenDirs)...)
		if strings.TrimSpace(m.Install) == "" {
			errs = append(errs, fmt.Errorf("config: deps.modules[%q]: install command is empty", m.Dir))
		}
	}
	return errs
}

// validateModuleDir returns an error if dir is empty or a duplicate, and
// records the dir in seen when it's valid.
func validateModuleDir(index int, dir string, seen map[string]bool) []error {
	switch {
	case strings.TrimSpace(dir) == "":
		return []error{fmt.Errorf("config: deps.modules[%d]: dir is empty", index)}
	case seen[dir]:
		return []error{fmt.Errorf("config: deps.modules[%d]: duplicate dir %q", index, dir)}
	default:
		seen[dir] = true
		return nil
	}
}

// validateOpen rejects shortcut names that are empty or contain path
// separators (either `/` or the OS-specific separator) to avoid collisions
// with file-path resolution in `rimba open`.
func validateOpen(open map[string]string) []error {
	var errs []error
	for name := range open {
		if name == "" {
			errs = append(errs, errors.New("config: open: shortcut name is empty"))
			continue
		}
		if strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
			errs = append(errs, fmt.Errorf("config: open[%q]: shortcut name must not contain path separators", name))
		}
	}
	return errs
}
