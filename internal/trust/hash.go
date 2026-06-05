// Package trust manages per-repo consent for committed shell commands.
package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
)

// Commands returns all shell-executing strings from cfg in display order:
// post_create, then post_rename, then non-empty deps.modules[].install.
func Commands(cfg *config.Config) []string {
	var cmds []string
	cmds = append(cmds, cfg.PostCreate...)
	cmds = append(cmds, cfg.PostRename...)
	if cfg.Deps != nil {
		for _, m := range cfg.Deps.Modules {
			if strings.TrimSpace(m.Install) != "" {
				cmds = append(cmds, m.Install)
			}
		}
	}
	return cmds
}

// HasCommands reports whether cfg contains any committed shell commands.
func HasCommands(cfg *config.Config) bool {
	return len(Commands(cfg)) > 0
}

// Hash returns a canonical "sha256:<hex>" fingerprint of cfg's command set.
// Returns "" when there are no commands.
//
// The hash is field-blind: the field a command originates from (post_create,
// post_rename, or deps install) does not affect the hash, only its string
// value does. This means moving a command between fields without changing its
// content does not require re-consent — restructuring config is not a new
// threat. Conversely, adding the same command string to a second field (e.g.
// adding an already-approved post_create string to post_rename) does not
// re-arm the gate; re-consent is only required when the command content itself
// is new or changed.
func Hash(cfg *config.Config) string {
	cmds := Commands(cfg)
	if len(cmds) == 0 {
		return ""
	}

	sorted := make([]string, len(cmds))
	copy(sorted, cmds)
	sort.Strings(sorted)

	h := sha256.New()
	for _, c := range sorted {
		h.Write([]byte(c))
		h.Write([]byte{0}) // NUL delimiter prevents "ab"+"c" == "a"+"bc"
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
