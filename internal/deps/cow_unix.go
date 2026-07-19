//go:build unix

package deps

import (
	"os"
	"syscall"
)

// sameDevice reports whether a and b live on the same filesystem device —
// the necessary precondition for a CoW reflink/clonefile between them.
// ok is false when either path's device id can't be determined, in which
// case same is meaningless and callers must not treat it as a match.
func sameDevice(a, b string) (same bool, ok bool) {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, false
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false, false
	}

	aStat, aOK := aInfo.Sys().(*syscall.Stat_t)
	bStat, bOK := bInfo.Sys().(*syscall.Stat_t)
	if !aOK || !bOK {
		return false, false
	}

	return aStat.Dev == bStat.Dev, true
}
