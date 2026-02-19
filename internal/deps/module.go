package deps

import (
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
)

// Lockfile and directory constants used for ecosystem detection.
const (
	LockfilePnpm = "pnpm-lock.yaml"
	LockfileYarn = "yarn.lock"
	LockfileNpm  = "package-lock.json"
	LockfileGo   = "go.sum"

	DirNodeModules = "node_modules"
	DirVendor      = "vendor"
	DirYarnCache   = ".yarn/cache"
)

// Module represents a detected or configured dependency module.
type Module struct {
	Dir        string   `json:"dir"`                   // Primary dir: "node_modules" or "service-api/vendor"
	Lockfile   string   `json:"lockfile"`              // "pnpm-lock.yaml" or "service-api/go.sum"
	InstallCmd string   `json:"install_cmd,omitempty"` // "pnpm install --frozen-lockfile" or "go mod vendor"
	WorkDir    string   `json:"work_dir,omitempty"`    // Subdir to run install in: "" (root) or "service-api"
	Recursive  bool     `json:"recursive,omitempty"`   // If true, clone ALL dirs named Dir found recursively (monorepo)
	ExtraDirs  []string `json:"extra_dirs,omitempty"`  // Additional dirs to clone (e.g., ".yarn/cache")
	CloneOnly  bool     `json:"clone_only,omitempty"`  // If true, only clone (don't run install if no match). For Go vendor.
}

type preset struct {
	Lockfile   string
	Dir        string
	InstallCmd string
	Recursive  bool
	ExtraDirs  []string
	CloneOnly  bool
}

// presets defines built-in ecosystem detection rules, ordered by priority.
// For the same Dir, the first matching lockfile wins (pnpm > yarn > npm).
var presets = []preset{
	{
		Lockfile:   LockfilePnpm,
		Dir:        DirNodeModules,
		InstallCmd: "pnpm install --frozen-lockfile",
		Recursive:  true,
	},
	{
		Lockfile:   LockfileYarn,
		Dir:        DirNodeModules,
		InstallCmd: "yarn install",
		Recursive:  true,
		ExtraDirs:  []string{DirYarnCache},
	},
	{
		Lockfile:   LockfileNpm,
		Dir:        DirNodeModules,
		InstallCmd: "npm ci",
		Recursive:  true,
	},
	{
		Lockfile:   LockfileGo,
		Dir:        DirVendor,
		InstallCmd: "go mod vendor",
		CloneOnly:  true,
	},
}

// DetectModules scans a worktree path for known lockfiles and returns matching modules.
// It scans the root directory first, then depth-1 subdirectories.
func DetectModules(worktreePath string) ([]Module, error) {
	var modules []Module
	seenDirs := make(map[string]bool)

	// Phase 1: Scan root for lockfiles
	modules = detectRootModules(worktreePath, modules, seenDirs)

	// Phase 2: Scan depth-1 subdirectories for lockfiles
	modules = detectSubdirModules(worktreePath, modules, seenDirs)

	return modules, nil
}

func detectRootModules(worktreePath string, modules []Module, seenDirs map[string]bool) []Module {
	for _, p := range presets {
		if seenDirs[p.Dir] {
			continue
		}
		lockPath := filepath.Join(worktreePath, p.Lockfile)
		if _, err := os.Stat(lockPath); err == nil {
			modules = append(modules, moduleFromPreset(p, "", ""))
			seenDirs[p.Dir] = true
		}
	}
	return modules
}

func detectSubdirModules(worktreePath string, modules []Module, seenDirs map[string]bool) []Module {
	entries, err := os.ReadDir(worktreePath)
	if err != nil {
		return modules
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		subdir := entry.Name()
		modules = matchPresetsInSubdir(worktreePath, subdir, modules, seenDirs)
	}
	return modules
}

func matchPresetsInSubdir(worktreePath, subdir string, modules []Module, seenDirs map[string]bool) []Module {
	for _, p := range presets {
		depDir := filepath.Join(subdir, p.Dir)
		if seenDirs[depDir] {
			continue
		}
		lockPath := filepath.Join(worktreePath, subdir, p.Lockfile)
		if _, err := os.Stat(lockPath); err == nil {
			modules = append(modules, moduleFromPreset(p, subdir, depDir))
			seenDirs[depDir] = true
		}
	}
	return modules
}

func moduleFromPreset(p preset, subdir, depDir string) Module {
	if subdir == "" {
		return Module{
			Dir:        p.Dir,
			Lockfile:   p.Lockfile,
			InstallCmd: p.InstallCmd,
			Recursive:  p.Recursive,
			ExtraDirs:  p.ExtraDirs,
			CloneOnly:  p.CloneOnly,
		}
	}
	return Module{
		Dir:        depDir,
		Lockfile:   filepath.Join(subdir, p.Lockfile),
		InstallCmd: p.InstallCmd,
		WorkDir:    subdir,
		Recursive:  p.Recursive,
		ExtraDirs:  prefixDirs(subdir, p.ExtraDirs),
		CloneOnly:  p.CloneOnly,
	}
}

// FilterCloneOnly removes CloneOnly modules whose dep dir doesn't exist in any worktree.
func FilterCloneOnly(modules []Module, worktreePaths []string) []Module {
	var result []Module
	for _, m := range modules {
		if !m.CloneOnly {
			result = append(result, m)
			continue
		}

		for _, wtPath := range worktreePaths {
			if info, err := os.Stat(filepath.Join(wtPath, m.Dir)); err == nil && info.IsDir() {
				result = append(result, m)
				break
			}
		}
	}
	return result
}

// MergeWithConfig merges auto-detected modules with user-configured modules.
// Config modules override auto-detected ones for the same Dir.
func MergeWithConfig(detected []Module, configModules []config.ModuleConfig) []Module {
	if len(configModules) == 0 {
		return detected
	}

	configDirs := make(map[string]config.ModuleConfig)
	for _, cm := range configModules {
		configDirs[cm.Dir] = cm
	}

	var result []Module
	seenDirs := make(map[string]bool)
	for _, m := range detected {
		if cm, ok := configDirs[m.Dir]; ok {
			result = append(result, moduleFromConfig(cm))
		} else {
			result = append(result, m)
		}
		seenDirs[m.Dir] = true
	}

	for _, cm := range configModules {
		if !seenDirs[cm.Dir] {
			result = append(result, moduleFromConfig(cm))
		}
	}

	return result
}

func moduleFromConfig(cm config.ModuleConfig) Module {
	return Module{
		Dir:        cm.Dir,
		Lockfile:   cm.Lockfile,
		InstallCmd: cm.Install,
		WorkDir:    cm.WorkDir,
	}
}

func prefixDirs(prefix string, dirs []string) []string {
	if len(dirs) == 0 {
		return nil
	}
	result := make([]string, len(dirs))
	for i, d := range dirs {
		result[i] = filepath.Join(prefix, d)
	}
	return result
}
