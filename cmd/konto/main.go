// Command konto is the terminal UI frontend for hledger. It is the composition
// root (ADR 0003): it wires config, adapters and use cases together.
package main

import (
	"fmt"
	"io"

	"github.com/sbradl/konto/internal/domain"
)

// app is konto's start-up sequence with its collaborators injected.
type app struct {
	stderr    io.Writer
	preflight func() (domain.HledgerVersion, error)
	program   func() error
	exit      func(code int)
}

// run checks hledger, then runs the UI; any failure is reported to stderr and
// ends the process with exit code 1.
func (a app) run() {
	if _, err := a.preflight(); err != nil {
		a.fail(err)
		return
	}
	if err := a.program(); err != nil {
		a.fail(err)
	}
}

func (a app) fail(err error) {
	_, _ = fmt.Fprintf(a.stderr, "konto: %v\n", err)
	a.exit(1)
}
