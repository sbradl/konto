// Command konto is the terminal UI frontend for hledger. It is the composition
// root (ADR 0003): it wires config, adapters and use cases together.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/hledger"
)

func main() {
	run(os.Stderr, hledger.PreflightOnPath, os.Exit)
}

// run starts konto; on a startup failure it reports to stderr and calls exit(1),
// otherwise it returns normally.
func run(stderr io.Writer, preflight func() (domain.HledgerVersion, error), exit func(code int)) {
	if _, err := preflight(); err != nil {
		_, _ = fmt.Fprintf(stderr, "konto: %v\n", err)
		exit(1)
	}
}
