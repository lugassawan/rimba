package agentfile

import "strings"

// containsBlock checks whether content includes the rimba marker block.
func containsBlock(content string) bool {
	return strings.Contains(content, BeginMarker) && strings.Contains(content, EndMarker)
}

// removeBlock strips the rimba marker block from content.
func removeBlock(content string) string {
	before, afterBegin, found := strings.Cut(content, BeginMarker)
	if !found {
		return content
	}

	_, afterEnd, found := strings.Cut(afterBegin, EndMarker)
	if !found {
		// Corrupt: BEGIN without END — remove from BEGIN to end of file
		before = strings.TrimRight(before, "\n")
		if before == "" {
			return ""
		}
		return before + "\n"
	}

	after := strings.TrimLeft(afterEnd, "\n")
	before = strings.TrimRight(before, "\n")

	if before == "" {
		return after
	}
	if after == "" {
		return before + "\n"
	}
	return before + "\n" + after
}
