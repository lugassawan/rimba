package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// FileName is the config file name used by rimba.
const FileName = ".rimba.toml"

type Config struct {
	WorktreeDir   string   `toml:"worktree_dir"`
	DefaultSource string   `toml:"default_source"`
	CopyFiles     []string `toml:"copy_files"`
	PostCreate    []string `toml:"post_create,omitempty"`

	Deps  *DepsConfig  `toml:"deps,omitempty"`
	Fleet *FleetConfig `toml:"fleet,omitempty"`
}

// FleetConfig holds settings for fleet (AI agent orchestration).
type FleetConfig struct {
	StateDir      string                 `toml:"state_dir,omitempty"`
	LogDir        string                 `toml:"log_dir,omitempty"`
	DefaultAgent  string                 `toml:"default_agent,omitempty"`
	MaxConcurrent int                    `toml:"max_concurrent,omitempty"`
	Agents        map[string]AgentConfig `toml:"agents,omitempty"`
}

// AgentConfig defines how to launch a specific AI agent.
type AgentConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args,omitempty"`
}

// FleetStateDir returns the fleet state directory, with a sensible default.
func (c *Config) FleetStateDir() string {
	if c.Fleet != nil && c.Fleet.StateDir != "" {
		return c.Fleet.StateDir
	}
	return ".rimba/fleet"
}

// FleetLogDir returns the fleet log directory, with a sensible default.
func (c *Config) FleetLogDir() string {
	if c.Fleet != nil && c.Fleet.LogDir != "" {
		return c.Fleet.LogDir
	}
	return ".rimba/fleet/logs"
}

// FleetDefaultAgent returns the default agent name.
func (c *Config) FleetDefaultAgent() string {
	if c.Fleet != nil && c.Fleet.DefaultAgent != "" {
		return c.Fleet.DefaultAgent
	}
	return "claude"
}

// FleetMaxConcurrent returns the max concurrent agents.
func (c *Config) FleetMaxConcurrent() int {
	if c.Fleet != nil && c.Fleet.MaxConcurrent > 0 {
		return c.Fleet.MaxConcurrent
	}
	return 4
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

// IsAutoDetectDeps returns whether automatic dependency detection is enabled.
// Defaults to true when Deps or AutoDetect is not configured.
func (c *Config) IsAutoDetectDeps() bool {
	if c.Deps == nil || c.Deps.AutoDetect == nil {
		return true
	}
	return *c.Deps.AutoDetect
}

// Validation error messages for required config fields.
const (
	ErrMsgEmptyWorktreeDir   = "worktree_dir must not be empty"
	ErrMsgEmptyDefaultSource = "default_source must not be empty"
)

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

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, ctxKey{}, cfg)
}

func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(ctxKey{}).(*Config)
	return cfg
}
