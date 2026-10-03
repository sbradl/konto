//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

const (
	rulesTitle       = "RULES"
	entryTitle       = "ENTRY"
	occurrencesTitle = "OCCURRENCES"
	impactTitle      = "ANNUAL IMPACT"
)

// filledRows are the non-blank rows of the panel titled title.
func filledRows(rows []recentRow) []recentRow {
	var filled []recentRow
	for _, row := range rows {
		if strings.TrimSpace(row.text) != "" {
			filled = append(filled, row)
		}
	}
	return filled
}

// expectPanel waits until check accepts the non-blank rows of the panel titled title.
func (w *world) expectPanel(title string, check func(rows []recentRow) error) error {
	return w.expectTable(title, func(rows []recentRow) error { return check(filledRows(rows)) })
}

func (w *world) expectRuleOrder(table *godog.Table) error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		for i, want := range table.Rows[1:] {
			if i >= len(rows) {
				return fmt.Errorf("the list has %d rules, want at least %d", len(rows), len(table.Rows)-1)
			}
			position := 0
			for _, cell := range want.Cells {
				at := strings.Index(rows[i].text[position:], cell.Value)
				if at < 0 {
					return fmt.Errorf("rule row %d is %q, want %q in order", i+1, rows[i].text, cell.Value)
				}
				position += at + len(cell.Value)
			}
		}
		return nil
	})
}

// selectedRule is the index of the highlighted rule among the list rows.
func (w *world) selectedRule(rows []recentRow) (int, error) {
	s, err := w.session()
	if err != nil {
		return 0, err
	}
	for i, row := range rows {
		if s.cellBackground(row.x+2, row.y) == selectionBackground {
			if fg := s.cellColour(row.x+2, row.y); fg != 15 && fg != -1 {
				return 0, fmt.Errorf("the selected rule has foreground %d, want 15", fg)
			}
			return i, nil
		}
	}
	return 0, errors.New("no rule is selected")
}

func (w *world) expectRuleSelected(which string) error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		got, err := w.selectedRule(rows)
		if err != nil {
			return err
		}
		want := map[string]int{"first": 0, "second": 1, "last": len(rows) - 1}[which]
		if got != want {
			return fmt.Errorf("rule %d is selected, want the %s (%d)", got, which, want)
		}
		return nil
	})
}

func (w *world) expectRuleStatus(description, status string) error {
	return w.expectRowHolds(rulesTitle, description, status)
}

func (w *world) expectEntry(doc *godog.DocString) error {
	want := strings.Split(strings.TrimSpace(doc.Content), "\n")
	return w.expectPanel(entryTitle, func(rows []recentRow) error {
		if len(rows) < len(want) {
			return fmt.Errorf("the entry panel has %d lines, want %d", len(rows), len(want))
		}
		for i, line := range want {
			if strings.TrimSpace(rows[i].text) != strings.TrimSpace(line) {
				return fmt.Errorf("entry line %d is %q, want %q", i+1, rows[i].text, line)
			}
		}
		return nil
	})
}

func (w *world) expectEntryColour(text string, colour int) error {
	return w.expectFigureColour(entryTitle, text, text, colour)
}

func (w *world) expectOccurrences(table *godog.Table, exactly bool) error {
	return w.expectPanel(occurrencesTitle, func(rows []recentRow) error {
		if exactly && len(rows) != len(table.Rows) {
			return fmt.Errorf("%d occurrences listed, want exactly %d", len(rows), len(table.Rows))
		}
		for i, want := range table.Rows {
			if i >= len(rows) || !strings.Contains(rows[i].text, want.Cells[0].Value) {
				return fmt.Errorf("occurrence %d is not %s", i+1, want.Cells[0].Value)
			}
		}
		return nil
	})
}

func (w *world) expectFirstOccurrence(date string) error {
	return w.expectPanel(occurrencesTitle, func(rows []recentRow) error {
		if len(rows) == 0 || !strings.Contains(rows[0].text, date) {
			return fmt.Errorf("the first occurrence is not %s", date)
		}
		return nil
	})
}

func (w *world) expectImpact(label, figure string, colour int) error {
	return w.expectFigureColour(impactTitle, label, figure, colour)
}

func (w *world) expectNoneOf3(a, b, c string) error {
	line, err := w.statusLine()
	if err != nil {
		return err
	}
	for _, text := range []string{a, b, c} {
		if strings.Contains(line, text) {
			return fmt.Errorf("the status bar %q lists %q", line, text)
		}
	}
	return nil
}

func (w *world) expectNoneOf2(a, b string) error {
	for _, text := range []string{a, b} {
		if err := w.expectScreen(false, text); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) expectTabCount(count string) error {
	return w.expectLine(0, "tab bar", count+" txns", strings.Contains)
}

func (w *world) expectRuleListEmpty() error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, "monthly") || strings.Contains(row.text, "upcoming") {
				return fmt.Errorf("the list shows %q, want no rules", row.text)
			}
		}
		return nil
	})
}

// selectRule moves the cursor down to the rule with the description.
func (w *world) selectRule(description string) error {
	var target int
	if err := w.expectPanel(rulesTitle, func(rows []recentRow) error {
		for i, row := range rows {
			if strings.Contains(row.text, description) {
				target = i
				return nil
			}
		}
		return fmt.Errorf("no rule %q is listed", description)
	}); err != nil {
		return err
	}
	for range target {
		if err := w.press("↓"); err != nil {
			return err
		}
	}
	return nil
}

// useIncludedRule installs a journal whose only `~` rule lives in an included file.
func (w *world) useIncludedRule(name, file string) error {
	rule := "~ monthly from 2026-11-01  Included rule\n    expenses:rent  5,00 EUR\n    assets:checking\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(w.journal), file), []byte(rule), 0o644); err != nil {
		return err
	}
	return w.useJournal(name, "\ninclude "+file+"\n")
}

func registerScheduledSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the rule list shows in order$`, w.expectRuleOrder)
	sc.Step(`^the (first|second|last) rule is selected(?: with background \d+ and foreground \d+)?$`, w.expectRuleSelected)
	sc.Step(`^the status of "([^"]*)" is "([^"]*)"$`, w.expectRuleStatus)
	sc.Step(`^the next occurrence of "([^"]*)" is "([^"]*)"$`, w.expectRuleStatus)
	sc.Step(`^the entry panel shows$`, w.expectEntry)
	sc.Step(`^the entry panel shows "([^"]*)" coloured (\d+)$`, w.expectEntryColour)
	sc.Step(`^the "~" line is coloured (\d+)$`, func(colour int) error { return w.expectEntryColour("~", colour) })
	sc.Step(`^the occurrences panel lists$`, func(t *godog.Table) error { return w.expectOccurrences(t, false) })
	sc.Step(`^the occurrences panel lists exactly$`, func(t *godog.Table) error { return w.expectOccurrences(t, true) })
	sc.Step(`^the occurrences panel's first date is "([^"]*)"$`, w.expectFirstOccurrence)
	sc.Step(`^the annual impact table shows "([^"]*)" with "([^"]*)" coloured (\d+)$`, w.expectImpact)
	sc.Step(`^it shows "([^"]*)" with "([^"]*)" coloured (\d+)$`, w.expectImpact)
	sc.Step(`^the annual impact total is "([^"]*)" coloured (\d+)$`, func(figure string, colour int) error {
		return w.expectImpact("Total", figure, colour)
	})
	sc.Step(`^there is no valuation toggle$`, func() error { return w.expectNoneOf2("valuation", "market") })
	sc.Step(`^I do not see "([^"]*)" or "([^"]*)"$`, w.expectNoneOf2)
	sc.Step(`^the status bar does not list "([^"]*)" or "([^"]*)" or "([^"]*)"$`, w.expectNoneOf3)
	sc.Step(`^the tab bar shows "(\d+)" transactions$`, w.expectTabCount)
	sc.Step(`^I select the rule "([^"]*)"$`, w.selectRule)
	sc.Step(`^the journal "([^"]*)" with a rule in included file "([^"]*)"$`, w.useIncludedRule)
	sc.Step(`^the rule is listed$`, func() error { return w.expectRowHolds(rulesTitle, "Included rule") })
	sc.Step(`^the rule list is empty$`, w.expectRuleListEmpty)
	sc.Step(`^I see a hint to press "n" to add a rule$`, func() error { return w.expectScreen(true, "press n to add a rule") })
}
