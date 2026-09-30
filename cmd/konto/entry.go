package main

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/hledger"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/ui"
)

// main is the process entry point. It only wires the real collaborators into
// app, so it is excluded from coverage (see the cover task in mise.toml).
func main() {
	app{
		stderr: os.Stderr,
		resolve: func() (config.Config, error) {
			return resolveConfig(os.Args[1:], sysEnv{getenv: os.Getenv, userHomeDir: os.UserHomeDir, getwd: os.Getwd})
		},
		preflight: hledger.PreflightOnPath,
		lock:      lockJournal,
		program:   runProgram,
		exit:      os.Exit,
	}.run()
}

func runProgram(cfg config.Config) error {
	loader := ledger.NewLoader(hledger.New("hledger", cfg.JournalPath))
	_, err := tea.NewProgram(ui.New(cfg.JournalPath, loader)).Run()
	return err //nolint:wrapcheck // the program's own error is already self-describing
}
