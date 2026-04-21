package resolver

import (
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts s into a lowercase, hyphen-separated branch-safe string.
// Non-alphanumeric runs are collapsed to a single "-", edges are trimmed,
// and the result is capped at 50 characters. Returns "pr" for empty input.
func Slugify(s string) string {
	slug := nonAlphanumRe.ReplaceAllString(strings.ToLower(s), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "pr"
	}
	if len(slug) > 50 {
		slug = strings.TrimRight(slug[:50], "-")
	}
	return slug
}
