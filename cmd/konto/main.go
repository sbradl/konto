// Command konto is the terminal UI frontend for hledger. It is the composition
// root (ADR 0003): it wires config, adapters and use cases together.
package main

import (
	"fmt"
	"io"

	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/domain"
)

// app is konto's start-up sequence with its collaborators injected.
type app struct {
	stderr    io.Writer
	resolve   func() (config.Config, error)
	preflight func() (domain.HledgerVersion, error)
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
	if err := a.program(cfg); err != nil {
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
