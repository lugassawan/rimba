package gh

import (
	"os/exec"
	"testing"
)

func TestIsAvailableMissing(t *testing.T) {
	t.Setenv("PATH", "")
	if IsAvailable() {
		t.Fatal("IsAvailable() = true with empty PATH, want false")
	}
}

func TestIsAvailablePresent(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed on host; skipping positive detection test")
	}
	if !IsAvailable() {
		t.Fatal("IsAvailable() = false with gh on PATH, want true")
	}
}
