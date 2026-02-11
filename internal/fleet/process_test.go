package fleet

import (
	"os"
	"testing"
)

func TestIsAliveCurrentProcess(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestIsAliveInvalidPID(t *testing.T) {
	if IsAlive(0) {
		t.Error("PID 0 should not be alive")
	}
	if IsAlive(-1) {
		t.Error("PID -1 should not be alive")
	}
}

func TestIsAliveDeadProcess(t *testing.T) {
	// PID 99999999 should not exist.
	if IsAlive(99999999) {
		t.Error("PID 99999999 should not be alive")
	}
}

func TestStopProcessInvalidPID(t *testing.T) {
	err := StopProcess(0)
	if err == nil {
		t.Fatal("expected error for PID 0")
	}

	err = StopProcess(-5)
	if err == nil {
		t.Fatal("expected error for negative PID")
	}
}

func TestStopProcessNonexistent(t *testing.T) {
	err := StopProcess(99999999)
	if err == nil {
		t.Fatal("expected error for nonexistent process")
	}
}
