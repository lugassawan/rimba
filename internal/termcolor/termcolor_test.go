package termcolor

import (
	"os"
	"testing"
)

func TestPainterEnabled(t *testing.T) {
	p := &Painter{disabled: false}

	got := p.Paint("hello", Green)
	want := "\033[32mhello\033[0m"
	if got != want {
		t.Errorf("Paint() = %q, want %q", got, want)
	}
}

func TestPainterMultipleColors(t *testing.T) {
	p := &Painter{disabled: false}

	got := p.Paint("hello", Bold, Red)
	want := "\033[1m\033[31mhello\033[0m"
	if got != want {
		t.Errorf("Paint() = %q, want %q", got, want)
	}
}

func TestPainterDisabled(t *testing.T) {
	p := &Painter{disabled: true}

	got := p.Paint("hello", Green)
	if got != "hello" {
		t.Errorf("disabled Paint() = %q, want %q", got, "hello")
	}
}

func TestPainterNoColors(t *testing.T) {
	p := &Painter{disabled: false}

	got := p.Paint("hello")
	if got != "hello" {
		t.Errorf("Paint() with no colors = %q, want %q", got, "hello")
	}
}

func TestNewPainterForceDisable(t *testing.T) {
	p := NewPainter(true)
	got := p.Paint("x", Red)
	if got != "x" {
		t.Errorf("forceDisable Paint() = %q, want %q", got, "x")
	}
}

func TestNewPainterWithNoColorEnv(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	p := NewPainter(false)
	got := p.Paint("x", Red)
	if got != "x" {
		t.Errorf("NO_COLOR Paint() = %q, want %q", got, "x")
	}
}

func TestNewPainterEnabled(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		}
	}()

	p := NewPainter(false)
	got := p.Paint("x", Red)
	// Should have ANSI codes since forceDisable=false and NO_COLOR is unset
	if got == "x" {
		t.Error("expected ANSI-colored output when NO_COLOR is unset and forceDisable=false")
	}
}
