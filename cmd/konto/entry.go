package main

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/hledger"
	"github.com/sbradl/konto/internal/ui"
)

// main is the process entry point. It only hands the real process (arguments, environment, clock, terminal,
// exit) to app and buildPorts, so it is excluded from coverage (see the cover task in mise.toml).
func main() {
	app{
		stderr: os.Stderr,
		resolve: func() (config.Config, error) {
			return resolveConfig(os.Args[1:], sysEnv{getenv: os.Getenv, userHomeDir: os.UserHomeDir, getwd: os.Getwd})
		},
		preflight: hledger.PreflightOnPath,
		lock:      lockJournal,
		program: func(cfg config.Config) error {
			ports := buildPorts(cfg, os.Getenv, todayFunc(os.Getenv, time.Now))
			model, opts := withIdleSignal(ui.New(cfg.JournalPath, ports), os.Getenv, os.Stdout)
			_, err := tea.NewProgram(model, opts...).Run()

			return err //nolint:wrapcheck // the program's own error is already self-describing
		},
		exit: os.Exit,
	}.run()
}
