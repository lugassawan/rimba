package resolver

// PrefixType represents a branch prefix category.
type PrefixType string

const (
	PrefixFeature PrefixType = "feature"
	PrefixBugfix  PrefixType = "bugfix"
	PrefixHotfix  PrefixType = "hotfix"
	PrefixDocs    PrefixType = "docs"
	PrefixTest    PrefixType = "test"
	PrefixChore   PrefixType = "chore"
)

// DefaultPrefixType is the prefix used when no explicit type flag is provided.
var DefaultPrefixType = PrefixFeature

// prefixMap maps each PrefixType to its branch prefix string.
var prefixMap = map[PrefixType]string{
	PrefixFeature: "feature/",
	PrefixBugfix:  "bugfix/",
	PrefixHotfix:  "hotfix/",
	PrefixDocs:    "docs/",
	PrefixTest:    "test/",
	PrefixChore:   "chore/",
}

// orderedTypes defines the deterministic iteration order for AllPrefixes.
var orderedTypes = []PrefixType{
	PrefixFeature,
	PrefixBugfix,
	PrefixHotfix,
	PrefixDocs,
	PrefixTest,
	PrefixChore,
}

// prefixAliases maps non-canonical tokens to their canonical PrefixType.
// Compiled-in and closed, like prefixMap — not user-configurable (see #269).
var prefixAliases = map[string]PrefixType{
	"fix": PrefixBugfix,
}

// PrefixString returns the branch prefix string for a PrefixType.
// The second return value is false if the type is unknown.
func PrefixString(pt PrefixType) (string, bool) {
	s, ok := prefixMap[pt]
	return s, ok
}

// AllPrefixes returns all known prefix strings in a deterministic order.
func AllPrefixes() []string {
	out := make([]string, len(orderedTypes))
	for i, pt := range orderedTypes {
		out[i] = prefixMap[pt]
	}
	return out
}

// ValidPrefixType reports whether s is a recognized PrefixType value.
func ValidPrefixType(s string) bool {
	_, ok := prefixMap[PrefixType(s)]
	return ok
}

// PrefixTokenToString resolves a leading path segment that is either a
// canonical prefix name ("bugfix") or a known alias ("fix") to its branch
// prefix string. alias is true when the token was a non-canonical alias;
// ok is false when the token is neither a prefix nor an alias.
func PrefixTokenToString(token string) (prefix string, alias bool, ok bool) {
	if s, ok := prefixMap[PrefixType(token)]; ok {
		return s, false, true
	}
	if pt, ok := prefixAliases[token]; ok {
		return prefixMap[pt], true, true
	}
	return "", false, false
}
