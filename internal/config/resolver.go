package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/gitref"
	"github.com/lugassawan/rimba/internal/resolver"
)

// PrefixEntry declares a single custom branch prefix registration under the
// [[resolver.prefix]] array-of-tables in .rimba/settings.toml.
type PrefixEntry struct {
	Prefix  string   `toml:"prefix"`
	Aliases []string `toml:"aliases,omitempty"`
}

// ResolverConfig holds the optional [resolver] section, letting teams declare
// custom branch prefixes and creation aliases on top of the built-ins.
type ResolverConfig struct {
	Prefix []PrefixEntry `toml:"prefix"`
}

// PrefixSet builds the resolver.PrefixSet for this config, merging built-in
// defaults with any configured custom entries.
func (c *Config) PrefixSet() *resolver.PrefixSet {
	if c.Resolver == nil || len(c.Resolver.Prefix) == 0 {
		return resolver.DefaultPrefixSet()
	}

	specs := make([]resolver.PrefixSpec, len(c.Resolver.Prefix))
	for i, e := range c.Resolver.Prefix {
		specs[i] = resolver.PrefixSpec{Prefix: e.Prefix, Aliases: e.Aliases}
	}
	return resolver.NewPrefixSet(specs)
}

// PrefixSetFromContext is total: it never panics, degrading to
// resolver.DefaultPrefixSet() when ctx carries no Config.
func PrefixSetFromContext(ctx context.Context) *resolver.PrefixSet {
	cfg := FromContext(ctx)
	if cfg == nil {
		return resolver.DefaultPrefixSet()
	}
	return cfg.PrefixSet()
}

// validateResolver checks the [resolver] section for unsafe prefixes,
// malformed aliases, and collisions with built-ins or other custom entries.
func validateResolver(rc *ResolverConfig) []error {
	if rc == nil {
		return nil
	}

	var errs []error
	seenPrefixes := make(map[string]bool, len(rc.Prefix))
	seenAliases := make(map[string]bool)
	builtins := resolver.DefaultPrefixSet()
	ownTokens := buildOwnTokens(rc.Prefix, builtins)

	for i, entry := range rc.Prefix {
		errs = append(errs, validateResolverPrefix(i, entry.Prefix, seenPrefixes, builtins, ownTokens)...)
		errs = append(errs, validateResolverAliases(entry, seenAliases, builtins, ownTokens)...)
	}
	return errs
}

// buildOwnTokens maps each entry's own canonical type-name token to its
// prefix; first-write-wins, so collisions blame later duplicates, not order.
func buildOwnTokens(entries []PrefixEntry, builtins *resolver.PrefixSet) map[string]string {
	own := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Prefix == "" {
			continue
		}
		token := builtins.TypeName(e.Prefix)
		if _, exists := own[token]; exists {
			continue
		}
		own[token] = e.Prefix
	}
	return own
}

// validateResolverPrefix checks a single entry's Prefix: non-empty, ref-safe,
// unique, and not silently colliding with a built-in or another entry's token.
func validateResolverPrefix(index int, prefix string, seen map[string]bool, builtins *resolver.PrefixSet, ownTokens map[string]string) []error {
	if prefix == "" {
		return []error{errhint.WithFix(
			fmt.Errorf("config: resolver.prefix[%d]: prefix is empty", index),
			"set prefix = \"<prefix>\" for the entry in [[resolver.prefix]] in .rimba/settings.toml",
		)}
	}
	if err := gitref.Validate(prefix); err != nil {
		return []error{errhint.WithFix(
			fmt.Errorf("config: resolver.prefix[%d]: %w", index, err),
			"use a safe prefix string (letters, digits, '.', '_', '/', '-') in .rimba/settings.toml",
		)}
	}
	if seen[prefix] {
		return []error{errhint.WithFix(
			fmt.Errorf("config: resolver.prefix[%d]: duplicate prefix %q", index, prefix),
			"remove the duplicate [[resolver.prefix]] entry in .rimba/settings.toml",
		)}
	}
	seen[prefix] = true
	if err := validatePrefixNotColliding(index, prefix, builtins, ownTokens); err != nil {
		return []error{err}
	}
	return nil
}

// validatePrefixNotColliding catches a prefix that would silently redefine an
// existing token, since PrefixSet.foldOrAdd's exact-string match can't detect it.
func validatePrefixNotColliding(index int, prefix string, builtins *resolver.PrefixSet, ownTokens map[string]string) error {
	if builtinPrefix, _, ok := builtins.TokenToPrefix(prefix); ok && builtinPrefix != prefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix[%d]: prefix %q collides with built-in prefix %q", index, prefix, builtinPrefix),
			"choose a prefix that doesn't match a built-in type name or alias in .rimba/settings.toml",
		)
	}
	if owner, ok := ownTokens[builtins.TypeName(prefix)]; ok && owner != prefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix[%d]: prefix %q collides with another entry's prefix %q", index, prefix, owner),
			"use distinct prefixes that don't normalize to the same type name in .rimba/settings.toml",
		)
	}
	return nil
}

// validateResolverAliases checks entry's Aliases: non-empty, no '/', unique,
// and not shadowing a built-in or another entry's own canonical token.
func validateResolverAliases(entry PrefixEntry, seenAliases map[string]bool, builtins *resolver.PrefixSet, ownTokens map[string]string) []error {
	var errs []error
	for _, alias := range entry.Aliases {
		switch {
		case alias == "":
			errs = append(errs, errhint.WithFix(
				errors.New("config: resolver.prefix: alias is empty"),
				"remove the empty alias entry in [[resolver.prefix]] in .rimba/settings.toml",
			))
		case strings.ContainsRune(alias, '/'):
			errs = append(errs, errhint.WithFix(
				fmt.Errorf("config: resolver.prefix: alias %q must not contain '/'", alias),
				"rename the alias in [[resolver.prefix]] to remove '/' in .rimba/settings.toml",
			))
		case seenAliases[alias]:
			errs = append(errs, errhint.WithFix(
				fmt.Errorf("config: resolver.prefix: duplicate alias %q", alias),
				"remove the duplicate alias across [[resolver.prefix]] entries in .rimba/settings.toml",
			))
		default:
			seenAliases[alias] = true
			if err := validateAliasNotShadowing(alias, entry.Prefix, builtins, ownTokens); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// validateAliasNotShadowing flags an alias stealing a creation token from an
// unrelated prefix; redeclaring the entry's own built-in/token is not a shadow.
func validateAliasNotShadowing(alias, entryPrefix string, builtins *resolver.PrefixSet, ownTokens map[string]string) error {
	if builtinPrefix, _, ok := builtins.TokenToPrefix(alias); ok && builtinPrefix != entryPrefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix: alias %q shadows built-in prefix %q", alias, builtinPrefix),
			"rename the alias, or set prefix = \""+builtinPrefix+"\" in [[resolver.prefix]] in .rimba/settings.toml",
		)
	}
	if alias == builtins.TypeName(entryPrefix) {
		// Already reported by validatePrefixNotColliding against entryPrefix.
		return nil
	}
	if ownerPrefix, ok := ownTokens[alias]; ok && ownerPrefix != entryPrefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix: alias %q shadows prefix %q", alias, ownerPrefix),
			"rename the alias, or set prefix = \""+ownerPrefix+"\" in [[resolver.prefix]] in .rimba/settings.toml",
		)
	}
	return nil
}
