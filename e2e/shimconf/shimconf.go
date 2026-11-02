//go:build e2e

// Package shimconf is the hledgershim's config file, shared by the shim and the harness that writes it.
package shimconf

// Config steers the shim; the zero value hands every call to the real hledger.
type Config struct {
	Version    string   `json:"version,omitempty"`    // fake `hledger --version` as "hledger <Version>"
	DelayMs    int      `json:"delayMs,omitempty"`    // sleep before every call
	Fail       string   `json:"fail,omitempty"`       // print to stderr and exit 1 (except --version)
	GarbageFor []string `json:"garbageFor,omitempty"` // print garbage for calls naming these commands
	FailCSV    string   `json:"failCsv,omitempty"`    // print to stderr and exit 1 for CSV conversions only
}
