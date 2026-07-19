package deps

import (
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
)

// Lockfile and directory constants used for ecosystem detection.
const (
	LockfilePnpm   = "pnpm-lock.yaml"
	LockfileYarn   = "yarn.lock"
	LockfileNpm    = "package-lock.json"
	LockfileGo     = "go.sum"
	LockfileCargo  = "Cargo.lock"
	LockfileUv     = "uv.lock"
	LockfilePoetry = "poetry.lock"

	LockfileGradleSettings    = "settings.gradle"
	LockfileGradleSettingsKts = "settings.gradle.kts"
	LockfileGradle            = "build.gradle"
	LockfileGradleKts         = "build.gradle.kts"

	DirNodeModules       = "node_modules"
	DirVendor            = "vendor"
	DirYarnCache         = ".yarn/cache"
	DirTarget            = "target"
	DirVenv              = ".venv"
	DirGradle            = ".gradle"
	DirGradleBuildOutput = "build"
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
	// Eager is computed by ResolveModules (see resolveEagerness): whether
	// this module installs automatically or is deferred until explicitly
	// requested via `rimba deps install --path`.
	Eager     bool                                        `json:"eager"`
	PostClone func(srcWT, dstWT string, mod Module) error `json:"-"` // Optional hook run after successful clone.
}

type preset struct {
	Lockfile   string
	Dir        string
	InstallCmd string
	Recursive  bool
	ExtraDirs  []string
	CloneOnly  bool
	Relocate   bool
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
	{
		Lockfile:  LockfileCargo,
		Dir:       DirTarget,
		CloneOnly: true,
	},
	{
		Lockfile:  LockfileUv,
		Dir:       DirVenv,
		CloneOnly: true,
		Relocate:  true,
	},
	{
		Lockfile:  LockfilePoetry,
		Dir:       DirVenv,
		CloneOnly: true,
		Relocate:  true,
	},
	// Gradle: clone project-local build state (.gradle/ + build/) from a sibling
	// worktree. CloneOnly with no InstallCmd — rimba never invokes gradle; a stale
	// clone is a harmless warm cache (Gradle re-validates via content hashes).
	// settings.* ordered before build.* so a multi-project root is preferred.
	{Lockfile: LockfileGradleSettings, Dir: DirGradle, ExtraDirs: []string{DirGradleBuildOutput}, CloneOnly: true},
	{Lockfile: LockfileGradleSettingsKts, Dir: DirGradle, ExtraDirs: []string{DirGradleBuildOutput}, CloneOnly: true},
	{Lockfile: LockfileGradle, Dir: DirGradle, ExtraDirs: []string{DirGradleBuildOutput}, CloneOnly: true},
	{Lockfile: LockfileGradleKts, Dir: DirGradle, ExtraDirs: []string{DirGradleBuildOutput}, CloneOnly: true},
}

// DetectModules scans a worktree for known lockfiles. Root is always scanned.
// When service is non-empty, only that subdirectory is checked instead of all depth-1 dirs.
func DetectModules(worktreePath, service string) ([]Module, error) {
	var modules []Module
	seenDirs := make(map[string]bool)

	// Phase 1: Scan root for lockfiles
	modules = detectRootModules(worktreePath, modules, seenDirs)

	// Phase 2: Scan depth-1 subdirectories for lockfiles
	modules = detectSubdirModules(worktreePath, service, modules, seenDirs)

	return modules, nil
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
// A config entry whose Dir matches a detected module patches it (only the
// config entry's explicitly-set fields override — see patchModule); a Dir
// matching nothing detected defines a brand-new module, fully from config.
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
			result = append(result, patchModule(m, cm))
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

// patchModule applies cm's explicitly-set fields onto an auto-detected
// module, keeping everything else — including Recursive/CloneOnly/ExtraDirs,
// which ModuleConfig has no field for — from detection. Never blanks a field
// the config entry left empty, unlike moduleFromConfig.
func patchModule(detected Module, cm config.ModuleConfig) Module {
	patched := detected
	if cm.Lockfile != "" {
		patched.Lockfile = cm.Lockfile
	}
	if cm.Install != "" {
		patched.InstallCmd = cm.Install
	}
	if cm.WorkDir != "" {
		patched.WorkDir = cm.WorkDir
	}
	return patched
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

func detectSubdirModules(worktreePath, service string, modules []Module, seenDirs map[string]bool) []Module {
	if service != "" {
		return matchPresetsInSubdir(worktreePath, service, modules, seenDirs)
	}

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
	mod := Module{
		Dir:        p.Dir,
		Lockfile:   p.Lockfile,
		InstallCmd: p.InstallCmd,
		Recursive:  p.Recursive,
		ExtraDirs:  p.ExtraDirs,
		CloneOnly:  p.CloneOnly,
	}
	if subdir != "" {
		mod.Dir = depDir
		mod.Lockfile = filepath.Join(subdir, p.Lockfile)
		mod.WorkDir = subdir
		mod.ExtraDirs = prefixDirs(subdir, p.ExtraDirs)
	}
	if p.Relocate {
		mod.PostClone = relocateVenv
	}
	return mod
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
