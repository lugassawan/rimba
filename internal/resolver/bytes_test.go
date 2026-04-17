package resolver_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{15 * 1024 * 1024, "15MB"},
		{1932735283, "1.8GB"}, // 1.8 * 1024^3, rounded to nearest byte
		{2 * 1024 * 1024 * 1024 * 1024, "2.0TB"},
		{-1, "0B"},
	}
	for _, tc := range cases {
		got := resolver.FormatBytes(tc.n)
		if got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
