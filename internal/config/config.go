package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	TrustFile = "trust.local.toml"
	LocalGlob = "*.local.toml" // gitignore glob for personal *.local.toml overrides
)

// DefaultCommandTimeout is the subprocess deadline used when command_timeout is unset.
const DefaultCommandTimeout = 120 * time.Second

// ErrConfigAbsent means no config file exists — check via errors.Is to
// distinguish absence from a malformed or unreadable config.
var ErrConfigAbsent = errors.New("config not found")

type Config struct {
	WorktreeDir string `toml:"worktree_dir,omitempty"`
	// DefaultSource is internal-only: always derived from the repo's default
	// branch, never read from or written to TOML.
	DefaultSource  string   `toml:"-"`
	CommandTimeout string   `toml:"command_timeout,omitempty"`
	CopyFiles      []string `toml:"copy_files"`
	PostCreate     []string `toml:"post_create,omitempty"`
	PostRename     []string `toml:"post_rename,omitempty"`

	Deps          *DepsConfig          `toml:"deps,omitempty"`
	Open          map[string]string    `toml:"open,omitempty"`
	Resolver      *ResolverConfig      `toml:"resolver,omitempty"`
	Observability *ObservabilityConfig `toml:"observability,omitempty"`
}

// DefaultObservabilityRetentionDays is used when [observability] retention_days is unset.
const DefaultObservabilityRetentionDays = 14

// ObservabilityConfig holds optional observability settings.
type ObservabilityConfig struct {
	Enabled *bool `toml:"enabled,omitempty"`
	// RetentionDays is a pointer: nil means "unset" (default), 0 means
	// "explicitly disable pruning" — a plain int's zero value can't tell those apart.
	RetentionDays *int `toml:"retention_days,omitempty"`
}

// IsObservabilityEnabled reports whether the observability layer should record this
// invocation. RIMBA_NO_OBSERVABILITY (any value, checked via os.LookupEnv) forces it
// off regardless of config. Config [observability] enabled defaults to true when unset.
func (c *Config) IsObservabilityEnabled() bool {
	if _, off := os.LookupEnv("RIMBA_NO_OBSERVABILITY"); off {
		return false
	}
	if c.Observability == nil || c.Observability.Enabled == nil {
		return true
	}
	return *c.Observability.Enabled
}

// ObservabilityRetentionDays returns the configured retention window in days.
// <= 0 means "pruning disabled" (an explicit choice, never "delete everything").
// Unset (nil) returns DefaultObservabilityRetentionDays.
func (c *Config) ObservabilityRetentionDays() int {
	if c.Observability == nil || c.Observability.RetentionDays == nil {
		return DefaultObservabilityRetentionDays
	}
	return *c.Observability.RetentionDays
}

// EffectiveCommandTimeout returns the parsed CommandTimeout, or DefaultCommandTimeout
// when the field is empty, unparseable, or non-positive.
func (c *Config) EffectiveCommandTimeout() time.Duration {
	if c.CommandTimeout == "" {
		return DefaultCommandTimeout
	}
	d, err := time.ParseDuration(c.CommandTimeout)
	if err != nil || d <= 0 {
		return DefaultCommandTimeout
	}
	return d
}

// DepsConfig holds optional dependency management settings.
type DepsConfig struct {
	AutoDetect *bool          `toml:"auto_detect,omitempty"`
	Modules    []ModuleConfig `toml:"modules,omitempty"`
	// Concurrency caps parallel module installs. 0 = auto.
	Concurrency int `toml:"concurrency,omitempty"`
}

// ModuleConfig defines a manually configured dependency module, or (when
// Lockfile/Install are both left empty and Dir matches an auto-detected
// module) a "patch by Dir" entry that inherits everything else from detection.
type ModuleConfig struct {
	Dir      string `toml:"dir"`
	Lockfile string `toml:"lockfile,omitempty"`
	Install  string `toml:"install,omitempty"`
	WorkDir  string `toml:"work_dir,omitempty"`
	// Eager overrides the eager/lazy default for this module's Dir. nil
	// means unset (fall through to service-scope inference, then the
	// Recursive-flag heuristic).
	Eager *bool `toml:"eager,omitempty"`
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

// Validate returns a joined error of all invariant violations, or nil if valid.
func (c *Config) Validate() error {
	var errs []error
	errs = appendIf(errs, validateWorktreeDir(c.WorktreeDir)...)
	errs = appendIf(errs, validateCommandTimeout(c.CommandTimeout)...)
	errs = appendIf(errs, validateDeps(c.Deps)...)
	errs = appendIf(errs, validateOpen(c.Open)...)
	errs = appendIf(errs, validateResolver(c.Resolver)...)
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
		if os.IsNotExist(err) {
			return nil, errhint.WithFix(
				fmt.Errorf("%w: %s", ErrConfigAbsent, path),
				"run 'rimba init' to create a default .rimba/settings.toml",
			)
		}
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
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
	if local.CommandTimeout != "" {
		merged.CommandTimeout = local.CommandTimeout
	}
	if local.CopyFiles != nil {
		merged.CopyFiles = local.CopyFiles
	}
	if local.PostCreate != nil {
		merged.PostCreate = local.PostCreate
	}
	if local.PostRename != nil {
		merged.PostRename = local.PostRename
	}
	if local.Deps != nil {
		merged.Deps = local.Deps
	}
	if local.Open != nil {
		merged.Open = local.Open
	}
	if local.Resolver != nil {
		merged.Resolver = local.Resolver
	}
	if local.Observability != nil {
		merged.Observability = local.Observability
	}

	return &merged
}

// LoadDir loads the team config (required) and optional local override from a
// .rimba/ directory and merges them. Validation is the caller's responsibility.
func LoadDir(dirPath string) (*Config, error) {
	team, err := loadRaw(filepath.Join(dirPath, TeamFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read team config: %w", err)
	}
	if team == nil {
		return nil, errhint.WithFix(
			fmt.Errorf("%s does not exist: %w", filepath.Join(dirPath, TeamFile), ErrConfigAbsent),
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
		return []error{errhint.WithFix(
			fmt.Errorf("config: worktree_dir must be relative, got %q", dir),
			"set worktree_dir to a path relative to the repo root in .rimba/settings.toml",
		)}
	}
	return nil
}

// validateDeps checks that each module has a non-empty, unique Dir (it's the
// downstream map key) and that lockfile/install are set together per
// validateModuleLockfileInstall.
func validateDeps(deps *DepsConfig) []error {
	if deps == nil {
		return nil
	}
	autoDetect := deps.AutoDetect == nil || *deps.AutoDetect
	var errs []error
	seenDirs := make(map[string]bool, len(deps.Modules))
	for i, m := range deps.Modules {
		errs = append(errs, validateModuleDir(i, m.Dir, seenDirs)...)
		errs = append(errs, validateModuleLockfileInstall(m, autoDetect)...)
	}
	return errs
}

// validateModuleLockfileInstall requires Lockfile and Install to be set
// together (a full module definition), or both empty (a patch-by-Dir entry) —
// but only when auto_detect is on, since there's nothing to patch otherwise.
func validateModuleLockfileInstall(m ModuleConfig, autoDetect bool) []error {
	lockSet := strings.TrimSpace(m.Lockfile) != ""
	installSet := strings.TrimSpace(m.Install) != ""
	if lockSet != installSet {
		return []error{errhint.WithFix(
			fmt.Errorf("config: deps.modules[%q]: lockfile and install must be set together", m.Dir),
			"set both lockfile and install to define a new module, or remove both to patch an auto-detected module by dir in .rimba/settings.toml",
		)}
	}
	if !lockSet && !autoDetect {
		return []error{errhint.WithFix(
			fmt.Errorf("config: deps.modules[%q]: lockfile and install are required when deps.auto_detect is false", m.Dir),
			"set both lockfile and install, or enable deps.auto_detect to use this as a patch-by-dir entry",
		)}
	}
	return nil
}

// validateModuleDir returns an error if dir is empty or a duplicate, and
// records the dir in seen when it's valid.
func validateModuleDir(index int, dir string, seen map[string]bool) []error {
	switch {
	case strings.TrimSpace(dir) == "":
		return []error{errhint.WithFix(
			fmt.Errorf("config: deps.modules[%d]: dir is empty", index),
			"set dir = \"<path>\" for the module in .rimba/settings.toml",
		)}
	case seen[dir]:
		return []error{errhint.WithFix(
			fmt.Errorf("config: deps.modules[%d]: duplicate dir %q", index, dir),
			"remove the duplicate [[deps.modules]] entry from .rimba/settings.toml",
		)}
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
			errs = append(errs, errhint.WithFix(
				errors.New("config: open: shortcut name is empty"),
				"remove the empty-keyed entry under [open] in .rimba/settings.toml",
			))
			continue
		}
		if strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
			errs = append(errs, errhint.WithFix(
				fmt.Errorf("config: open[%q]: shortcut name must not contain path separators", name),
				"rename the shortcut in [open] to a name without '/' in .rimba/settings.toml",
			))
		}
	}
	return errs
}

// validateCommandTimeout rejects non-empty durations that are unparseable or non-positive.
func validateCommandTimeout(s string) []error {
	if s == "" {
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return []error{fmt.Errorf("command_timeout: invalid duration %q: %w", s, err)}
	}
	if d <= 0 {
		return []error{fmt.Errorf("command_timeout: duration must be positive, got %q", s)}
	}
	return nil
}
