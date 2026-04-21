package resolver

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix: login redirect!", "fix-login-redirect"},
		{"Add OAuth2 support", "add-oauth2-support"},
		{"  leading/trailing spaces  ", "leading-trailing-spaces"},
		{"múltiple unicøde chars", "m-ltiple-unic-de-chars"},
		{"already-fine", "already-fine"},
		{"UPPERCASE TITLE", "uppercase-title"},
		{"dots.and.dots", "dots-and-dots"},
		{"consecutive---dashes", "consecutive-dashes"},
		{"", "pr"},
		{"   ", "pr"},
		{"---", "pr"},
		// Long title capped at 50 chars, no trailing dash
		{"This is a very long pull request title that should be trimmed at fifty chars exactly", "this-is-a-very-long-pull-request-title-that-should"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > 50 {
				t.Errorf("Slugify(%q) len=%d > 50", tt.input, len(got))
			}
		})
	}
}
