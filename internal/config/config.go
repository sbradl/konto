// Package config resolves konto's start-up settings. konto has no config
// file in v1 (spec §7): the journal path comes from --file, $LEDGER_FILE or
// the home default, in that order.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Config is what konto needs to know before it starts.
type Config struct {
	JournalPath string
	Home        string // the user's home directory, as resolved once at start-up
}

// Environment is the process context the journal path depends on.
type Environment struct {
	Getenv func(key string) string
	Home   string // the user's home directory
	Cwd    string // the working directory relative paths resolve against
}

// Resolve reads the command line (without the program name) and environment.
// The journal path it returns is always absolute.
func Resolve(args []string, env Environment) (Config, error) {
	flags := flag.NewFlagSet("konto", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	file := flags.String("file", "", "journal file")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if flags.NArg() > 0 {
		return Config{}, errors.New("invalid arguments: unexpected " + flags.Arg(0))
	}

	return Config{JournalPath: journalPath(*file, env), Home: env.Home}, nil
}

// journalPath is --file, else $LEDGER_FILE, else ~/.hledger.journal, always absolute.
func journalPath(flagged string, env Environment) string {
	for _, path := range []string{flagged, env.Getenv("LEDGER_FILE")} {
		if path != "" {
			return absolute(env.Cwd, expandHome(env.Home, path))
		}
	}

	return filepath.Join(env.Home, ".hledger.journal")
}

// expandHome puts the home directory in place of a leading ~ or ~/, as the shell would; ~user is left alone.
func expandHome(home, path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}

	return path
}

func absolute(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(cwd, path)
}
