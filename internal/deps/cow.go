package deps

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// cowProbeCache memoizes probeCowCapable per destination directory for the
// life of the process — one real probe per `rimba add`/`restore`/etc.
// invocation regardless of how many modules clone into the same worktree.
var cowProbeCache sync.Map // dstDir string -> bool

// cowEligibleOverrideEnv lets e2e tests pin cowEligible's decision ("1" or
// "0") instead of depending on the test host's real filesystem. Internal test
// seam only — not a documented user-facing config knob — needed because CI
// runners' temp filesystems don't reliably support (or reliably lack)
// reflink/clonefile, and the compiled e2e binary can't use the Go-level
// var-injection seam unit tests use.
const cowEligibleOverrideEnv = "RIMBA_COW_ELIGIBLE_OVERRIDE"

// cowEligible reports whether cloning src onto dstDir is expected to be a
// true reflink/clonefile (near-instant) rather than a byte-copy in disguise.
//
// The production copy path (cowCopyCmd) intentionally uses the permissive
// "-c"/"--reflink=auto" flags, which silently perform a full byte-copy
// instead of erroring when the filesystem can't honor a reflink — that
// silent fallback is exactly what turned a handful of "cloned" node_modules
// spans into 14-123s pessimizations instead of the sub-second clones the
// flag promises. This probe answers the question those flags refuse to:
// same-device is the necessary precondition, and the temp-file probe (using
// the strict "always"/"-c" forms, which DO fail outright rather than
// silently falling back) confirms the filesystem actually honors it.
//
// A package var so tests can force the outcome deterministically instead of
// depending on the test host's real filesystem.
var cowEligible = func(ctx context.Context, src, dstDir string) bool {
	if v, ok := os.LookupEnv(cowEligibleOverrideEnv); ok {
		return v == "1"
	}

	if same, ok := sameDevice(src, dstDir); !ok || !same {
		return false
	}

	if v, cached := cowProbeCache.Load(dstDir); cached {
		return v.(bool) //nolint:forcetypeassert // only this file ever stores into cowProbeCache
	}
	result := probeCowCapable(ctx, dstDir)
	cowProbeCache.Store(dstDir, result)
	return result
}

// cowProbeCmd builds the strict CoW-probe copy command for the host OS. A
// package var so tests can inject a synthetic probe outcome, mirroring
// cowCopyCmd's own injection seam.
var cowProbeCmd = func(ctx context.Context, src, dst string) *exec.Cmd {
	switch runtime.GOOS {
	case goosDarwin:
		return exec.CommandContext(ctx, "cp", "-c", src, dst)
	case goosLinux:
		// --reflink=always (unlike the production path's =auto) errors
		// instead of silently falling back, which is exactly the signal
		// this probe needs.
		return exec.CommandContext(ctx, "cp", "--reflink=always", src, dst)
	default:
		return exec.CommandContext(ctx, "false")
	}
}

// probeCowCapable attempts a real reflink/clonefile of a throwaway temp file
// inside dir and reports whether it succeeded. Any setup failure (can't
// create the temp file, unsupported OS) reports false: the downside of a
// false negative here is a fast install (~2-5s); the downside of a false
// positive is the 14-123s pessimization this probe exists to prevent.
func probeCowCapable(ctx context.Context, dir string) bool {
	if runtime.GOOS != goosDarwin && runtime.GOOS != goosLinux {
		return false
	}

	srcFile, err := os.CreateTemp(dir, ".rimba-cow-probe-*")
	if err != nil {
		return false
	}
	srcPath := srcFile.Name()
	_ = srcFile.Close()
	defer func() { _ = os.Remove(srcPath) }()

	dstPath := srcPath + ".dst"
	defer func() { _ = os.Remove(dstPath) }()

	cmd := cowProbeCmd(ctx, srcPath, dstPath)
	configureProcessGroup(cmd)
	_, err = cmd.CombinedOutput()
	return err == nil
}
