package domain

// HledgerVersion identifies an installed hledger release, mirroring the
// `hledger X.Y[.Z]` form printed by `hledger --version`.
type HledgerVersion struct {
	Major, Minor, Patch int
}

// AtLeast reports whether v is no older than min, comparing major then minor.
func (v HledgerVersion) AtLeast(min HledgerVersion) bool {
	return v.Major > min.Major || (v.Major == min.Major && v.Minor >= min.Minor)
}
