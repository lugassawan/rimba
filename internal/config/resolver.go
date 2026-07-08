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
// custom branch prefixes and creation aliases on top of the built-ins (see #269).
type ResolverConfig struct {
	Prefix []PrefixEntry `toml:"prefix"`
}

// PrefixSet builds the resolver.PrefixSet for this config: the built-in
// defaults when no [resolver] section is configured, or the built-ins
// additively merged with the configured custom entries otherwise.
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

// PrefixSetFromContext resolves a resolver.PrefixSet from the *Config stored
// in ctx (see WithConfig). It is total: it never panics and never returns
// nil, degrading to resolver.DefaultPrefixSet() when ctx carries no Config.
// This is the single funnel callers use to go from ctx to a valid PrefixSet.
func PrefixSetFromContext(ctx context.Context) *resolver.PrefixSet {
	cfg := FromContext(ctx)
	if cfg == nil {
		return resolver.DefaultPrefixSet()
	}
	return cfg.PrefixSet()
}

// validateResolver checks the [resolver] section for unsafe prefixes,
// malformed aliases, and collisions (duplicate prefixes/aliases, or aliases
// that would shadow a built-in type/alias, or another custom entry's own
// canonical type name, on an unrelated prefix).
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

// buildOwnTokens maps each entry's own canonical type-name token — the token
// NewPrefixSet implicitly registers for a brand-new custom prefix (or reuses
// when folding into a built-in) — to the prefix it belongs to. This lets
// validateAliasNotShadowing catch an alias silently stealing another
// entry's own creation token, not just a built-in's.
func buildOwnTokens(entries []PrefixEntry, builtins *resolver.PrefixSet) map[string]string {
	own := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Prefix == "" {
			continue
		}
		own[builtins.TypeName(e.Prefix)] = e.Prefix
	}
	return own
}

// validateResolverPrefix checks a single entry's Prefix field: non-empty,
// ref-safe, unique across all [[resolver.prefix]] entries, and not silently
// colliding with a built-in's or another entry's own canonical token.
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

// validatePrefixNotColliding reports an error if prefix, once registered,
// would silently overwrite an existing token mapping: a built-in's own type
// name or alias reused as a raw prefix (e.g. "bugfix" without the trailing
// "/", or the "fix" alias), or another entry's own canonical token (e.g.
// "PROJ/" and "PROJ" both normalizing to the same type). Without this check,
// PrefixSet.foldOrAdd's exact-string membership match treats such a prefix
// as brand new and silently redefines the colliding token.
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

// validateResolverAliases checks entry's Aliases: non-empty, no '/', unique
// across all entries (including within the same entry), and not shadowing a
// built-in type/alias or another entry's own canonical token, unless
// entry.Prefix is the exact prefix that token belongs to.
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

// validateAliasNotShadowing reports an error if alias resolves to a built-in
// prefix, or another entry's own canonical token, different from
// entryPrefix — i.e. the alias would silently steal a creation token from an
// unrelated prefix. An entry that redeclares its own built-in or own token
// (entryPrefix equals the prefix the token resolves to) is not a shadow: it
// is folding into or re-affirming that exact prefix.
func validateAliasNotShadowing(alias, entryPrefix string, builtins *resolver.PrefixSet, ownTokens map[string]string) error {
	if builtinPrefix, _, ok := builtins.TokenToPrefix(alias); ok && builtinPrefix != entryPrefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix: alias %q shadows built-in prefix %q", alias, builtinPrefix),
			"rename the alias, or set prefix = \""+builtinPrefix+"\" in [[resolver.prefix]] in .rimba/settings.toml",
		)
	}
	if ownerPrefix, ok := ownTokens[alias]; ok && ownerPrefix != entryPrefix {
		return errhint.WithFix(
			fmt.Errorf("config: resolver.prefix: alias %q shadows prefix %q", alias, ownerPrefix),
			"rename the alias, or set prefix = \""+ownerPrefix+"\" in [[resolver.prefix]] in .rimba/settings.toml",
		)
	}
	return nil
}
