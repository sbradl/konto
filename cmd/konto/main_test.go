package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

type outcome struct {
	stderr bytes.Buffer
	exit   int // -1 when exit was not called
	ran    bool
}

func passingPreflight() (domain.HledgerVersion, error) {
	return domain.HledgerVersion{Major: 1, Minor: 32}, nil
}

func start(preflight func() (domain.HledgerVersion, error), program error) *outcome {
	got := &outcome{exit: -1}
	a := app{
		stderr:    &got.stderr,
		preflight: preflight,
		program:   func() error { got.ran = true; return program },
		exit:      func(code int) { got.exit = code },
	}
	a.run()
	return got
}

// A failed preflight prints the problem to stderr, exits 1 and starts no UI.
func TestRunReportsPreflightFailureAndExitsOne(t *testing.T) {
	failing := func() (domain.HledgerVersion, error) {
		return domain.HledgerVersion{}, errors.New("hledger binary not found on PATH (konto needs hledger 1.32 or newer)")
	}

	got := start(failing, nil)

	if got.exit != 1 {
		t.Errorf("exit code = %d, want 1", got.exit)
	}
	if got.ran {
		t.Error("the UI must not start after a failed preflight")
	}
	for _, want := range []string{"hledger", "1.32"} {
		if !strings.Contains(got.stderr.String(), want) {
			t.Errorf("stderr should mention %q, got: %q", want, got.stderr.String())
		}
	}
}

// A passing preflight starts the UI; when the user quits, konto ends quietly.
func TestRunStartsUIAfterPreflight(t *testing.T) {
	got := start(passingPreflight, nil)

	if !got.ran {
		t.Error("the UI should start after a passing preflight")
	}
	if got.exit != -1 || got.stderr.Len() != 0 {
		t.Errorf("got exit %d and stderr %q, want neither", got.exit, got.stderr.String())
	}
}

// A UI that fails is reported on stderr with exit 1.
func TestRunReportsUIFailureAndExitsOne(t *testing.T) {
	got := start(passingPreflight, errors.New("no tty"))

	if got.exit != 1 {
		t.Errorf("exit code = %d, want 1", got.exit)
	}
	if !strings.Contains(got.stderr.String(), "no tty") {
		t.Errorf("stderr should carry the cause, got: %q", got.stderr.String())
	}
}
