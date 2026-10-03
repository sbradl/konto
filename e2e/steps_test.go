//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// registerSteps binds the generic step vocabulary (features/README.md). View-
// specific phrasings are added next to the view they test, as it is built.
func registerSteps(sc *godog.ScenarioContext, w *world) {
	registerGivenSteps(sc, w)
	registerWhenSteps(sc, w)
	registerScreenSteps(sc, w)
	registerProcessSteps(sc, w)
	registerFileSteps(sc, w)
	registerJournalPathSteps(sc, w)
	registerInstanceSteps(sc, w)
	registerFirstRunSteps(sc, w)
	registerParseErrorSteps(sc, w)
	registerChromeSteps(sc, w)
}

func registerGivenSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal "([^"]*)"$`, func(name string) error { return w.useJournal(name, "") })
	sc.Step(`^the journal "([^"]*)" with (?:appended text|a rule) "((?s:.*))"$`, func(name, text string) error {
		// Gherkin turns \n in a table cell into a real newline, hence (?s). Appended text starts
		// on its own line unless it says otherwise: a leading newline is its first (blank) line.
		text = unescape(text)
		if !strings.HasPrefix(text, "\n") {
			text = "\n" + text
		}
		return w.useJournal(name, text)
	})
	sc.Step(`^a journal containing "(.*)"$`, func(text string) error {
		return os.WriteFile(w.journal, []byte(unescape(text)+"\n"), 0o644)
	})
	sc.Step(`^no journal exists at the resolved path$`, func() error {
		if w.lastStepType == outcomeStep {
			return expectAbsent(w.journal)
		}
		return removeIfExists(w.journal)
	})
	sc.Step(`^today is (\d{4}-\d{2}-\d{2})$`, func(day string) { w.env["KONTO_TODAY"] = day })
	sc.Step(`^the terminal is (\d+)×(\d+)$`, w.setTerminalSize)
	sc.Step(`^XDG_DOWNLOAD_DIR is not configured$`, func() { w.unset["XDG_DOWNLOAD_DIR"] = true })
	sc.Step(`^a Postbank CSV "([^"]*)" for IBAN "([^"]*)" with closing balance ([\d.,-]+) EUR( and CRLF line endings| and a trailing blank line)?:$`,
		func(file, iban, balance, variant string, table *godog.Table) error {
			rows, err := postbankRows(table)
			if err != nil {
				return err
			}
			export := postbankExport{
				IBAN: iban, ClosingBalance: balance, Rows: rows,
				CRLF: strings.Contains(variant, "CRLF"), TrailingBlank: strings.Contains(variant, "blank"),
			}
			return w.writeExport(file, export)
		})
	sc.Step(`^a Postbank CSV "([^"]*)" for IBAN "([^"]*)" with closing balance ([\d.,-]+) EUR and a broken row "(.*)"$`,
		func(file, iban, balance, broken string) error {
			row := postbankRow{Date: "29.9.2026", Type: "Kartenzahlung", Payee: "Kiosk", Purpose: "x", Amount: "-1,00"}
			return w.writeExport(file, postbankExport{IBAN: iban, ClosingBalance: balance, Rows: []postbankRow{row}, BrokenRow: broken})
		})
	sc.Step(`^the download directory contains "([^"]*)" with these exact bytes \(UTF-8 BOM first, LF line ends\):$`,
		func(file string, doc *godog.DocString) error {
			return os.WriteFile(filepath.Join(w.downloads, file), []byte("\xef\xbb\xbf"+doc.Content+"\n"), 0o644)
		})
	sc.Step(`^a file "([^"]*)" in the download directory containing "(.*)"$`, func(file, content string) error {
		return os.WriteFile(filepath.Join(w.downloads, file), []byte(unescape(content)), 0o644)
	})
	registerHledgerShimSteps(sc, w)
	registerEditorSteps(sc, w)
}

func registerHledgerShimSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^hledger is not on PATH$`, func() { w.env["PATH"] = filepath.Join(w.dir, "empty-path") })
	sc.Step(`^hledger reports version "([^"]*)"$`, func(v string) error {
		w.shim.Version = v
		return w.writeShim()
	})
	sc.Step(`^hledger is slowed by (\d+)(ms|s)$`, func(n int, unit string) error {
		w.shim.DelayMs = n
		if unit == "s" {
			w.shim.DelayMs = n * 1000
		}
		return w.writeShim()
	})
	sc.Step(`^hledger starts failing(?: with "([^"]*)")?$`, func(msg string) error {
		w.shim.Fail = "hledger: simulated failure " + msg
		return w.writeShim()
	})
	sc.Step(`^hledger returns garbage for "([^"]*)"$`, func(command string) error {
		w.shim.GarbageFor = append(w.shim.GarbageFor, command)
		return w.writeShim()
	})
}

func registerEditorSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^EDITOR is a stub recording its arguments$`, func() error {
		return w.editorStub(`printf '%s\n' "$*" >> "` + w.editorLog() + `"`)
	})
	sc.Step(`^EDITOR is a stub that replaces the journal with "([^"]*)"$`, func(name string) error {
		return w.editorStub(`printf '%s\n' "$*" >> "` + w.editorLog() + `"` + "\n" +
			`cp "` + w.fixture(name+".journal") + `" "` + w.journal + `"`)
	})
}

func registerWhenSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^I start konto$`, func() error {
		if w.s != nil { // a scenario that changes the journal after its Background restarts konto
			w.s.stop()
			w.s = nil
		}
		return w.start()
	})
	sc.Step(`^I start konto with "([^"]*)"$`, func(args string) error { return w.start(strings.Fields(args)...) })
	sc.Step(`^I press "([^"]*)"$`, func(key string) error { return w.press(key) })
	sc.Step(`^I press "([^"]*)" twice$`, func(key string) error { return w.press(key, key) })
	sc.Step(`^I press "([^"]*)" (\d+) times$`, func(key string, n int) error { return w.press(repeat(key, n)...) })
	sc.Step(`^I type "([^"]*)"$`, func(text string) error { return w.typeText(text) })
	sc.Step(`^I resize the terminal to (\d+)×(\d+)$`, func(width, height int) error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if err := s.resize(width, height); err != nil {
			return err
		}
		s.settle(w.suite.timeout)
		return nil
	})
}

func registerScreenSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^I see "([^"]*)"$`, func(text string) error { return w.expectScreen(true, text) })
	sc.Step(`^I do not see "([^"]*)"$`, func(text string) error { return w.expectScreen(false, text) })
	sc.Step(`^I see the tab bar$`, func() error { return w.expectScreen(true, tabBarMarker) })
	sc.Step(`^I do not see the tab bar$`, func() error { return w.expectScreen(false, tabBarMarker) })
	sc.Step(`^the tab bar reads "(.*)"$`, func(text string) error { return w.expectLine(0, "tab bar", text, strings.Contains) })
	sc.Step(`^the status bar shows "([^"]*)"$`, func(text string) error { return w.expectLine(-1, "status bar", text, strings.Contains) })
	sc.Step(`^the status bar starts with "([^"]*)"$`, func(text string) error {
		return w.expectLine(-1, "status bar", text, func(line, t string) bool { return strings.HasPrefix(strings.TrimSpace(line), t) })
	})
	sc.Step(`^the status bar does not show "([^"]*)"$`, func(text string) error {
		return w.expectLine(-1, "status bar", text, func(line, t string) bool { return !strings.Contains(line, t) })
	})
	sc.Step(`^"([^"]*)" is coloured (\d+)$`, func(text string, colour int) error { return w.expectColour(text, colour) })
	sc.Step(`^the screen is unchanged$`, func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if now := s.screenText(); now != w.baseline {
			return fmt.Errorf("screen changed:\n--- before ---\n%s\n--- after ---\n%s", w.baseline, now)
		}
		return nil
	})
	sc.Step(`^the screen contains no 24-bit or 256-colour escape sequences$`, func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if m := extendedColour.Find(s.rawOutput()); m != nil {
			return fmt.Errorf("found extended colour sequence %q", m)
		}
		return nil
	})
}

func registerProcessSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^konto exits with code (\d+)$`, w.expectExitCode)
	sc.Step(`^konto is still running$`, w.expectRunning)
	sc.Step(`^stderr mentions "([^"]*)"(?: and "([^"]*)")?$`, func(a, b string) error { return w.expectStderr(a, b) })
	sc.Step(`^stderr mentions the minimum version "([^"]*)"$`, func(v string) error { return w.expectStderr(v) })
	sc.Step(`^every recorded hledger invocation contains "-f <absolute journal path>"$`, func() error {
		return w.expectEveryCall([]string{"-f", w.journal})
	})
	sc.Step(`^the editor was called with "(.*)"$`, w.expectEditorCall)
}

func (w *world) expectExitCode(want int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	got, err := s.waitExit(w.suite.timeout)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("exit code %d, want %d; stderr:\n%s", got, want, s.stderr.String())
	}
	return nil
}

func (w *world) expectRunning() error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	if s.exited() {
		return fmt.Errorf("konto exited with code %d; stderr:\n%s", s.exitCode, s.stderr.String())
	}
	return nil
}

func (w *world) expectEditorCall(want string) error {
	if included, ok := strings.CutPrefix(want, "+<line> <absolute path of "); ok {
		return w.expectEditorOpenedIncluded(strings.TrimSuffix(included, ">"))
	}
	b, err := os.ReadFile(w.editorLog())
	if err != nil {
		return fmt.Errorf("the editor was never called: %w", err)
	}
	if !containsBlock(string(b), w.expand(want)) {
		return fmt.Errorf("editor calls:\n%s\nwant %q", b, w.expand(want))
	}
	return nil
}

func registerFileSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal contains$`, func(doc *godog.DocString) error { return expectFileBlock(w.journal, doc.Content) })
	sc.Step(`^categories.rules contains$`, func(doc *godog.DocString) error { return expectFileBlock(w.rules, doc.Content) })
	sc.Step(`^the journal is unchanged$`, w.expectJournalFilesUnchanged)
	sc.Step(`^categories.rules is unchanged$`, func() error { return expectUnchanged(w.rules, w.rulesAtStart) })
	sc.Step(`^(?:every )?hledger check passes$`, func() error { return runChecks(w.suite.realHledger, w.journal, false) })
	sc.Step(`^every hledger check passes individually$`, func() error { return runChecks(w.suite.realHledger, w.journal, true) })
}

// tabBarMarker is the start of the v1 tab bar (spec §6.4a).
const tabBarMarker = "1 dash │ 2 import"

var extendedColour = regexp.MustCompile(`\x1b\[[0-9;:]*[34]8[;:][25][;:]`)

func (w *world) expectScreen(present bool, text string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	if present {
		if s.waitFor(w.suite.timeout, func(screen string) bool { return strings.Contains(screen, text) }) {
			return nil
		}
		return fmt.Errorf("did not see %q on screen:\n%s", text, s.screenText())
	}
	s.settle(w.suite.timeout)
	if screen := s.screenText(); strings.Contains(screen, text) {
		return fmt.Errorf("unexpectedly saw %q on screen:\n%s", text, screen)
	}
	return nil
}

// expectLine checks one screen row; row -1 is the last row.
func (w *world) expectLine(row int, what, text string, ok func(line, text string) bool) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var line string
	match := s.waitFor(w.suite.timeout, func(string) bool {
		lines := s.screenLines()
		if row < 0 {
			row = len(lines) + row
		}
		line = lines[row]
		return ok(line, text)
	})
	if !match {
		return fmt.Errorf("%s %q does not satisfy %q", what, line, text)
	}
	return nil
}

func (w *world) expectColour(text string, want int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	if !s.waitFor(w.suite.timeout, func(screen string) bool { return strings.Contains(screen, text) }) {
		return fmt.Errorf("%q is not on screen:\n%s", text, s.screenText())
	}
	seen := s.colours(text)
	if len(seen) == 0 {
		return fmt.Errorf("%q left the screen before its colour was read", text)
	}
	if !slices.Contains(seen, want) {
		return fmt.Errorf("%q is coloured %v, want %d", text, seen, want)
	}
	return nil
}

func (w *world) expectStderr(parts ...string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	if _, err := s.waitExit(w.suite.timeout); err != nil {
		return err
	}
	for _, p := range parts {
		if !strings.Contains(s.stderr.String(), p) {
			return fmt.Errorf("stderr does not mention %q:\n%s", p, s.stderr.String())
		}
	}
	return nil
}

func (w *world) expectEveryCall(want []string) error {
	calls, err := w.hledgerCalls()
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return errors.New("konto made no hledger calls")
	}
	for _, c := range calls {
		if !containsSeq(c, want) {
			return fmt.Errorf("hledger call %q lacks %q", c, want)
		}
	}
	return nil
}

// hledgerCalls reads the shim's log, skipping the version preflight.
func (w *world) hledgerCalls() ([][]string, error) {
	log, err := os.ReadFile(w.shimLog)
	if err != nil {
		return nil, err
	}
	var calls [][]string
	for line := range strings.SplitSeq(strings.TrimSpace(string(log)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			return nil, fmt.Errorf("hledger call log line %q: %w", line, err)
		}
		if len(args) != 1 || args[0] != "--version" {
			calls = append(calls, args)
		}
	}
	return calls, nil
}

func (w *world) typeText(text string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	w.waitLoaded(s)
	if err := s.send(text); err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	return nil
}

func (w *world) writeExport(file string, e postbankExport) error {
	b, err := e.render()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.downloads, file), b, 0o644)
}

func (w *world) editorLog() string { return filepath.Join(w.dir, "editor-calls.log") }

func (w *world) editorStub(body string) error {
	stub := filepath.Join(w.dir, "editor-stub")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		return err
	}
	w.env["EDITOR"], w.env["VISUAL"] = stub, ""
	return nil
}

func postbankRows(t *godog.Table) ([]postbankRow, error) {
	if len(t.Rows) < 2 {
		return nil, errors.New("the CSV table needs a header and at least one row")
	}
	want := []string{"date", "type", "payee", "purpose", "amount"}
	for i, c := range t.Rows[0].Cells {
		if i >= len(want) || c.Value != want[i] {
			return nil, fmt.Errorf("CSV table header must be %v", want)
		}
	}
	var rows []postbankRow
	for _, r := range t.Rows[1:] {
		v := func(i int) string { return r.Cells[i].Value }
		rows = append(rows, postbankRow{Date: v(0), Type: v(1), Payee: v(2), Purpose: v(3), Amount: strings.TrimSpace(v(4))})
	}
	return rows, nil
}

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

func expectUnchanged(path string, before []byte) error {
	now, _ := os.ReadFile(path)
	if !bytes.Equal(now, before) {
		return fmt.Errorf("%s changed:\n--- before ---\n%s\n--- after ---\n%s", filepath.Base(path), before, now)
	}
	return nil
}

func containsSeq(have, want []string) bool {
	for i := 0; i+len(want) <= len(have); i++ {
		if equalLines(have[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

// unescape turns the Gherkin cell escape \n into a newline.
func unescape(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

func repeat(key string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = key
	}
	return keys
}

func expectAbsent(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("%s exists", path)
	}
	return nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// expectJournalFilesUnchanged compares every journal file of the sandbox with its state at start.
func (w *world) expectJournalFilesUnchanged() error {
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
