//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cucumber/godog"
)

// accountRow is one row of the Accounts panel as drawn.
type accountRow struct {
	y    int
	text string // the panel's inside, without borders
}

// name is the account name: the word after the source glyph.
func (r accountRow) name() string {
	if fields := strings.Fields(r.text); len(fields) > 1 {
		return fields[1]
	}
	return ""
}

// accountsPanel finds the Accounts panel's rows: the lines under its title down to its bottom border.
func (w *world) accountsPanel() ([]accountRow, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	var rows []accountRow
	inside := false
	for y, line := range s.screenLines() {
		switch {
		case strings.HasPrefix(line, "╭─ ACCOUNTS "):
			inside = true
		case inside && strings.HasPrefix(line, "╰"):
			return rows, nil
		case inside:
			if parts := strings.Split(line, "│"); len(parts) > 1 {
				rows = append(rows, accountRow{y: y, text: parts[1]})
			}
		}
	}
	return nil, errors.New("no Accounts panel on screen:\n" + s.screenText())
}

// expectAccounts waits until check accepts the panel's non-empty rows.
func (w *world) expectAccounts(check func(rows []accountRow) error) error {
	var failure error
	s, err := w.session()
	if err != nil {
		return err
	}
	s.waitFor(w.suite.timeout, func(string) bool {
		rows, perr := w.accountsPanel()
		if perr != nil {
			failure = perr
			return false
		}
		var filled []accountRow
		for _, row := range rows {
			if strings.TrimSpace(row.text) != "" {
				filled = append(filled, row)
			}
		}
		failure = check(filled)
		return failure == nil
	})
	if failure != nil {
		return fmt.Errorf("%w\n%s", failure, s.screenText())
	}
	return nil
}

// accountNamed is the row of the named account.
func accountNamed(rows []accountRow, name string) (accountRow, error) {
	for _, row := range rows {
		if row.name() == name {
			return row, nil
		}
	}
	return accountRow{}, fmt.Errorf("no Accounts row for %q", name)
}

// cellOf is the screen column of text inside a panel row; the panel starts at column 0, behind its border.
func cellOf(row accountRow, text string) (int, error) {
	before, _, found := strings.Cut(row.text, text)
	if !found {
		return 0, fmt.Errorf("row %q does not show %q", row.text, text)
	}
	return 1 + utf8.RuneCountInString(before), nil
}

// rowColour is the foreground colour of text on the account's row.
func (w *world) rowColour(account, text string) (int, error) {
	var colour int
	err := w.expectAccounts(func(rows []accountRow) error {
		row, err := accountNamed(rows, account)
		if err != nil {
			return err
		}
		x, err := cellOf(row, text)
		if err != nil {
			return err
		}
		s, _ := w.session()
		colour = s.cellColour(x, row.y)
		return nil
	})
	return colour, err
}

func (w *world) expectRowColour(account, text string, want int, negate bool) error {
	got, err := w.rowColour(account, text)
	if err != nil {
		return err
	}
	if (got == want) == negate {
		return fmt.Errorf("%q on %s is coloured %d, negated=%v want %d", text, account, got, negate, want)
	}
	return nil
}

func (w *world) expectSelected(account string, background, foreground int) error {
	return w.expectAccounts(func(rows []accountRow) error {
		row, err := accountNamed(rows, account)
		if err != nil {
			return err
		}
		s, _ := w.session()
		x, _ := cellOf(row, account)
		if got := s.cellBackground(x, row.y); got != background {
			return fmt.Errorf("row %s has background %d, want %d", account, got, background)
		}
		if foreground >= 0 {
			if got := s.cellColour(x, row.y); got != foreground {
				return fmt.Errorf("row %s has foreground %d, want %d", account, got, foreground)
			}
		}
		return nil
	})
}

// selectedRow is the row drawn on the selection background.
func (w *world) selectedRow(rows []accountRow) (accountRow, error) {
	s, err := w.session()
	if err != nil {
		return accountRow{}, err
	}
	for _, row := range rows {
		if x, err := cellOf(row, row.name()); err == nil && s.cellBackground(x, row.y) == selectionBackground {
			return row, nil
		}
	}
	return accountRow{}, errors.New("no Accounts row is selected")
}

// rowsStep adapts a per-panel check to a step.
func (w *world) rowsStep(check func(rows []accountRow) error) func() error {
	return func() error { return w.expectAccounts(check) }
}

func expectOrder(table *godog.Table) func(rows []accountRow) error {
	return func(rows []accountRow) error {
		for i, tr := range table.Rows {
			if i >= len(rows) || rows[i].name() != tr.Cells[0].Value {
				return fmt.Errorf("row %d is not %q", i+1, tr.Cells[0].Value)
			}
		}
		return nil
	}
}

func checkNotIndented(rows []accountRow) error {
	for _, row := range rows {
		if !strings.HasPrefix(row.text, " ") || strings.HasPrefix(row.text, "  ") {
			return fmt.Errorf("row %q is indented", row.text)
		}
	}
	return nil
}

func checkAllHaveBalances(rows []accountRow) error {
	for _, row := range rows {
		if !strings.HasSuffix(strings.TrimSpace(row.text), "EUR") && !strings.Contains(row.text, "closed ") {
			return fmt.Errorf("row %q has no balance", row.text)
		}
	}
	return nil
}

func checkNoParents(rows []accountRow) error {
	for _, row := range rows {
		for _, other := range rows {
			if strings.HasPrefix(other.name(), row.name()+":") {
				return fmt.Errorf("%q is a parent of %q", row.name(), other.name())
			}
		}
	}
	return nil
}

func checkSameEndColumn(rows []accountRow) error {
	width := func(row accountRow) int { return utf8.RuneCountInString(strings.TrimRight(row.text, " ")) }
	for _, row := range rows {
		if width(row) != width(rows[0]) {
			return fmt.Errorf("row %q ends at %d, the first row at %d", row.text, width(row), width(rows[0]))
		}
	}
	return nil
}

func expectShows(account, balance string) func(rows []accountRow) error {
	return func(rows []accountRow) error {
		row, err := accountNamed(rows, account)
		if err == nil && !strings.Contains(row.text, balance) {
			err = fmt.Errorf("row %q does not show %q", row.text, balance)
		}
		return err
	}
}

func registerAccountsLayoutSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the Accounts panel lists in order$`, func(table *godog.Table) error {
		return w.expectAccounts(expectOrder(table))
	})
	sc.Step(`^no Accounts row is indented$`, w.rowsStep(checkNotIndented))
	sc.Step(`^no Accounts row is a parent or header row$`, w.rowsStep(checkAllHaveBalances))
	sc.Step(`^there are no subtotal rows$`, w.rowsStep(checkNoParents))
	sc.Step(`^every balance in the Accounts panel ends at the same column$`, w.rowsStep(checkSameEndColumn))
	sc.Step(`^"([^"]*)" shows "([^"]*)"$`, func(account, balance string) error {
		return w.expectAccounts(expectShows(account, balance))
	})
}

func registerAccountsStyleSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)" on "([^"]*)" is coloured (\d+)$`, func(text, account string, want int) error {
		return w.expectRowColour(account, text, want, false)
	})
	sc.Step(`^"([^"]*)" on "([^"]*)" is not coloured (\d+)$`, func(text, account string, want int) error {
		return w.expectRowColour(account, text, want, true)
	})
	sc.Step(`^the row "([^"]*)" shows the glyph "([^"]*)" coloured (\d+)$`, func(account, glyph string, want int) error {
		return w.expectRowColour(account, glyph, want, false)
	})
	sc.Step(`^the last Accounts row is "([^"]*)"$`, func(account string) error {
		return w.expectAccounts(func(rows []accountRow) error {
			if got := rows[len(rows)-1].name(); got != account {
				return fmt.Errorf("the last row is %q", got)
			}
			return nil
		})
	})
	sc.Step(`^it shows "([^"]*)" in the balance column$`, func(text string) error {
		return w.expectAccounts(func(rows []accountRow) error {
			if last := rows[len(rows)-1]; !strings.HasSuffix(strings.TrimSpace(last.text), text) {
				return fmt.Errorf("the last row %q does not end in %q", last.text, text)
			}
			return nil
		})
	})
	sc.Step(`^the row is coloured (\d+)$`, func(want int) error {
		return w.expectAccounts(func(rows []accountRow) error {
			last := rows[len(rows)-1]
			return w.expectRowColour(last.name(), last.name(), want, false)
		})
	})
	sc.Step(`^the row "([^"]*)" is selected with background (\d+) and foreground (\d+)$`, w.expectSelected)
	sc.Step(`^the row "([^"]*)" is selected$`, func(account string) error { return w.expectSelected(account, selectionBackground, -1) })
}

func registerAccountsPagingSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the selected row is more than 1 row below the first$`, func() error {
		return w.expectAccounts(func(rows []accountRow) error {
			selected, err := w.selectedRow(rows)
			if err == nil && selected.y-rows[0].y <= 1 {
				err = fmt.Errorf("the selected row %q is not below the second row", selected.text)
			}
			return err
		})
	})
	sc.Step(`^the selected row is visible$`, func() error {
		return w.expectAccounts(func(rows []accountRow) error { _, err := w.selectedRow(rows); return err })
	})
	sc.Step(`^the last account is selected and visible$`, func() error {
		return w.expectAccounts(func(rows []accountRow) error {
			selected, err := w.selectedRow(rows)
			if err == nil && selected.y != rows[len(rows)-1].y {
				err = fmt.Errorf("the selected row %q is not the last one shown", selected.text)
			}
			return err
		})
	})
}

// declare appends account directives to the journal, as a hand edit would.
func (w *world) declare(lines ...string) error {
	f, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString("\n" + strings.Join(lines, "\n") + "\n")
	return errors.Join(werr, f.Close())
}

// removeTags strips the tag comment off the named accounts' directives.
func (w *world) removeTags(accounts ...string) error {
	b, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		for _, account := range accounts {
			if strings.HasPrefix(line, "account "+account+" ") {
				lines[i], _, _ = strings.Cut(line, ";")
				lines[i] = strings.TrimRight(lines[i], " ")
			}
		}
	}
	return os.WriteFile(w.journal, []byte(strings.Join(lines, "\n")), 0o644)
}

func (w *world) expectPopupRow(severity, account string) error {
	return w.withPopup(func(p popup) error {
		for _, row := range p.rows {
			if strings.Contains(row, severity) && strings.Contains(row, account) {
				return nil
			}
		}
		return fmt.Errorf("no %s row names %s", severity, account)
	})
}

func (w *world) expectNotInPopup(text string) error {
	time.Sleep(settleDelay)
	return w.withPopup(func(p popup) error {
		if strings.Contains(strings.Join(p.rows, "\n"), text) {
			return fmt.Errorf("the checks panel shows %q", text)
		}
		return nil
	})
}

// settleDelay lets a just-pressed key take effect before asserting something is absent.
const settleDelay = 50 * time.Millisecond

func (w *world) withDeclared(name string, lines ...string) error {
	if err := w.useJournal(name, ""); err != nil {
		return err
	}
	return w.declare(lines...)
}

func registerAccountsFixtureSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal "([^"]*)" with "([^"]*)" declared as "([^"]*)"$`, func(name, account, tag string) error {
		return w.withDeclared(name, "account "+account+"  ; "+tag)
	})
	sc.Step(`^the journal "([^"]*)" with (\d+) open import accounts$`, func(name string, n int) error {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = fmt.Sprintf("account assets:extra-%02d  ; konto:source:import, konto:iban:DE00%016d", i+1, i+1)
		}
		return w.withDeclared(name, lines...)
	})
	sc.Step(`^the journal "([^"]*)" with the tags? removed from "([^"]*)"(?: and "([^"]*)")?$`, func(name, first, second string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.removeTags(first, second)
	})
	sc.Step(`^I see a "(WARN|ERR)" row naming "([^"]*)"$`, w.expectPopupRow)
	sc.Step(`^I do not see "([^"]*)" in the checks panel$`, w.expectNotInPopup)
}

func registerAccountsPanelSteps(sc *godog.ScenarioContext, w *world) {
	registerAccountsLayoutSteps(sc, w)
	registerAccountsStyleSteps(sc, w)
	registerAccountsPagingSteps(sc, w)
	registerAccountsFixtureSteps(sc, w)
}
