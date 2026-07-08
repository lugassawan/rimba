package resolver

import "strings"

// PrefixSpec describes a single prefix registration: the branch prefix
// string itself (e.g. "feature/" or a custom "PROJ-") and the creation
// tokens (aliases) that should resolve to it.
type PrefixSpec struct {
	Prefix  string
	Aliases []string
}

// PrefixSet is a registry of branch prefixes and their creation aliases.
// DefaultPrefixSet reproduces today's hardcoded behavior exactly; NewPrefixSet
// additively merges custom specs on top of the built-ins (see #269).
type PrefixSet struct {
	strip         []string            // ordered prefix strings to strip, in match priority order
	tokenToPrefix map[string]string   // creation token (canonical type name or alias) -> prefix string
	membership    map[string]struct{} // set of registered prefix strings, for merge dedup
	hasCustom     bool
}

// DefaultPrefixSet returns a PrefixSet containing only the built-in prefixes,
// identical in behavior to the package-level free functions in prefix.go.
func DefaultPrefixSet() *PrefixSet {
	s := &PrefixSet{
		strip:         make([]string, 0, len(orderedTypes)),
		tokenToPrefix: make(map[string]string, len(orderedTypes)+len(prefixAliases)),
		membership:    make(map[string]struct{}, len(orderedTypes)),
	}

	for _, pt := range orderedTypes {
		p := prefixMap[pt]
		s.strip = append(s.strip, p)
		s.membership[p] = struct{}{}
		s.tokenToPrefix[string(pt)] = p
	}

	for token, pt := range prefixAliases {
		s.tokenToPrefix[token] = prefixMap[pt]
	}

	return s
}

// NewPrefixSet builds a PrefixSet by additively merging custom specs on top
// of the built-ins. A custom spec whose Prefix matches an existing built-in
// prefix string folds its Aliases into that built-in (without duplicating the
// prefix in Strip()). A custom spec with a new Prefix string registers a
// brand-new creatable+strippable type and sets HasCustom() true.
func NewPrefixSet(custom []PrefixSpec) *PrefixSet {
	s := DefaultPrefixSet()
	for _, spec := range custom {
		s.foldOrAdd(spec)
	}
	return s
}

// Strip returns the ordered list of all prefix strings to strip, in match
// priority order.
func (s *PrefixSet) Strip() []string {
	out := make([]string, len(s.strip))
	copy(out, s.strip)
	return out
}

// TokenToPrefix resolves a leading path segment that is either a canonical
// type name ("bugfix") or a known alias ("fix") to its branch prefix string.
// alias is true when the token was a non-canonical alias; ok is false when
// the token is neither a known type name nor an alias.
func (s *PrefixSet) TokenToPrefix(t string) (prefix string, alias bool, ok bool) {
	p, ok := s.tokenToPrefix[t]
	if !ok {
		return "", false, false
	}
	return p, s.TypeName(p) != t, true
}

// ValidType reports whether t is a recognized canonical type name (not an
// alias).
func (s *PrefixSet) ValidType(t string) bool {
	p, ok := s.tokenToPrefix[t]
	if !ok {
		return false
	}
	return s.TypeName(p) == t
}

// TypeName returns the display type name for a matched branch prefix string.
// For built-ins this is the canonical PrefixType string (e.g. "feature/" ->
// "feature"). For custom prefixes it strips a trailing "/" if present,
// otherwise returns the prefix as-is (e.g. "PROJ-" -> "PROJ-").
func (s *PrefixSet) TypeName(prefix string) string {
	return strings.TrimSuffix(prefix, "/")
}

// TypeToPrefix is the inverse of TypeName: given a display type name, it
// returns the registered prefix string, if any.
func (s *PrefixSet) TypeToPrefix(typeName string) (string, bool) {
	for _, p := range s.strip {
		if s.TypeName(p) == typeName {
			return p, true
		}
	}
	return "", false
}

// TypeNames returns the display type name for every registered prefix, in
// Strip() order — the single source of truth for "valid types" hints and
// shell completions, so built-in and custom prefixes are listed uniformly.
func (s *PrefixSet) TypeNames() []string {
	out := make([]string, len(s.strip))
	for i, p := range s.strip {
		out[i] = s.TypeName(p)
	}
	return out
}

// IsOrphan reports whether branch was created under a prefix that is no
// longer registered in this set. It is always false when branch is the main
// branch, regardless of prefix match.
func (s *PrefixSet) IsOrphan(branch, mainBranch string) bool {
	if branch == mainBranch {
		return false
	}
	for _, p := range s.strip {
		if _, ok := strings.CutPrefix(branch, p); ok {
			return false
		}
	}
	return true
}

// HasCustom reports whether this set includes any custom (non-built-in)
// prefix registrations.
func (s *PrefixSet) HasCustom() bool {
	return s.hasCustom
}

// foldOrAdd merges a single custom spec into s: folding its aliases into an
// existing prefix, or registering it as a brand-new type.
func (s *PrefixSet) foldOrAdd(spec PrefixSpec) {
	if _, exists := s.membership[spec.Prefix]; exists {
		for _, a := range spec.Aliases {
			s.tokenToPrefix[a] = spec.Prefix
		}
		return
	}

	s.strip = append(s.strip, spec.Prefix)
	s.membership[spec.Prefix] = struct{}{}
	s.tokenToPrefix[s.TypeName(spec.Prefix)] = spec.Prefix
	for _, a := range spec.Aliases {
		s.tokenToPrefix[a] = spec.Prefix
	}
	s.hasCustom = true
}
