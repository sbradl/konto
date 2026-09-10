package hledger

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// versionPattern matches the "hledger X.Y[.Z]" prefix of `hledger --version`
// output, ignoring any trailing platform suffix.
var versionPattern = regexp.MustCompile(`^hledger (\d+)\.(\d+)(?:\.(\d+))?`)

// minVersion is the oldest hledger release konto supports.
var minVersion = domain.HledgerVersion{Major: 1, Minor: 32}

// Preflight verifies that the hledger binary is available before konto starts.
func Preflight(lookPath func(name string) (string, error), runVersion func() (string, error)) (domain.HledgerVersion, error) {
	if _, err := lookPath("hledger"); err != nil {
		return domain.HledgerVersion{}, fmt.Errorf("hledger binary not found on PATH: %w", err)
	}
	out, err := runVersion()
	if err != nil {
		return domain.HledgerVersion{}, fmt.Errorf("running hledger --version: %w", err)
	}
	version, err := parseVersion(out)
	if err != nil {
		return domain.HledgerVersion{}, err
	}
	if !version.AtLeast(minVersion) {
		return domain.HledgerVersion{}, fmt.Errorf("hledger %d.%d is too old; konto needs %d.%d or newer", version.Major, version.Minor, minVersion.Major, minVersion.Minor)
	}
	return version, nil
}

// parseVersion extracts the version components from `hledger --version` output
// of the form "hledger X.Y[.Z]".
func parseVersion(out string) (domain.HledgerVersion, error) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return domain.HledgerVersion{}, fmt.Errorf("unrecognised hledger version %q", out)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return domain.HledgerVersion{Major: major, Minor: minor, Patch: patch}, nil
}
