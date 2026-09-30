package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/journal"
)

type outcome struct {
	stderr bytes.Buffer
	exit   int // -1 when exit was not called
	ran    bool
	cfg    config.Config
	events []string // "lock", "run", "release" in the order they happened
}

func passingPreflight() (domain.HledgerVersion, error) {
	return domain.HledgerVersion{Major: 1, Minor: 32}, nil
}

func passingConfig() (config.Config, error) {
	return config.Config{JournalPath: "/data/main.journal"}, nil
}

func start(preflight func() (domain.HledgerVersion, error), program error) *outcome {
	return startWith(passingConfig, preflight, program, nil)
}

func startWith(
	resolve func() (config.Config, error),
	preflight func() (domain.HledgerVersion, error),
	program error,
	lockErr error,
) *outcome {
	got := &outcome{exit: -1}
	a := app{
		stderr:    &got.stderr,
		resolve:   resolve,
		preflight: preflight,
		lock: func(path string) (func() error, error) {
			got.events = append(got.events, "lock "+path)
			return func() error { got.events = append(got.events, "release"); return nil }, lockErr
		},
		program: func(cfg config.Config) error {
			got.ran = true
			got.cfg = cfg
			got.events = append(got.events, "run")
			return program
		},
		exit: func(code int) { got.exit = code },
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

// The UI is started with the resolved configuration.
func TestRunPassesResolvedConfigToUI(t *testing.T) {
	got := start(passingPreflight, nil)

	if want := "/data/main.journal"; got.cfg.JournalPath != want {
		t.Errorf("UI got journal %q, want %q", got.cfg.JournalPath, want)
	}
}

// A bad command line is reported on stderr with exit 1, before anything starts.
func TestRunReportsConfigFailureAndExitsOne(t *testing.T) {
	failing := func() (config.Config, error) { return config.Config{}, errors.New("invalid arguments: --nope") }
	preflight := func() (domain.HledgerVersion, error) {
		t.Error("preflight must not run after a config failure")
		return domain.HledgerVersion{}, nil
	}

	got := startWith(failing, preflight, nil, nil)

	if got.exit != 1 || got.ran {
		t.Errorf("got exit %d, ui ran %v; want exit 1 and no UI", got.exit, got.ran)
	}
	if !strings.Contains(got.stderr.String(), "--nope") {
		t.Errorf("stderr should carry the cause, got: %q", got.stderr.String())
	}
}

func system(home, cwd string) sysEnv {
	return sysEnv{
		getenv:      func(string) string { return "" },
		userHomeDir: func() (string, error) { return home, nil },
		getwd:       func() (string, error) { return cwd, nil },
	}
}

// The journal path is resolved against the user's home directory.
func TestResolveConfigUsesHomeDirectory(t *testing.T) {
	cfg, err := resolveConfig(nil, system("/home/alex", "/work"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/home/alex/.hledger.journal"; cfg.JournalPath != want {
		t.Errorf("JournalPath = %q, want %q", cfg.JournalPath, want)
	}
}

// A relative --file is resolved against the working directory.
func TestResolveConfigUsesWorkingDirectory(t *testing.T) {
	cfg, err := resolveConfig([]string{"--file", "j.journal"}, system("/home/alex", "/work"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/work/j.journal"; cfg.JournalPath != want {
		t.Errorf("JournalPath = %q, want %q", cfg.JournalPath, want)
	}
}

// Without a home or working directory konto cannot resolve its journal.
func TestResolveConfigFailsWithoutDirectories(t *testing.T) {
	noHome := system("", "/work")
	noHome.userHomeDir = func() (string, error) { return "", errors.New("no $HOME") }
	noCwd := system("/home/alex", "")
	noCwd.getwd = func() (string, error) { return "", errors.New("no cwd") }

	for cause, sys := range map[string]sysEnv{"no $HOME": noHome, "no cwd": noCwd} {
		_, err := resolveConfig(nil, sys)

		if err == nil || !strings.Contains(err.Error(), cause) {
			t.Errorf("expected an error carrying %q, got: %v", cause, err)
		}
	}
}

// A bad command line surfaces as a configuration error.
func TestResolveConfigRejectsBadArguments(t *testing.T) {
	_, err := resolveConfig([]string{"--nope"}, system("/home/alex", "/work"))

	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("expected an error naming the bad flag, got: %v", err)
	}
}

// The journal is locked for the whole run and released when the UI ends.
func TestRunHoldsJournalLockWhileUIRuns(t *testing.T) {
	got := start(passingPreflight, nil)

	want := []string{"lock /data/main.journal", "run", "release"}
	if !reflect.DeepEqual(got.events, want) {
		t.Errorf("events = %v, want %v", got.events, want)
	}
}

// A journal already in use by another konto is reported with exit 1, without a UI.
func TestRunRefusesWhenJournalIsLocked(t *testing.T) {
	got := startWith(passingConfig, passingPreflight, nil, errors.New("konto is already running on this journal"))

	if got.exit != 1 || got.ran {
		t.Errorf("got exit %d, ui ran %v; want exit 1 and no UI", got.exit, got.ran)
	}
	if !strings.Contains(got.stderr.String(), "already running") {
		t.Errorf("stderr should say konto is already running, got: %q", got.stderr.String())
	}
}

// A UI failure still releases the lock before konto exits.
func TestRunReleasesLockWhenUIFails(t *testing.T) {
	got := start(passingPreflight, errors.New("no tty"))

	if !reflect.DeepEqual(got.events, []string{"lock /data/main.journal", "run", "release"}) {
		t.Errorf("events = %v, want the lock released after run", got.events)
	}
}

// lockJournal holds the journal until the returned release is called.
func TestLockJournalHoldsUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.journal")
	release, err := lockJournal(path)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}

	if _, err := lockJournal(path); !errors.Is(err, journal.ErrAlreadyRunning) {
		t.Errorf("second lock error = %v, want ErrAlreadyRunning", err)
	}

	if err := release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	again, err := lockJournal(path)
	if err != nil {
		t.Fatalf("lock after release failed: %v", err)
	}
	_ = again()
}
