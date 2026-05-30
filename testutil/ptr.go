package testutil

// Ptr returns a pointer to v — useful in tests for constructing *T literals inline.
//
//go:fix inline
func Ptr[T any](v T) *T { return new(v) }
