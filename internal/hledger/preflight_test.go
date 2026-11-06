package hledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

func okLookPath(string) (string, error) { return "/usr/bin/hledger", nil }

// konto refuses to start when the hledger binary is not on PATH.
func TestPreflightRefusesWhenHledgerNotOnPath(t *testing.T) {
	lookPath := func(string) (string, error) {
		return "", errors.New("exec: \"hledger\": executable file not found in $PATH")
	}
	runVersion := func() (string, error) {
		t.Fatal("runVersion must not be called when hledger is not on PATH")

		return "", nil
	}

	_, err := Preflight(lookPath, runVersion)

	if err == nil {
		t.Fatal("expected an error when hledger is not on PATH, got nil")
	}

	if !strings.Contains(err.Error(), "hledger") {
		t.Errorf("error should identify hledger as not found, got: %v", err)
	}

	if !strings.Contains(err.Error(), "1.32") {
		t.Errorf("error should name the minimum version 1.32, got: %v", err)
	}
}

// konto refuses to start when `hledger --version` is on PATH but fails to run.
func TestPreflightRefusesWhenVersionCommandFails(t *testing.T) {
	runVersion := func() (string, error) {
		return "", errors.New("exec: signal: killed")
	}

	_, err := Preflight(okLookPath, runVersion)

	if err == nil {
		t.Fatal("expected an error when hledger --version fails to run, got nil")
	}

	if !strings.Contains(err.Error(), "killed") {
		t.Errorf("error should mention the underlying cause, got: %v", err)
	}
}

// konto refuses to start when `hledger --version` runs but its output carries
// no recognisable version number.
func TestPreflightRefusesWhenVersionUnrecognisable(t *testing.T) {
	runVersion := func() (string, error) {
		return "hledger banana\n", nil
	}

	_, err := Preflight(okLookPath, runVersion)

	if err == nil {
		t.Fatal("expected an error when hledger --version output has no version number, got nil")
	}

	if !strings.Contains(err.Error(), "banana") {
		t.Errorf("error should quote the unrecognised output, got: %v", err)
	}
}

// Preflight reports the installed hledger version.
func TestPreflightReportsInstalledVersion(t *testing.T) {
	tests := []struct {
		name       string
		versionOut string
		want       domain.HledgerVersion
	}{
		{"supported release", "hledger 1.32\n", domain.HledgerVersion{Major: 1, Minor: 32}},
		{"reports the actual major/minor/patch", "hledger 1.40.2\n", domain.HledgerVersion{Major: 1, Minor: 40, Patch: 2}},
		{"newer major version", "hledger 2.0\n", domain.HledgerVersion{Major: 2, Minor: 0, Patch: 0}},
		{
			"real --version output with a platform suffix", "hledger 1.32.2, linux-x86_64\n",
			domain.HledgerVersion{Major: 1, Minor: 32, Patch: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Preflight(okLookPath, versionOutput(tt.versionOut))
			if err != nil {
				t.Fatalf("expected no error for a supported hledger, got: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected version %+v, got %+v", tt.want, got)
			}
		})
	}
}

// Preflight refuses a release older than the supported minimum, naming both.
func TestPreflightRefusesTooOldVersion(t *testing.T) {
	tests := []struct {
		name       string
		versionOut string
		installed  string
	}{
		{"older than the 1.32 minimum", "hledger 1.31\n", "1.31"},
		{"major version below the minimum", "hledger 0.99\n", "0.99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Preflight(okLookPath, versionOutput(tt.versionOut))
			if err == nil {
				t.Fatalf("expected an error for %q, got nil", tt.versionOut)
			}
			for _, want := range []string{tt.installed, "1.32"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error should name %q, got: %v", want, err)
				}
			}
		})
	}
}

func versionOutput(out string) func() (string, error) {
	return func() (string, error) { return out, nil }
}

// fakeHledger puts an executable "hledger" that prints stdout and exits with
// code on a PATH containing nothing else.
func fakeHledger(t *testing.T, stdout string, code int) {
	t.Helper()
	dir := t.TempDir()

	script := fmt.Sprintf("#!/bin/sh\necho '%s'\nexit %d\n", stdout, code)
	if err := os.WriteFile(filepath.Join(dir, "hledger"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// konto refuses to start when no hledger is on PATH, naming the minimum.
func TestPreflightOnPathRefusesWithoutHledger(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := PreflightOnPath()

	if err == nil || !strings.Contains(err.Error(), "1.32") {
		t.Fatalf("expected a not-found error naming 1.32, got: %v", err)
	}
}

// konto starts when the hledger on PATH is recent enough.
func TestPreflightOnPathAcceptsRecentHledger(t *testing.T) {
	fakeHledger(t, "hledger 1.32", 0)

	version, err := PreflightOnPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := (domain.HledgerVersion{Major: 1, Minor: 32}); version != want {
		t.Errorf("version = %+v, want %+v", version, want)
	}
}

// konto refuses to start when the hledger on PATH is too old.
func TestPreflightOnPathRefusesOldHledger(t *testing.T) {
	fakeHledger(t, "hledger 1.31", 0)

	_, err := PreflightOnPath()

	if err == nil || !strings.Contains(err.Error(), "1.32") {
		t.Fatalf("expected a too-old error naming 1.32, got: %v", err)
	}
}

// konto refuses to start when `hledger --version` exits non-zero.
func TestPreflightOnPathRefusesWhenVersionCommandFails(t *testing.T) {
	fakeHledger(t, "boom", 3)

	_, err := PreflightOnPath()

	if err == nil {
		t.Fatal("expected an error when hledger --version exits non-zero, got nil")
	}
}
