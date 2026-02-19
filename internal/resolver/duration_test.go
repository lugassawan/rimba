package resolver_test

import (
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"3h", 3 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"", 0, true},
		{"x", 0, true},
		{"7x", 0, true},
		{"abc", 0, true},
		{"-5d", 0, true},
		{"0d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := resolver.ParseDuration(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
