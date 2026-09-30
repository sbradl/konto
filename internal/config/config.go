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
)

// Config is what konto needs to know before it starts.
type Config struct {
	JournalPath string
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
	for _, path := range []string{*file, env.Getenv("LEDGER_FILE")} {
		if path != "" {
			return Config{JournalPath: absolute(env.Cwd, path)}, nil
		}
	}
	return Config{JournalPath: filepath.Join(env.Home, ".hledger.journal")}, nil
}

func absolute(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}
