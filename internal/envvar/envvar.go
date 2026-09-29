// Package envvar names the environment variables rimba reads, so each name is
// spelled once. It imports nothing, so any package can use it without a cycle.
package envvar

const (
	// Debug enables debug output and timing when set.
	Debug = "RIMBA_DEBUG"
	// Quiet suppresses advisory hints when set.
	Quiet = "RIMBA_QUIET"
	// NoObservability disables observability when set to any value.
	NoObservability = "RIMBA_NO_OBSERVABILITY"
	// TrustYes auto-approves the trust prompt.
	TrustYes = "RIMBA_TRUST_YES"
	// CowEligibleOverride pins the copy-on-write eligibility decision ("1" or "0")
	// for e2e tests; an internal test seam, not a user-facing knob.
	CowEligibleOverride = "RIMBA_COW_ELIGIBLE_OVERRIDE"
)
