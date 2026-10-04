//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"
)

// accountOffset is how many rows the cursor must move to reach the named account.
func (w *world) accountOffset(name string) (int, error) {
	var offset int
	err := w.expectAccounts(func(rows []accountRow) error {
		selected, err := w.selectedRow(rows)
		if err != nil {
			return err
		}
		target, err := indexOfRow(rows, func(row accountRow) bool { return row.name() == name })
		if err != nil {
			return fmt.Errorf("no Accounts row for %q", name)
		}
		current, _ := indexOfRow(rows, func(row accountRow) bool { return row.y == selected.y })
		offset = target - current
		return nil
	})
	return offset, err
}

func indexOfRow(rows []accountRow, match func(accountRow) bool) (int, error) {
	for i, row := range rows {
		if match(row) {
			return i, nil
		}
	}
	return -1, errors.New("no such row")
}

// selectAccount moves the Accounts cursor onto the named account.
func (w *world) selectAccount(name string) error {
	if s, err := w.session(); err == nil { // a refresh after a write must land before rows are read
		s.settle(w.suite.timeout)
		s.waitFor(w.suite.timeout, func(screen string) bool { return !strings.Contains(screen, "⣾") })
	}
	offset, err := w.accountOffset(name)
	if err != nil || offset == 0 {
		return err
	}
	if offset < 0 {
		return w.press(repeat("↑", -offset)...)
	}
	return w.press(repeat("↓", offset)...)
}

// directiveLine finds the account directive of the named account in the sandbox's journal files.
func (w *world) directiveLine(account string) (string, error) {
	for _, content := range w.journalFiles() {
		for line := range strings.SplitSeq(string(content), "\n") {
			if strings.HasPrefix(line, "account "+account+" ") || line == "account "+account {
				return line, nil
			}
		}
	}
	return "", fmt.Errorf("no account directive for %s", account)
}

func (w *world) expectDirective(account string, contains bool, texts []string) error {
	line, err := w.directiveLine(account)
	if err != nil {
		return err
	}
	for _, text := range texts {
		if strings.Contains(line, text) != contains {
			return fmt.Errorf("directive %q: contains %q = %v, want %v", line, text, !contains, contains)
		}
	}
	w.focusedRow = account
	return nil
}

func (w *world) statusLine() (string, error) {
	var line string
	err := w.expectLine(-1, "status bar", "", func(l, _ string) bool { line = l; return true })
	return line, err
}

func (w *world) expectStatusHas(all bool, texts []string) error {
	var failure error
	err := w.expectLine(-1, "status bar", "", func(line, _ string) bool {
		matched := 0
		for _, text := range texts {
			if strings.Contains(line, text) {
				matched++
			}
		}
		failure = nil
		if (all && matched != len(texts)) || (!all && matched == 0) {
			failure = fmt.Errorf("status bar %q does not show %v", line, texts)
		}
		return failure == nil
	})
	if err != nil {
		return failure
	}
	return nil
}

func (w *world) expectNoPrompt() error {
	w.s.settle(w.suite.timeout)
	line, err := w.statusLine()
	if err != nil {
		return err
	}
	if strings.Contains(line, "(y/n)") {
		return fmt.Errorf("a y/n prompt is open: %q", line)
	}
	return nil
}

func rowIsClosed(row accountRow) bool { return strings.Contains(row.text, "closed ") }

// firstRowWhere is the index of the first of rows for which closed-ness equals want, or -1.
func firstRowWhere(rows []accountRow, closed bool) int {
	i, err := indexOfRow(rows, func(row accountRow) bool { return rowIsClosed(row) == closed })
	if err != nil {
		return -1
	}
	return i
}

// expectRowPlace checks where the named account's row sits: among the open accounts or below them.
func (w *world) expectRowPlace(account string, closed bool) error {
	return w.expectAccounts(func(rows []accountRow) error {
		at, err := indexOfRow(rows, func(row accountRow) bool { return row.name() == account })
		if err != nil {
			return fmt.Errorf("no Accounts row for %q", account)
		}
		if rowIsClosed(rows[at]) != closed {
			return fmt.Errorf("row %q: closed = %v, want %v", rows[at].text, rowIsClosed(rows[at]), closed)
		}
		w.focusedRow = account
		// Open accounts come first: nothing open follows a closed row, nothing closed precedes an open one.
		if closed && firstRowWhere(rows[at+1:], false) >= 0 {
			return fmt.Errorf("an open account is listed below %s", account)
		}
		if !closed && firstRowWhere(rows[:at], true) >= 0 {
			return fmt.Errorf("a closed account is listed above %s", account)
		}
		return nil
	})
}

func (w *world) expectLinesChanged(n int) error {
	changed := 0
	for path, before := range w.filesAtStart {
		now, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		beforeLines, nowLines := strings.Split(string(before), "\n"), strings.Split(string(now), "\n")
		if len(beforeLines) != len(nowLines) {
			return fmt.Errorf("%s changed its line count", path)
		}
		for i := range beforeLines {
			if beforeLines[i] != nowLines[i] {
				changed++
			}
		}
	}
	if changed != n {
		return fmt.Errorf("%d journal lines changed, want %d", changed, n)
	}
	return nil
}

func (w *world) expectByteIdentical() error {
	now := w.journalFiles()
	for path, before := range w.filesAtStart {
		if !bytes.Equal(now[path], before) {
			return fmt.Errorf("%s differs from the start:\n%s", path, now[path])
		}
	}
	return nil
}

func (w *world) expectFocusedShows(text string, uncoloured int) error {
	return w.expectAccounts(func(rows []accountRow) error {
		row, err := accountNamed(rows, w.focusedRow)
		if err != nil {
			return err
		}
		if !strings.Contains(row.text, text) {
			return fmt.Errorf("row %q does not show %q", row.text, text)
		}
		if uncoloured >= 0 {
			if got, _ := w.rowColour(w.focusedRow, w.focusedRow); got == uncoloured {
				return fmt.Errorf("row %s is coloured %d", w.focusedRow, got)
			}
		}
		return nil
	})
}

func registerAccountSelectionSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I am on the dashboard$`, func() error { return w.press("1") })
	step(sc, `^I am on "(\d)"$`, func(view string) error { return w.press(view) })
	step(sc, `^I select "([^"]*)"(?: on the dashboard)?$`, w.selectAccount)
	step(sc, `^I select "([^"]*)" and press "([^"]*)"$`, func(account, key string) error {
		if err := w.selectAccount(account); err != nil {
			return err
		}
		return w.press(key)
	})
	step(sc, `^I select "([^"]*)" and press "([^"]*)" and "([^"]*)"$`, func(account, first, second string) error {
		if err := w.selectAccount(account); err != nil {
			return err
		}
		return w.press(first, second)
	})
}

func registerPromptSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see a y/n prompt naming "([^"]*)"$`, func(account string) error {
		return w.expectStatusHas(true, []string{account, "(y/n)"})
	})
	step(sc, `^the (?:y/n )?prompt is still open$`, func() error { return w.expectStatusHas(true, []string{"(y/n)"}) })
	step(sc, `^no y/n prompt is shown$`, w.expectNoPrompt)
	step(sc, `^I see a refusal mentioning ("[^"]*" and "[^"]*")$`, func(list string) error {
		return w.expectStatusHas(true, quotedList(list))
	})
	step(sc, `^I see a refusal mentioning ("[^"]*" or "[^"]*")$`, func(list string) error {
		return w.expectStatusHas(false, quotedList(list))
	})
	step(sc, `^I see a refusal mentioning "([^"]*)"$`, func(text string) error { return w.expectStatusHas(true, []string{text}) })
	step(sc, `^I see a refusal "([^"]*)"$`, func(text string) error { return w.expectStatusHas(true, []string{text}) })
	step(sc, `^the refusal names ("[^"]*" and "[^"]*")$`, func(list string) error { return w.expectStatusHas(true, quotedList(list)) })
	step(sc, `^no full-screen error is shown$`, func() error { return w.expectScreen(true, tabBarMarker) })
}

func registerDirectiveSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the journal's "account ([^"]*)" directive contains "([^"]*)"$`, func(account, text string) error {
		return w.expectDirective(account, true, []string{text})
	})
	step(sc, `^the journal's "account ([^"]*)" directive does not contain "([^"]*)"$`, func(account, text string) error {
		return w.expectDirective(account, false, []string{text})
	})
	step(sc, `^the directive still contains ("[^"]*"(?: and "[^"]*")*)$`, func(list string) error {
		return w.expectDirective(w.directiveOwner(), true, quotedList(list))
	})
	step(sc, `^the directive contains "([^"]*)"$`, func(text string) error {
		return w.expectDirective("assets:spare", true, []string{text})
	})
	step(sc, `^exactly (\d+) lines? of the journal changed$`, w.expectLinesChanged)
	step(sc, `^the journal is byte-identical to the start$`, w.expectByteIdentical)
}

// directiveOwner is the account the last directive step looked at.
func (w *world) directiveOwner() string { return w.focusedRow }

func registerAccountRowSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the row "([^"]*)" is listed below every open account$`, func(account string) error { return w.expectRowPlace(account, true) })
	step(sc, `^the row "([^"]*)" is listed among the open accounts$`, func(account string) error { return w.expectRowPlace(account, false) })
	step(sc, `^it shows "([^"]*)" and is not coloured (\d+)$`, w.expectFocusedShows)
	step(sc, `^it shows "([^"]*)"$`, func(text string) error { return w.expectFocusedShows(text, -1) })
	step(sc, `^the journal "([^"]*)" with two rules "([^"]*)" and "([^"]*)" referencing "([^"]*)"$`,
		func(name, first, second, account string) error {
			rule := func(description string) string {
				return "\n~ monthly from 2026-11-01  " + description + "\n    " + account + "  10,00 EUR\n    assets:checking\n"
			}
			return w.useJournal(name, rule(first)+rule(second))
		})
}

func registerAccountChangeSteps(sc *godog.ScenarioContext, w *world) {
	registerAccountSelectionSteps(sc, w)
	registerPromptSteps(sc, w)
	registerDirectiveSteps(sc, w)
	registerAccountRowSteps(sc, w)
}
