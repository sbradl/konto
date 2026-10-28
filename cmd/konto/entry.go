package main

import (
	"os"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/accounts"
	"github.com/sbradl/konto/internal/carry"
	"github.com/sbradl/konto/internal/checks"
	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/files"
	"github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/hledger"
	"github.com/sbradl/konto/internal/journal"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/report"
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
	today := todayFunc(os.Getenv, time.Now)
	file := journal.NewFile(cfg.JournalPath, today)
	rulesPath := file.RulesPath()
	client := hledger.New(hledger.Binary, file.Window)
	loader := ledger.NewLoader(client, file, today)
	writer := carry.NewWriter(file, client)
	_, err := tea.NewProgram(ui.New(cfg.JournalPath, ui.Ports{
		Loader: loader, Checker: checks.NewRunner(client), Creator: file, Editor: openInEditor,
		// The Verifier renders through the carry writer, not file: file satisfies the same port but would check
		// a write without the year carry-over it brings.
		Accounts: accounts.NewService(file, today), Writer: writer, Verifier: checks.NewVerifier(writer, client),
		Reports: report.NewService(client, today), Forecast: forecast.NewService(client, today),
		Rules:       periodic.NewService(client, today),
		Importer:    csvimport.NewService(files.System{}, client, rulesPath),
		DownloadDir: downloadDir(os.Getenv, cfg.Home),
	})).Run()
	return err //nolint:wrapcheck // the program's own error is already self-describing
}

func openInEditor(file string, line int) *exec.Cmd {
	return editorCommand(os.Getenv, file, line)
}
