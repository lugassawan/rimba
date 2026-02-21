package operations

// ProgressFunc is called to report operation progress.
// CLI wires this to spinner updates; MCP passes nil.
type ProgressFunc func(message string)

// notify safely invokes fn if non-nil.
func notify(fn ProgressFunc, message string) {
	if fn != nil {
		fn(message)
	}
}
