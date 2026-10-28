package config

import "testing"

const (
	home = "/home/alex"
	cwd  = "/work"
)

func env(vars map[string]string) Environment {
	return Environment{Getenv: func(key string) string { return vars[key] }, Home: home, Cwd: cwd}
}

func mustResolve(t *testing.T, args []string, vars map[string]string) Config {
	t.Helper()
	cfg, err := Resolve(args, env(vars))
	if err != nil {
		t.Fatalf("Resolve(%v) failed: %v", args, err)
	}
	return cfg
}

// With nothing set, the journal is ~/.hledger.journal.
func TestJournalDefaultsToHomeHledgerJournal(t *testing.T) {
	cfg := mustResolve(t, nil, nil)

	if want := "/home/alex/.hledger.journal"; cfg.JournalPath != want {
		t.Errorf("JournalPath = %q, want %q", cfg.JournalPath, want)
	}
}

// $LEDGER_FILE beats the home default.
func TestLedgerFileEnvBeatsDefault(t *testing.T) {
	cfg := mustResolve(t, nil, map[string]string{"LEDGER_FILE": "/data/env.journal"})

	if want := "/data/env.journal"; cfg.JournalPath != want {
		t.Errorf("JournalPath = %q, want %q", cfg.JournalPath, want)
	}
}

// An empty $LEDGER_FILE counts as unset.
func TestEmptyLedgerFileEnvIsIgnored(t *testing.T) {
	cfg := mustResolve(t, nil, map[string]string{"LEDGER_FILE": ""})

	if want := "/home/alex/.hledger.journal"; cfg.JournalPath != want {
		t.Errorf("JournalPath = %q, want %q", cfg.JournalPath, want)
	}
}

// --file beats $LEDGER_FILE, in both the spaced and the = spelling.
func TestFileFlagBeatsEnv(t *testing.T) {
	vars := map[string]string{"LEDGER_FILE": "/data/env.journal"}
	for _, args := range [][]string{{"--file", "/data/flag.journal"}, {"--file=/data/flag.journal"}} {
		cfg := mustResolve(t, args, vars)

		if want := "/data/flag.journal"; cfg.JournalPath != want {
			t.Errorf("%v: JournalPath = %q, want %q", args, cfg.JournalPath, want)
		}
	}
}

// A bad command line is refused with the reason.
func TestResolveRejectsBadArguments(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown flag":     {"--nope"},
		"flag without arg": {"--file"},
		"stray argument":   {"extra"},
	} {
		_, err := Resolve(args, env(nil))

		if err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// A relative path, whichever source it comes from, is resolved against the
// working directory and cleaned.
func TestRelativeJournalPathBecomesAbsolute(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		vars map[string]string
	}{
		"flag": {[]string{"--file", "./sub/../j.journal"}, nil},
		"env":  {nil, map[string]string{"LEDGER_FILE": "./j.journal"}},
	} {
		cfg := mustResolve(t, tc.args, tc.vars)

		if want := "/work/j.journal"; cfg.JournalPath != want {
			t.Errorf("%s: JournalPath = %q, want %q", name, cfg.JournalPath, want)
		}
	}
}

// The home directory is kept, whichever journal path wins, for what else starts from it.
func TestResolveKeepsTheHomeDirectory(t *testing.T) {
	for _, args := range [][]string{nil, {"--file", "/data/flag.journal"}} {
		if cfg := mustResolve(t, args, nil); cfg.Home != "/home/alex" {
			t.Errorf("%v: Home = %q, want /home/alex", args, cfg.Home)
		}
	}
}
