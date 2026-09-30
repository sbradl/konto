package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

// A failed preflight prints the problem to stderr and exits 1.
func TestRunReportsPreflightFailureAndExitsOne(t *testing.T) {
	var stderr bytes.Buffer
	preflight := func() (domain.HledgerVersion, error) {
		return domain.HledgerVersion{}, errors.New("hledger binary not found on PATH (konto needs hledger 1.32 or newer)")
	}
	got := -1

	run(&stderr, preflight, func(code int) { got = code })

	if got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
	for _, want := range []string{"hledger", "1.32"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr should mention %q, got: %q", want, stderr.String())
		}
	}
}

// A passing preflight prints nothing and does not exit.
func TestRunDoesNotExitWhenPreflightPasses(t *testing.T) {
	var stderr bytes.Buffer
	preflight := func() (domain.HledgerVersion, error) {
		return domain.HledgerVersion{Major: 1, Minor: 32}, nil
	}

	run(&stderr, preflight, func(code int) { t.Errorf("unexpected exit(%d)", code) })

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

// main wires the real preflight: with a recent hledger on PATH it returns
// without exiting.
func TestMainReturnsWithRecentHledger(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'hledger 1.32'\n"
	if err := os.WriteFile(filepath.Join(dir, "hledger"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	main()
}
