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
	writePanic(&b, "boom", []byte("STACK"))
	out := b.String()
	if !strings.Contains(out, "boom") || !strings.Contains(out, "STACK") {
		t.Errorf("writePanic output missing value or stack: %q", out)
	}
}
