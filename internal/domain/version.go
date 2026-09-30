// Package domain is konto's shared kernel (ADR 0012): the value types and
// entities more than one bounded context speaks in, with their pure rules.
// It does no I/O and imports nothing of konto's.
package domain

// HledgerVersion identifies an installed hledger release, mirroring the
// `hledger X.Y[.Z]` form printed by `hledger --version`.
type HledgerVersion struct {
	Major, Minor, Patch int
}

// AtLeast reports whether v is no older than minimum, comparing major then minor.
func (v HledgerVersion) AtLeast(minimum HledgerVersion) bool {
	return v.Major > minimum.Major || (v.Major == minimum.Major && v.Minor >= minimum.Minor)
}
