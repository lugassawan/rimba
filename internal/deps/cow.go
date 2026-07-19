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

// cowProbeOnce guards cowProbeCache's first write per dstDir so concurrent
// modules probing the same new destination share a single real probe instead
// of each racing to populate the cache.
var cowProbeOnce sync.Map // dstDir string -> *sync.Once

// cowEligibleOverrideEnv lets e2e tests pin cowEligible's decision ("1" or
// "0"), since CI temp filesystems don't reliably support (or lack)
// reflink/clonefile and the compiled e2e binary can't use the Go-injection
// seam unit tests use. Internal test seam only, not a user-facing config knob.
const cowEligibleOverrideEnv = "RIMBA_COW_ELIGIBLE_OVERRIDE"

// cowEligible reports whether cloning src onto dstDir is a true
// reflink/clonefile (near-instant) rather than cowCopyCmd's permissive
// "-c"/"--reflink=auto" silently falling back to a full byte-copy — the exact
// silent fallback that turned "cloned" node_modules spans into 14-123s
// pessimizations. A package var so tests can force the outcome deterministically.
var cowEligible = func(ctx context.Context, src, dstDir string) bool {
	if v, ok := os.LookupEnv(cowEligibleOverrideEnv); ok {
		return v == "1"
	}

	if same, ok := sameDevice(src, dstDir); !ok || !same {
		return false
	}

	onceI, _ := cowProbeOnce.LoadOrStore(dstDir, new(sync.Once))
	once := onceI.(*sync.Once) //nolint:forcetypeassert // only this file ever stores into cowProbeOnce
	once.Do(func() {
		cowProbeCache.Store(dstDir, probeCowCapable(ctx, dstDir))
	})
	v, _ := cowProbeCache.Load(dstDir)
	return v.(bool) //nolint:forcetypeassert // once.Do guarantees a store before this load
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
