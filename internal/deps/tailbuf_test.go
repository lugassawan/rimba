package deps

import (
	"strings"
	"testing"
)

func TestTailBufferUnderCapPreservesAll(t *testing.T) {
	var buf tailBuffer

	writes := []string{"first ", "second ", "third"}
	for _, w := range writes {
		n, err := buf.Write([]byte(w))
		if err != nil {
			t.Fatalf("Write(%q) error = %v", w, err)
		}
		if n != len(w) {
			t.Errorf("Write(%q) n = %d, want %d", w, n, len(w))
		}
	}

	want := strings.Join(writes, "")
	if got := buf.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTailBufferOverCapDropsOldestFirst(t *testing.T) {
	var buf tailBuffer

	// Write more than the cap across multiple small writes, byte by byte,
	// using a marker so we can assert exactly which bytes survive.
	oldest := strings.Repeat("A", outputTailCapBytes/2)
	newest := strings.Repeat("B", outputTailCapBytes)

	if _, err := buf.Write([]byte(oldest)); err != nil {
		t.Fatalf("Write(oldest) error = %v", err)
	}
	if _, err := buf.Write([]byte(newest)); err != nil {
		t.Fatalf("Write(newest) error = %v", err)
	}

	got := buf.String()
	if len(got) != outputTailCapBytes {
		t.Fatalf("String() len = %d, want %d", len(got), outputTailCapBytes)
	}
	want := newest[len(newest)-outputTailCapBytes:]
	if got != want {
		t.Error("String() did not equal the exact expected tail")
	}
	if strings.Contains(got, "A") {
		t.Error("String() contains bytes from the oldest write, want them dropped")
	}
}

func TestTailBufferOverCapByteExactTail(t *testing.T) {
	var buf tailBuffer

	// Write outputTailCapBytes+100 distinguishable bytes across many small
	// (but not single-byte, to keep the test fast) writes, so we can assert
	// the exact surviving byte range (not just length/prefix character), and
	// that the earliest bytes were dropped.
	const chunkSize = 4096
	total := outputTailCapBytes + 100
	all := make([]byte, total)
	for i := range all {
		all[i] = byte('a' + i%26)
	}
	for offset := 0; offset < total; offset += chunkSize {
		end := min(offset+chunkSize, total)
		if _, err := buf.Write(all[offset:end]); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	got := buf.String()
	want := string(all[total-outputTailCapBytes:])
	if got != want {
		t.Error("String() did not byte-exactly match the expected tail after chunked writes")
	}
}

func TestTailBufferSingleWriteLargerThanCap(t *testing.T) {
	var buf tailBuffer

	oversized := make([]byte, outputTailCapBytes+50)
	for i := range oversized {
		oversized[i] = byte('0' + i%10)
	}

	n, err := buf.Write(oversized)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(oversized) {
		t.Errorf("Write() n = %d, want %d (io.Writer contract: n == len(p) even when truncating internally)", n, len(oversized))
	}

	got := buf.String()
	want := string(oversized[len(oversized)-outputTailCapBytes:])
	if got != want {
		t.Error("String() did not equal the tail of the oversized single write")
	}
	if len(got) != outputTailCapBytes {
		t.Fatalf("String() len = %d, want %d", len(got), outputTailCapBytes)
	}
}

func TestTailBufferSingleWriteExactlyAtCap(t *testing.T) {
	var buf tailBuffer

	exact := strings.Repeat("x", outputTailCapBytes)
	if _, err := buf.Write([]byte(exact)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := buf.String(); got != exact {
		t.Error("String() did not equal the full write when write size == cap")
	}
}

func TestTailBufferZeroWritesEmptyNoPanic(t *testing.T) {
	var buf tailBuffer

	if got := buf.String(); got != "" {
		t.Errorf("String() = %q, want empty string for zero writes", got)
	}
}

func TestTailBufferMultipleWritesAcrossCapBoundary(t *testing.T) {
	var buf tailBuffer

	// A sequence of writes that individually stay under the cap, but whose
	// cumulative total crosses it more than once, to exercise the
	// shift-and-append path repeatedly.
	chunk := outputTailCapBytes / 4
	labels := []byte{'1', '2', '3', '4', '5', '6'}
	for _, label := range labels {
		if _, err := buf.Write([]byte(strings.Repeat(string(label), chunk))); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	got := buf.String()
	if len(got) != outputTailCapBytes {
		t.Fatalf("String() len = %d, want %d", len(got), outputTailCapBytes)
	}
	// Only the last 4 chunks (labels 3,4,5,6) fit within the cap.
	want := strings.Repeat("3", chunk) + strings.Repeat("4", chunk) + strings.Repeat("5", chunk) + strings.Repeat("6", chunk)
	if got != want {
		t.Error("String() did not equal the expected last-N-chunks tail")
	}
}
