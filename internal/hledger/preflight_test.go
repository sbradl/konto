package hledger

import (
	"errors"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

func okLookPath(string) (string, error) { return "/usr/bin/hledger", nil }

// konto refuses to start when the hledger binary is not on PATH.
func TestPreflightRefusesWhenHledgerNotOnPath(t *testing.T) {
	lookPath := func(name string) (string, error) {
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

// Preflight reports the installed hledger version, and refuses a release older
// than the supported minimum.
func TestPreflightChecksInstalledVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionOut  string
		wantVersion domain.HledgerVersion
		wantErr     []string // substrings the refusal must name; nil means expect success
	}{
		{
			name:        "supported release",
			versionOut:  "hledger 1.32\n",
			wantVersion: domain.HledgerVersion{Major: 1, Minor: 32},
		},
		{
			name:        "reports the actual major/minor/patch",
			versionOut:  "hledger 1.40.2\n",
			wantVersion: domain.HledgerVersion{Major: 1, Minor: 40, Patch: 2},
		},
		{
			name:        "newer major version",
			versionOut:  "hledger 2.0\n",
			wantVersion: domain.HledgerVersion{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:        "real --version output with a platform suffix",
			versionOut:  "hledger 1.32.2, linux-x86_64\n",
			wantVersion: domain.HledgerVersion{Major: 1, Minor: 32, Patch: 2},
		},
		{
			name:       "older than the 1.32 minimum",
			versionOut: "hledger 1.31\n",
			wantErr:    []string{"1.31", "1.32"},
		},
		{
			name:       "major version below the minimum",
			versionOut: "hledger 0.99\n",
			wantErr:    []string{"0.99", "1.32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVersion := func() (string, error) { return tt.versionOut, nil }

			got, err := Preflight(okLookPath, runVersion)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected an error for %q, got nil", tt.versionOut)
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error should name %q, got: %v", want, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error for a supported hledger, got: %v", err)
			}
			if got != tt.wantVersion {
				t.Errorf("expected version %+v, got %+v", tt.wantVersion, got)
			}
		})
	}
}
