//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// popupRows are the popup's non-blank body rows.
func popupRows(p popup) []string {
	var rows []string
	for _, row := range p.rows {
		if strings.TrimSpace(row) != "" {
			rows = append(rows, row)
		}
	}
	return rows
}

// checkCalls are the `hledger check <name>` invocations konto made.
func (w *world) checkCalls() ([][]string, error) {
	calls, err := w.hledgerCalls()
	if err != nil {
		return nil, err
	}
	var checks [][]string
	for _, call := range calls {
		if i := indexOf(call, "check"); i >= 0 {
			checks = append(checks, call[i:])
		}
	}
	return checks, nil
}

func indexOf(list []string, want string) int {
	for i, item := range list {
		if item == want {
			return i
		}
	}
	return -1
}

func (w *world) expectCheckInvocations(n int) error {
	w.waitLoaded(w.s)
	calls, err := w.checkCalls()
	if err != nil {
		return err
	}
	if len(calls) != n {
		return fmt.Errorf("konto ran %d hledger check invocations, want %d: %v", len(calls), n, calls)
	}
	return nil
}

func (w *world) expectOneCheckEach() error {
	calls, err := w.checkCalls()
	if err != nil {
		return err
	}
	for _, call := range calls {
		if len(call) != 2 {
			return fmt.Errorf("invocation %v does not name exactly one check", call)
		}
	}
	return nil
}

func (w *world) expectChecksRun(names []string) error {
	calls, err := w.checkCalls()
	if err != nil {
		return err
	}
	var ran []string
	for _, call := range calls {
		ran = append(ran, call[1])
	}
	// The checks run side by side, so only the set of them is observable.
	slices.Sort(ran)
	if want := slices.Sorted(slices.Values(names)); strings.Join(ran, " ") != strings.Join(want, " ") {
		return fmt.Errorf("checks run: %v, want %v", ran, want)
	}
	return nil
}

// panelRow finds the checks panel row naming text.
func (w *world) panelRow(text string) (string, error) {
	var found string
	err := w.withPopup(func(p popup) error {
		for _, row := range p.rows {
			if strings.Contains(row, text) {
				found = row
				return nil
			}
		}
		return fmt.Errorf("the checks panel has no row for %q", text)
	})
	return found, err
}

func (w *world) expectRowCount(n int) error {
	return w.withPopup(func(p popup) error {
		if got := len(popupRows(p)); got != n {
			return fmt.Errorf("the checks panel has %d rows, want %d: %q", got, n, p.rows)
		}
		return nil
	})
}

func (w *world) expectRowShows(severity, check, line string) error {
	return w.withPopup(func(p popup) error {
		row := popupRows(p)[0]
		if !strings.Contains(row, severity) || !strings.Contains(row, check) || !strings.Contains(row, ".journal:"+line) {
			return fmt.Errorf("row %q does not show %s, %s and a reference to line %s", row, severity, check, line)
		}
		return nil
	})
}

// selectedPanelRow is the index of the panel row drawn on the selection background.
func (w *world) selectedPanelRow(p popup) (int, error) {
	s, err := w.session()
	if err != nil {
		return 0, err
	}
	rows := popupRows(p)
	for i := range rows {
		if s.cellBackground(1, p.top+1+i) == selectionBackground {
			return i, nil
		}
	}
	return 0, errors.New("no checks panel row is selected")
}

func (w *world) expectPanelSelection(which string) error {
	return w.withPopup(func(p popup) error {
		got, err := w.selectedPanelRow(p)
		if err != nil {
			return err
		}
		want := map[string]int{"first": 0, "second": 1, "last": len(popupRows(p)) - 1}[which]
		if got != want {
			return fmt.Errorf("row %d is selected, want the %s (row %d)", got, which, want)
		}
		return nil
	})
}

func (w *world) expectSelectedColours(which string, background, foreground int) error {
	if err := w.expectPanelSelection(which); err != nil {
		return err
	}
	return w.withPopup(func(p popup) error {
		i, _ := w.selectedPanelRow(p)
		s, _ := w.session()
		if got := s.cellColour(1, p.top+1+i); got != foreground {
			return fmt.Errorf("the selected row has foreground %d, want %d", got, foreground)
		}
		if got := s.cellBackground(1, p.top+1+i); got != background {
			return fmt.Errorf("the selected row has background %d, want %d", got, background)
		}
		return nil
	})
}

func (w *world) expectPanelHeightAtMost(lines int) error {
	return w.withPopup(func(p popup) error {
		if height := len(p.rows) + 1; height > lines {
			return fmt.Errorf("the panel is %d lines tall, want at most %d", height, lines)
		}
		return nil
	})
}

func (w *world) reachLastRow() error {
	if err := w.press(repeat("↓", 30)...); err != nil {
		return err
	}
	return w.expectPanelSelection("last")
}

func (w *world) expectSelectionVisible() error {
	return w.withPopup(func(p popup) error { _, err := w.selectedPanelRow(p); return err })
}

func registerChecksPanelSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see "([^"]*)" and "([^"]*)" in the checks panel$`, func(a, b string) error {
		return w.expectPopupContains("checks", []string{a, b})
	})
	step(sc, `^the all-pass line is coloured dim-green$`, func() error { return w.expectColour("✓ all", 2) })
	step(sc, `^the checks panel has no "([^"]*)" rows$`, func(text string) error { return w.expectNotInPopup(text) })
	step(sc, `^the checks panel has exactly (\d+) rows?$`, w.expectRowCount)
	step(sc, `^the row shows severity "([^"]*)", check "([^"]*)" and a "file:(\d+)" reference$`, w.expectRowShows)
	step(sc, `^the checks panel has a row for check "([^"]*)"$`, func(check string) error {
		row, err := w.panelRow(check)
		w.focusedRow = row
		return err
	})
	step(sc, `^its severity is "([^"]*)"$`, func(severity string) error {
		if !strings.Contains(w.focusedRow, severity) {
			return fmt.Errorf("row %q does not have severity %s", w.focusedRow, severity)
		}
		return nil
	})
	step(sc, `^the (first|second|last) row is selected$`, w.expectPanelSelection)
	step(sc, `^the (second) row is selected with background (\d+) and foreground (\d+)$`, w.expectSelectedColours)
	step(sc, `^the panel is no taller than (\d+) lines$`, w.expectPanelHeightAtMost)
	step(sc, `^I can reach the last row with "↓"$`, w.reachLastRow)
	step(sc, `^the selected row is always visible$`, w.expectSelectionVisible)
}

func registerChecksRunSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^konto ran (\d+) separate "hledger check" invocations$`, w.expectCheckInvocations)
	step(sc, `^each invocation named exactly one check$`, w.expectOneCheckEach)
	step(sc, `^the checks run are ((?:"[^"]*" ?)+)$`, func(list string) error { return w.expectChecksRun(quotedList(list)) })
}

// lineEnding matches a posting's balance assertion.
var assertionSuffix = regexp.MustCompile(` = [\d.,]+ EUR$`)

// editLastPosting rewrites the last line of the journal that posts to account with an assertion.
func (w *world) editLastPosting(account string, edit func(line string) string) error {
	b, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], account) && assertionSuffix.MatchString(lines[i]) {
			lines[i] = edit(lines[i])
			return os.WriteFile(w.journal, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return fmt.Errorf("no posting with an assertion on %s", account)
}

// warnings declares n asset accounts without a source: n unclassified-account warnings.
func (w *world) warnings(name string, n int) error {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("account assets:unsorted-%02d", i+1)
	}
	return w.withDeclared(name, lines...)
}

const twoUndeclared = "\npayee Kiosk\n2026-09-01 Kiosk\n    expenses:first  1,00 EUR\n    assets:transfers\n" +
	"2026-09-02 Kiosk\n    expenses:second  1,00 EUR\n    assets:transfers\n"

const extraIssue = "payee Kiosk\n2026-09-01 Kiosk\n    expenses:coffee  2,00 EUR\n    assets:transfers\n"

func (w *world) includeWithIssue(name, include string) error {
	if err := w.useJournal(name, "\ninclude "+include+"\n"); err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(w.journal), include)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(extraIssue), 0o644)
}

func (w *world) expectEditorOpenedIncluded(include string) error {
	b, err := os.ReadFile(w.editorLog())
	if err != nil {
		return fmt.Errorf("the editor was never called: %w", err)
	}
	if want := filepath.Join(filepath.Dir(w.journal), include); !bytes.Contains(b, []byte(want)) {
		return fmt.Errorf("editor calls:\n%s\nwant one naming %s", b, want)
	}
	return nil
}

func registerWarningFixtureSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the journal "([^"]*)" with (\d+|one) (?:warnings?|unclassified accounts?)$`, func(name, n string) error {
		count := 1
		if n != "one" {
			_, _ = fmt.Sscanf(n, "%d", &count)
		}
		return w.warnings(name, count)
	})
	step(sc, `^the journal "([^"]*)" with a wrong assertion "= ([^"]*)" on the last "([^"]*)" posting$`,
		func(name, wrong, account string) error {
			if err := w.useJournal(name, ""); err != nil {
				return err
			}
			return w.editLastPosting(account, func(line string) string { return assertionSuffix.ReplaceAllString(line, " = "+wrong) })
		})
	step(sc, `^the journal "([^"]*)" without the assertion on the last "([^"]*)" transaction$`, func(name, account string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.editLastPosting(account, func(line string) string { return assertionSuffix.ReplaceAllString(line, "") })
	})
}

func registerChecksFixtureSteps(sc *godog.ScenarioContext, w *world) {
	registerWarningFixtureSteps(sc, w)
	step(sc, `^the journal "minimal" with two undeclared accounts on different lines$`, func() error {
		return w.useJournal("minimal", twoUndeclared)
	})
	step(sc, `^the journal "([^"]*)" includes "([^"]*)" with the issue in it$`, w.includeWithIssue)
	step(sc, `^a journal containing an unbalanced transaction$`, func() error {
		return w.useJournal("minimal", "\n2026-09-01 Broken\n    expenses:x  1,00 EUR\n    assets:transfers  -2,00 EUR\n")
	})
	step(sc, `^the checks panel lists (?:only )?the (first|second) undeclared account$`, func(which string) error {
		want, other := "expenses:first", "expenses:second"
		if which == "second" {
			want, other = other, want
		}
		return w.withPopup(func(p popup) error {
			text := strings.Join(p.rows, "\n")
			if !strings.Contains(text, want) || strings.Contains(text, other) {
				return fmt.Errorf("the panel should list %s and not %s:\n%s", want, other, text)
			}
			return nil
		})
	})
	step(sc, `^I fix that account externally and press "r"$`, func() error {
		if err := w.declare("account expenses:first"); err != nil {
			return err
		}
		return w.press("r")
	})
}

func (w *world) expectFullRefresh() error {
	w.waitLoaded(w.s)
	calls, err := w.hledgerCalls()
	if err != nil {
		return err
	}
	loads := 0
	for _, call := range calls {
		if indexOf(call, "print") >= 0 {
			loads++
		}
	}
	if loads < 2 {
		return fmt.Errorf("only %d loads of the journal, want a reload after the editor", loads)
	}
	return nil
}

func (w *world) expectLeftAlternateScreen() error {
	s, err := w.session()
	if err != nil {
		return err
	}
	out := string(s.rawOutput())
	entered := strings.Index(out, "\x1b[?1049h")
	if entered < 0 || !strings.Contains(out[entered:], "\x1b[?1049l") {
		return errors.New("konto never left the alternate screen after entering it")
	}
	return nil
}

func (w *world) visualStub() error {
	stub := filepath.Join(w.dir, "visual-stub")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + w.visualLog() + "\"\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		return err
	}
	w.env["VISUAL"] = stub
	return nil
}

func (w *world) expectVisualCalled() error {
	if _, err := os.Stat(w.visualLog()); err != nil {
		return fmt.Errorf("the VISUAL stub was never called: %w", err)
	}
	return nil
}

func (w *world) expectEditorNotCalled() error {
	if _, err := os.Stat(w.editorLog()); err == nil {
		return errors.New("the EDITOR stub was called")
	}
	return nil
}

func registerChecksEditorSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^EDITOR is a stub that declares "([^"]*)" in the journal$`, func(account string) error {
		return w.editorStub(`printf '\naccount ` + account + `\n' >> "` + w.journal + `"`)
	})
	step(sc, `^VISUAL is a stub recording its arguments$`, w.visualStub)
	step(sc, `^the VISUAL stub was called$`, w.expectVisualCalled)
	step(sc, `^the EDITOR stub was not called$`, w.expectEditorNotCalled)
	step(sc, `^a full refresh happened$`, w.expectFullRefresh)
	step(sc, `^the terminal left the alternate screen while the editor ran$`, w.expectLeftAlternateScreen)
	step(sc, `^after the editor exits I see the tab bar again$`, func() error { return w.expectScreen(true, tabBarMarker) })
}

func registerChecksSteps(sc *godog.ScenarioContext, w *world) {
	registerChecksPanelSteps(sc, w)
	registerChecksRunSteps(sc, w)
	registerChecksFixtureSteps(sc, w)
	registerChecksEditorSteps(sc, w)
}

func (w *world) visualLog() string { return filepath.Join(w.dir, "visual-calls.log") }
