package agentfile

import (
	"strings"
	"testing"
)

func TestContainsBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"both markers", BeginMarker + "\ncontent\n" + EndMarker, true},
		{"begin only", BeginMarker + "\ncontent", false},
		{"end only", "content\n" + EndMarker, false},
		{"no markers", "just content", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsBlock(tt.content); got != tt.want {
				t.Errorf("containsBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveBlock(t *testing.T) {
	block := BeginMarker + "\nrimba content\n" + EndMarker

	t.Run("removes block preserving surrounding", func(t *testing.T) {
		content := "# Header\n\n" + block + "\n\n# Footer\n"
		result := removeBlock(content)
		if strings.Contains(result, BeginMarker) {
			t.Error("BEGIN marker should be removed")
		}
		if !strings.Contains(result, "Header") {
			t.Error("content before block should be preserved")
		}
		if !strings.Contains(result, "Footer") {
			t.Error("content after block should be preserved")
		}
	})

	t.Run("removes block at start", func(t *testing.T) {
		content := block + "\n"
		result := removeBlock(content)
		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})

	t.Run("no markers returns unchanged", func(t *testing.T) {
		content := "just some text"
		result := removeBlock(content)
		if result != content {
			t.Errorf("expected unchanged content, got %q", result)
		}
	})

	t.Run("corrupt begin only", func(t *testing.T) {
		content := "# Header\n\n" + BeginMarker + "\ncorrupt"
		result := removeBlock(content)
		if strings.Contains(result, BeginMarker) {
			t.Error("BEGIN marker should be removed")
		}
		if !strings.Contains(result, "Header") {
			t.Error("content before block should be preserved")
		}
	})
}

func TestRemoveBlockCorruptBeginAtStart(t *testing.T) {
	// BEGIN at the very start of content, no END marker
	content := BeginMarker + "\nsome corrupt content"
	result := removeBlock(content)
	if result != "" {
		t.Errorf("expected empty result when BEGIN is at start with no END, got %q", result)
	}
}
