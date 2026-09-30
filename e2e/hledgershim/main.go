//go:build e2e

// Command hledgershim stands in for the hledger binary during e2e runs. The
// harness puts it first on PATH as "hledger". Every invocation is appended to a
// log; the behaviour is steered by a JSON config file the harness may rewrite
// mid-scenario. Anything not faked is handed to the real hledger via exec, so
// stdin, stdout, stderr and the exit code stay untouched.
//
// Environment:
//
//	KONTO_E2E_SHIM_CONFIG  path to the JSON config (see config)
//	KONTO_E2E_SHIM_LOG     path to the invocation log (one JSON array per line)
//	KONTO_E2E_REAL_HLEDGER absolute path of the real hledger
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"syscall"
	"time"
)

// exitShimError marks a failure of the shim itself, never of hledger.
const exitShimError = 3

type config struct {
	Version    string   `json:"version,omitempty"`    // fake `hledger --version` as "hledger <Version>"
	DelayMs    int      `json:"delayMs,omitempty"`    // sleep before every call
	Fail       string   `json:"fail,omitempty"`       // print to stderr and exit 1 (except --version)
	GarbageFor []string `json:"garbageFor,omitempty"` // print garbage for calls naming these commands
}

func main() {
	args := os.Args[1:]
	if err := record(os.Getenv("KONTO_E2E_SHIM_LOG"), args); err != nil {
		fail(err)
	}
	cfg, err := load(os.Getenv("KONTO_E2E_SHIM_CONFIG"))
	if err != nil {
		fail(err)
	}
	time.Sleep(time.Duration(cfg.DelayMs) * time.Millisecond)

	isVersion := slices.Contains(args, "--version")
	switch {
	case isVersion && cfg.Version != "":
		fmt.Printf("hledger %s, linux-x86_64\n", cfg.Version)
		return
	case cfg.Fail != "" && !isVersion:
		fmt.Fprintln(os.Stderr, cfg.Fail)
		os.Exit(1)
	case slices.ContainsFunc(cfg.GarbageFor, func(c string) bool { return slices.Contains(args, c) }):
		fmt.Println("}{ not json, not hledger ]]")
		return
	}

	realHledger := os.Getenv("KONTO_E2E_REAL_HLEDGER")
	err = syscall.Exec(realHledger, append([]string{realHledger}, args...), os.Environ())
	fail(fmt.Errorf("exec real hledger: %w", err))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hledgershim:", err)
	os.Exit(exitShimError)
}

func record(path string, args []string) (err error) {
	if path == "" {
		return nil
	}
	line, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode call: %w", err)
	}
	callLog, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open call log: %w", err)
	}
	defer func() {
		if cerr := callLog.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close call log: %w", cerr)
		}
	}()
	if _, err := callLog.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write call log: %w", err)
	}
	return nil
}

func load(path string) (config, error) {
	var cfg config
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
