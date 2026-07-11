package proc

import (
	"os"
	"testing"
)

func TestAliveCurrentProcess(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Error("Alive(os.Getpid()) = false, want true")
	}
}
