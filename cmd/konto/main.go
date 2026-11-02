// Command konto is the terminal UI frontend for hledger. It is the composition
// root (ADR 0003): it wires config, adapters and use cases together.
package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/accounts"
	"github.com/sbradl/konto/internal/carry"
	"github.com/sbradl/konto/internal/checks"
	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/files"
	"github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/hledger"
	"github.com/sbradl/konto/internal/journal"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/report"
	"github.com/sbradl/konto/internal/ui"
)

// app is konto's start-up sequence with its collaborators injected.
type app struct {
	stderr    io.Writer
	resolve   func() (config.Config, error)
	preflight func() (domain.HledgerVersion, error)
	lock      func(journalPath string) (release func() error, err error)
	program   func(config.Config) error
	exit      func(code int)
}

// run resolves the configuration, checks hledger, then runs the UI; any failure is reported to stderr and
// ends the process with exit code 1.
func (a app) run() {
	cfg, err := a.resolve()
	if err != nil {
		a.fail(err)
		return
	}
	if _, err := a.preflight(); err != nil {
		a.fail(err)
		return
	}
	release, err := a.lock(cfg.JournalPath)
	if err != nil {
		a.fail(err)
		return
	}
	err = a.program(cfg)
	_ = release()
	if err != nil {
		a.fail(err)
	}
}

func (a app) fail(err error) {
	_, _ = fmt.Fprintf(a.stderr, "konto: %v\n", err)
	a.exit(1)
}

// sysEnv is the operating-system context configuration is read from.
type sysEnv struct {
	getenv      func(key string) string
	userHomeDir func() (string, error)
	getwd       func() (string, error)
}

// resolveConfig reads the configuration from the command line and environment.
func resolveConfig(args []string, sys sysEnv) (config.Config, error) {
	home, err := sys.userHomeDir()
	if err != nil {
		return config.Config{}, fmt.Errorf("finding home directory: %w", err)
	}
	cwd, err := sys.getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("finding working directory: %w", err)
	}
	cfg, err := config.Resolve(args, config.Environment{Getenv: sys.getenv, Home: home, Cwd: cwd})
	if err != nil {
		return config.Config{}, fmt.Errorf("resolving configuration: %w", err)
	}
	return cfg, nil
}

// lockJournal takes the single-instance lock on the journal and returns the
// function that gives it up.
func lockJournal(journalPath string) (func() error, error) {
	lock, err := journal.Acquire(journalPath)
	if err != nil {
		return nil, fmt.Errorf("locking journal: %w", err)
	}
	return lock.Release, nil
}

// buildPorts wires the adapters for the journal at cfg.JournalPath into the use cases the UI drives; today is
// konto's today, getenv the environment.
func buildPorts(cfg config.Config, getenv func(string) string, today func() time.Time) ui.Ports {
	file := journal.NewFile(cfg.JournalPath, today)
	client := hledger.New(hledger.Binary, file.Window)
	return ui.Ports{
		Loader: ledger.NewLoader(client, file, today), Checker: checks.NewRunner(client), Creator: file,
		Editor:   func(path string, line int) *exec.Cmd { return editorCommand(getenv, path, line) },
		Accounts: accounts.NewService(file, today), Writer: carry.NewWriter(file, client),
		// The UI hands the Verifier the changeset the carry writer prepared, so it renders it through the very
		// file that writer writes to (TestBuildPortsVerifiesTheJournalTheWriterWrites).
		Verifier: checks.NewVerifier(file, client),
		Reports:  report.NewService(client, today), Forecast: forecast.NewService(client, today),
		Rules:       periodic.NewService(client, today),
		Importer:    csvimport.NewService(files.System{}, client, file.RulesPath()),
		DownloadDir: downloadDir(getenv, cfg.Home),
	}
}

// editorCommand opens file at line in the user's editor: $VISUAL, then
// $EDITOR, then vi (spec §4.7).
func editorCommand(getenv func(string) string, file string, line int) *exec.Cmd {
	editor := cmp.Or(strings.TrimSpace(getenv("VISUAL")), strings.TrimSpace(getenv("EDITOR")), "vi")
	words := strings.Fields(editor)
	args := slices.Concat(words[1:], []string{"+" + strconv.Itoa(line), file})
	//nolint:gosec // the user's own editor, by design
	return exec.CommandContext(context.Background(), words[0], args...)
}

// todayFunc is konto's "today": the local date now, or the day $KONTO_TODAY names (a test seam
// for the e2e suite). A value that is not a date falls back to the clock. Either way it is that calendar
// date at midnight UTC, as hledger's dates are read, so every date comparison is by day alone.
func todayFunc(getenv func(string) string, now func() time.Time) func() time.Time {
	return func() time.Time {
		if day, err := time.Parse(time.DateOnly, getenv("KONTO_TODAY")); err == nil {
			return day
		}
		year, month, day := now().Date()
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
}

// downloadDir is where the file picker starts: $XDG_DOWNLOAD_DIR, else ~/Downloads.
func downloadDir(getenv func(string) string, home string) string {
	if dir := strings.TrimSpace(getenv("XDG_DOWNLOAD_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(home, "Downloads")
}
