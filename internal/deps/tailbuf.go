package deps

// tailBuffer is an io.Writer that retains only the last outputTailCapBytes
// bytes written to it, discarding the oldest bytes first as new ones arrive.
// It's used to cap memory for subprocess output that's only ever surfaced as
// an error message's tail (manager.go's runInstall, hooks.go's
// RunPostCreateHooks) — a verbose install/hook command can write many MB
// even though only the last portion is ever shown.
type tailBuffer struct {
	buf []byte
}

// outputTailCapBytes bounds how many bytes of subprocess output tailBuffer
// retains.
const outputTailCapBytes = 64 * 1024

// Write appends p, dropping the oldest retained bytes first if the result
// would exceed outputTailCapBytes. It never returns an error.
func (t *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	switch {
	case n >= outputTailCapBytes:
		// p alone fills (or exceeds) the cap; keep only its tail and drop
		// everything previously retained.
		t.buf = append(t.buf[:0:0], p[n-outputTailCapBytes:]...)
	case len(t.buf)+n > outputTailCapBytes:
		keep := outputTailCapBytes - n
		copy(t.buf, t.buf[len(t.buf)-keep:])
		t.buf = append(t.buf[:keep], p...)
	default:
		t.buf = append(t.buf, p...)
	}
	return n, nil
}

// String returns the retained tail as a string.
func (t *tailBuffer) String() string {
	return string(t.buf)
}
