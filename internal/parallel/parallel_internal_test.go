package parallel

import (
	"bytes"
	"strings"
	"testing"
)

//go:noinline
func panicInWorker() { panic("stack probe") }

func TestGroupCapturesWorkerStack(t *testing.T) {
	var g Group
	g.Go(panicInWorker)
	g.wg.Wait() // not Wait(): that would re-raise

	if !strings.Contains(string(g.p.stack), "panicInWorker") {
		t.Errorf("captured stack does not name the panicking worker:\n%s", g.p.stack)
	}
}

func TestWritePanicIncludesValueAndStack(t *testing.T) {
	var b bytes.Buffer
	writePanic(&b, "my label", "boom", []byte("STACK"))
	out := b.String()
	for _, want := range []string{"my label", "boom", "STACK"} {
		if !strings.Contains(out, want) {
			t.Errorf("writePanic output missing %q: %q", want, out)
		}
	}
}
