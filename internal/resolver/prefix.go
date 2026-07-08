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
//
// Deprecated: this is a thin wrapper over DefaultPrefixSet().Strip(),
// kept for existing call sites (see #269).
func AllPrefixes() []string {
	return DefaultPrefixSet().Strip()
}

// ValidPrefixType reports whether s is a recognized PrefixType value.
//
// Deprecated: this is a thin wrapper over DefaultPrefixSet().ValidType(),
// kept for existing call sites (see #269).
func ValidPrefixType(s string) bool {
	return DefaultPrefixSet().ValidType(s)
}

// PrefixTokenToString resolves a leading path segment that is either a
// canonical prefix name ("bugfix") or a known alias ("fix") to its branch
// prefix string. alias is true when the token was a non-canonical alias;
// ok is false when the token is neither a prefix nor an alias.
//
// Deprecated: this is a thin wrapper over DefaultPrefixSet().TokenToPrefix(),
// kept for existing call sites (see #269).
func PrefixTokenToString(token string) (prefix string, alias bool, ok bool) {
	return DefaultPrefixSet().TokenToPrefix(token)
}
