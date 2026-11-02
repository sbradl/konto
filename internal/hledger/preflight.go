// Package hledger is the adapter to the hledger binary: it runs hledger and
// maps its output into domain types. It satisfies use-case ports structurally
// (ADR 0003).
package hledger

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// versionPattern matches the "hledger X.Y[.Z]" prefix of `hledger --version`
// output, ignoring any trailing platform suffix.
var versionPattern = regexp.MustCompile(`^hledger (\d+)\.(\d+)(?:\.(\d+))?`)

// Binary is the hledger executable konto runs, looked up on PATH: the one preflight checks and Client calls.
const Binary = "hledger"

// The oldest hledger release konto supports (spec §4.5).
const (
	minMajor = 1
	minMinor = 32
)

func minVersion() domain.HledgerVersion {
	return domain.HledgerVersion{Major: minMajor, Minor: minMinor}
}

// Preflight verifies that the hledger binary is available before konto starts.
func Preflight(
	lookPath func(name string) (string, error),
	runVersion func() (string, error),
) (domain.HledgerVersion, error) {
	if _, err := lookPath(Binary); err != nil {
		return domain.HledgerVersion{}, fmt.Errorf(
			"hledger binary not found on PATH (konto needs hledger %d.%d or newer): %w", minMajor, minMinor, err)
	}
	out, err := runVersion()
	if err != nil {
		return domain.HledgerVersion{}, fmt.Errorf("running hledger --version: %w", err)
	}
	version, err := parseVersion(out)
	if err != nil {
		return domain.HledgerVersion{}, err
	}
	if minimum := minVersion(); !version.AtLeast(minimum) {
		return domain.HledgerVersion{}, fmt.Errorf("hledger %d.%d is too old; konto needs %d.%d or newer",
			version.Major, version.Minor, minimum.Major, minimum.Minor)
	}
	return version, nil
}

// PreflightOnPath runs Preflight against Binary on PATH, with the time limit of any hledger call.
func PreflightOnPath() (domain.HledgerVersion, error) {
	return Preflight(exec.LookPath, func() (string, error) {
		out, _, err := callBinary(Binary).run(context.Background(), nil, "--version")
		return string(out), err
	})
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
