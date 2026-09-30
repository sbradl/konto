package main

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/hledger"
	"github.com/sbradl/konto/internal/ui"
)

// main is the process entry point. It only wires the real collaborators into
// app, so it is excluded from coverage (see the cover task in mise.toml).
func main() {
	app{
		stderr:    os.Stderr,
		preflight: hledger.PreflightOnPath,
		program:   runProgram,
		exit:      os.Exit,
	}.run()
}

func runProgram() error {
	_, err := tea.NewProgram(ui.New()).Run()
	return err //nolint:wrapcheck // the program's own error is already self-describing
}
