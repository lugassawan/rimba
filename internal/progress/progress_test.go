package progress

import "testing"

func TestNotifyWithFunc(t *testing.T) {
	var got string
	fn := Func(func(msg string) { got = msg })
	Notify(fn, "hello")
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestNotifyNil(t *testing.T) {
	// Must not panic.
	Notify(nil, "hello")
}

func TestNotifyfWithFunc(t *testing.T) {
	var got string
	fn := Func(func(msg string) { got = msg })
	Notifyf(fn, "%s (%d/%d)", "node_modules", 1, 2)
	want := "node_modules (1/2)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestNotifyfNil(t *testing.T) {
	// Must not panic.
	Notifyf(nil, "%s (%d/%d)", "node_modules", 1, 2)
}
