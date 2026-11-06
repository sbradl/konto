//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The harness's assertion primitives: retried checks and the checks on the files konto writes.

// eventually retries check, once konto is idle, until it passes or the timeout ends, and returns its last
// failure. Waiting for idle first keeps a check for absence from passing before a wrongful write lands.
func (w *world) eventually(check func() error) error {
	s, err := w.session()
	if err != nil {
		return err
	}

	if err := w.awaitIdleIfRunning(); err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		failure = check()

		return failure == nil
	})

	return failure
}

// allJournalText is every journal file's text, joined.
func allJournalText(files map[string][]byte) string {
	var all strings.Builder
	for _, content := range files {
		all.Write(content)
		all.WriteByte('\n')
	}

	return all.String()
}

// expectFileBlock checks that the file contains want's lines, consecutive and whitespace-normalised.
func expectFileBlock(path, want string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if !containsBlock(string(b), want) {
		return fmt.Errorf("%s does not contain\n%s\n--- actual ---\n%s", filepath.Base(path), want, b)
	}

	return nil
}

// expectJournalBlock checks that the lines appear in one of the ledger's journal files.
func (w *world) expectJournalBlock(want string) error {
	var names []string
	for path, content := range w.journalFiles() {
		if containsBlock(string(content), want) {
			return nil
		}
		names = append(names, filepath.Base(path))
	}

	return fmt.Errorf("none of %v contains\n%s", names, want)
}

// expectUnchanged checks the file is byte for byte what it was.
func expectUnchanged(path string, before []byte) error {
	now, _ := os.ReadFile(path)
	if !bytes.Equal(now, before) {
		return fmt.Errorf("%s changed:\n--- before ---\n%s\n--- after ---\n%s", filepath.Base(path), before, now)
	}

	return nil
}

func expectAbsent(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("%s exists", path)
	}

	return nil
}

// expectJournalFilesUnchanged compares every journal file of the sandbox with its state at start, once konto
// is idle: a wrongful write still in flight would land after the check.
func (w *world) expectJournalFilesUnchanged() error {
	if err := w.awaitIdleIfRunning(); err != nil {
		return err
	}
	now := w.journalFiles()
	for path, before := range w.filesAtStart {
		if err := expectUnchanged(path, before); err != nil {
			return err
		}
	}

	if len(now) != len(w.filesAtStart) {
		return errors.New("the set of journal files changed")
	}

	return nil
}

// expectRulesUnchanged checks categories.rules is what it was at start, once konto is idle.
func (w *world) expectRulesUnchanged() error {
	if err := w.awaitIdleIfRunning(); err != nil {
		return err
	}

	return expectUnchanged(w.rules, w.rulesAtStart)
}

// awaitIdleIfRunning waits for konto to be idle, when it was started and still runs.
func (w *world) awaitIdleIfRunning() error {
	if w.s == nil || w.s.exited() {
		return nil
	}

	return w.awaitIdle()
}
