package agentfile

import (
	"errors"
	"strings"
)

// containsBlock checks whether content includes a well-formed rimba marker block.
func containsBlock(content string) bool {
	return strings.Contains(content, BeginMarker) && strings.Contains(content, EndMarker)
}

// removeBlock strips the rimba marker block from content. It refuses to guess at
// content boundaries when BEGIN has no matching END, returning errCorruptBlock
// instead of silently truncating user prose.
func removeBlock(content string) (string, error) {
	before, afterBegin, found := strings.Cut(content, BeginMarker)
	if !found {
		return content, nil
	}

	_, afterEnd, found := strings.Cut(afterBegin, EndMarker)
	if !found {
		return "", errCorruptBlock
	}

	after := strings.TrimLeft(afterEnd, "\n")
	before = strings.TrimRight(before, "\n")

	if before == "" {
		return after, nil
	}
	if after == "" {
		return before + "\n", nil
	}
	return before + "\n" + after, nil
}

// isCorruptBlock reports whether content has a malformed rimba block: an
// orphaned BEGIN (no matching END) or a duplicate BEGIN marker.
func isCorruptBlock(content string) bool {
	if strings.Count(content, BeginMarker) > 1 {
		return true
	}
	_, err := removeBlock(content)
	return errors.Is(err, errCorruptBlock)
}
