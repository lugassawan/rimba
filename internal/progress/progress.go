package progress

import "fmt"

// Func is called to report operation progress.
// CLI wires this to spinner updates; MCP passes nil.
// Implementations must be safe to call from multiple goroutines concurrently —
// some callers (e.g. parallel dependency installs) invoke Func from worker pools.
type Func func(message string)

// Notify safely invokes fn if non-nil.
func Notify(fn Func, message string) {
	if fn != nil {
		fn(message)
	}
}

// Notifyf safely invokes fn with a formatted message if non-nil.
func Notifyf(fn Func, format string, args ...any) {
	if fn != nil {
		fn(fmt.Sprintf(format, args...))
	}
}
