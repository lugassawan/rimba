package termcolor

import (
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
