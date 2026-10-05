package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
	file := journal.NewFile(cfg.JournalPath).WithToday(today)
	rulesPath := filepath.Join(filepath.Dir(cfg.JournalPath), "categories.rules")
	client := hledger.New("hledger", file.Window)
	loader := ledger.NewLoader(client, file, today)
	writer := carry.NewWriter(file, client)
	home, _ := os.UserHomeDir() // without a home the picker falls back to a relative Downloads
	_, err := tea.NewProgram(ui.New(cfg.JournalPath, ui.Ports{
		Loader: loader, Checker: checks.NewRunner(client), Creator: file, Editor: openInEditor,
		Accounts: accounts.NewService(file, today), Writer: writer, Verifier: checks.NewVerifier(writer, client),
		Reports: report.NewService(client, today), Forecast: forecast.NewService(client, today),
		Rules:    periodic.NewService(client, today),
		Importer: csvimport.NewService(files.System{}, client, rulesPath), DownloadDir: downloadDir(os.Getenv, home),
	})).Run()
	return err //nolint:wrapcheck // the program's own error is already self-describing
}

func openInEditor(file string, line int) *exec.Cmd {
	return editorCommand(os.Getenv, file, line)
}
