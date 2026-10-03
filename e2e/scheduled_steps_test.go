//go:build e2e

package e2e

import (
	"bytes"
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
	if err := w.eventually(func() error { // the list must be the reloaded one, or the rule is not where it was
		if line, err := w.statusLine(); err != nil || strings.Contains(line, "⣾") {
			return errors.New("still reloading")
		}
		return nil
	}); err != nil {
		return err
	}
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
	return w.press(append([]string{"Home"}, repeat("↓", target)...)...)
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

// ruleBlocks are the `~` blocks of text: a rule line and the indented lines under it.
func ruleBlocks(text string) []string {
	var blocks []string
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "~") {
			continue
		}
		block := []string{lines[i]}
		for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			block = append(block, lines[i])
		}
		blocks = append(blocks, strings.Join(block, "\n"))
	}
	return blocks
}

func allJournalText(files map[string][]byte) string {
	var all strings.Builder
	for _, content := range files {
		all.Write(content)
		all.WriteByte('\n')
	}
	return all.String()
}

func (w *world) expectRuleCount(n int) error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		if len(rows) != n {
			return fmt.Errorf("the list has %d rules, want %d", len(rows), n)
		}
		return nil
	})
}

func (w *world) expectAnyRuleSelected() error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		_, err := w.selectedRule(rows)
		return err
	})
}

// expectNoRuleLine waits for the journal to hold no rule line mentioning text.
func (w *world) expectNoRuleLine(text string) error {
	return w.eventually(func() error {
		for _, block := range ruleBlocks(allJournalText(w.journalFiles())) {
			header, _, _ := strings.Cut(block, "\n")
			if strings.Contains(header, text) {
				return fmt.Errorf("the journal still has the rule %q", header)
			}
		}
		return nil
	})
}

// expectJournalLacks waits for none of the texts to be in the journal files.
func (w *world) expectJournalLacks(texts ...string) error {
	return w.eventually(func() error {
		all := allJournalText(w.journalFiles())
		for _, text := range texts {
			if strings.Contains(all, text) {
				return fmt.Errorf("the journal still contains %q", text)
			}
		}
		return nil
	})
}

func (w *world) eventually(check func() error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		failure = check()
		return failure == nil
	})
	return failure
}

func (w *world) expectOneBlockRemoved() error {
	return w.eventually(func() error {
		before := ruleBlocks(allJournalText(w.filesAtStart))
		now := allJournalText(w.journalFiles())
		missing := 0
		for _, block := range before {
			if !strings.Contains(now, block) {
				missing++
			}
		}
		if missing != 1 {
			return fmt.Errorf("%d rule blocks changed, want exactly one removed", missing)
		}
		return nil
	})
}

func (w *world) expectRuleIntact(description string) error {
	for _, block := range ruleBlocks(allJournalText(w.filesAtStart)) {
		if header, _, _ := strings.Cut(block, "\n"); strings.Contains(header, description) {
			return w.eventually(func() error {
				if !strings.Contains(allJournalText(w.journalFiles()), block) {
					return fmt.Errorf("the rule %q is not intact", description)
				}
				return nil
			})
		}
	}
	return fmt.Errorf("no rule %q at start", description)
}

func (w *world) expectMainJournalUnchanged() error {
	now, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	if !bytes.Equal(now, w.filesAtStart[w.journal]) {
		return errors.New("the main journal changed")
	}
	return nil
}

func (w *world) expectFileLacksRule(name string) error {
	return w.eventually(func() error {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(w.journal), name))
		if err != nil {
			return err
		}
		if len(ruleBlocks(string(b))) != 0 {
			return fmt.Errorf("%s still holds a rule", name)
		}
		return nil
	})
}

func (w *world) expectUpcomingLacks(description string) error {
	return w.expectUpcoming(func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, description) {
				return fmt.Errorf("Upcoming still lists %q", description)
			}
		}
		return nil
	})
}

func (w *world) expectJournalStillHas(text string) error {
	return w.eventually(func() error {
		if !strings.Contains(allJournalText(w.journalFiles()), text) {
			return fmt.Errorf("the journal no longer contains %q", text)
		}
		return nil
	})
}

func registerScheduledDeleteSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal still contains "([^"]*)"$`, w.expectJournalStillHas)
	sc.Step(`^the rule list has (\d+) rules$`, w.expectRuleCount)
	sc.Step(`^a rule is selected$`, w.expectAnyRuleSelected)
	sc.Step(`^the journal no longer contains "([^"]*)" as a rule$`, w.expectNoRuleLine)
	sc.Step(`^the journal no longer contains "([^"]*)"$`, func(text string) error { return w.expectJournalLacks(text) })
	sc.Step(`^the journal does not contain "([^"]*)" or "([^"]*)"$`, func(a, b string) error { return w.expectJournalLacks(a, b) })
	sc.Step(`^the other four rules are byte-identical$`, w.expectOneBlockRemoved)
	sc.Step(`^the rule "([^"]*)" is intact$`, w.expectRuleIntact)
	sc.Step(`^the main journal is unchanged$`, w.expectMainJournalUnchanged)
	sc.Step(`^"([^"]*)" no longer contains the rule$`, w.expectFileLacksRule)
	sc.Step(`^the Upcoming panel does not list "([^"]*)"$`, w.expectUpcomingLacks)
}
