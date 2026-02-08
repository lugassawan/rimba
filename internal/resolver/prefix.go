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
