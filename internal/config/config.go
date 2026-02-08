package config

import (
	"context"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	WorktreeDir   string   `toml:"worktree_dir"`
	DefaultSource string   `toml:"default_source"`
	CopyFiles     []string `toml:"copy_files"`
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
