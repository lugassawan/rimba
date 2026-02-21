package operations

import "testing"

func TestNotify_WithFunc(t *testing.T) {
	var got string
	fn := ProgressFunc(func(msg string) { got = msg })
	notify(fn, "hello")
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestNotify_Nil(t *testing.T) {
	// Must not panic.
	notify(nil, "hello")
}
