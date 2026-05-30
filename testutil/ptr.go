package testutil

// Ptr returns a pointer to v — useful in tests for constructing *T literals inline.
// Uses Go 1.26 new(expr) form (module minimum ≥ 1.26.3); //go:fix inline lets
// tooling rewrite call-sites automatically.
//
//go:fix inline
func Ptr[T any](v T) *T { return new(v) }
