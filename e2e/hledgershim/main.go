//go:build e2e

// Command hledgershim stands in for the hledger binary during e2e runs. The
// harness puts it first on PATH as "hledger". Every invocation is appended to a
// log, with what hledger reads on stdin for `-f -`; the behaviour is steered by
// a JSON config file the harness may rewrite mid-scenario. Anything not faked is
// handed to the real hledger via exec (as a child fed the logged stdin for
// `-f -`), so stdin, stdout, stderr and the exit code stay untouched.
//
// Environment:
//
//	KONTO_E2E_SHIM_CONFIG  path to the JSON config (see shimconf.Config)
//	KONTO_E2E_SHIM_LOG     path to the invocation log (one JSON invocation per line)
//	KONTO_E2E_REAL_HLEDGER absolute path of the real hledger
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/sbradl/konto/e2e/shimconf"
)

// exitShimError marks a failure of the shim itself, never of hledger.
const exitShimError = 3

// invocation is one logged call: its arguments, and the journal text it reads on stdin with `-f -`.
type invocation struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin,omitempty"`
}

func main() {
	call := invoked()
	args := call.Args
	if err := record(os.Getenv("KONTO_E2E_SHIM_LOG"), call); err != nil {
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
	case cfg.FailCSV != "" && slices.Contains(args, "--rules-file"):
		fmt.Fprintln(os.Stderr, cfg.FailCSV)
		os.Exit(1)
	case slices.ContainsFunc(cfg.GarbageFor, func(c string) bool { return slices.Contains(args, c) }):
		fmt.Println("}{ not json, not hledger ]]")
		return
	}

	handOver(os.Getenv("KONTO_E2E_REAL_HLEDGER"), call)
}

// invoked is this call: its arguments, and for `-f -` the stdin, read up.
func invoked() invocation {
	call := invocation{Args: os.Args[1:]}
	if readsStdin(call.Args) {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			fail(fmt.Errorf("read stdin: %w", err))
		}
		call.Stdin = string(input)
	}
	return call
}

// handOver runs the real hledger in the shim's place; a call whose stdin the shim read up gets it fed.
func handOver(realHledger string, call invocation) {
	if readsStdin(call.Args) {
		os.Exit(runFed(realHledger, call.Args, call.Stdin))
	}
	err := syscall.Exec(realHledger, append([]string{realHledger}, call.Args...), os.Environ())
	fail(fmt.Errorf("exec real hledger: %w", err))
}

// readsStdin reports whether hledger reads its journal from stdin: `-f -`.
func readsStdin(args []string) bool {
	for i := range len(args) - 1 {
		if args[i] == "-f" && args[i+1] == "-" {
			return true
		}
	}
	return false
}

// runFed runs the real hledger with args and input on stdin, and returns its exit code.
func runFed(bin string, args []string, input string) int {
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(input), os.Stdout, os.Stderr
	err := cmd.Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	if err != nil {
		fail(fmt.Errorf("run real hledger: %w", err))
	}
	return 0
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hledgershim:", err)
	os.Exit(exitShimError)
}

func record(path string, call invocation) (err error) {
	if path == "" {
		return nil
	}
	line, err := json.Marshal(call)
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

func load(path string) (shimconf.Config, error) {
	var cfg shimconf.Config
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
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // a field the harness writes but the shim lacks would be ignored silently
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
