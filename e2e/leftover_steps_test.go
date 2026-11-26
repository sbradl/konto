//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func (w *world) expectFocusedInputHolds(text string) error {
	return w.onForm(func(rows []string) error {
		for _, row := range rows {
			if strings.HasPrefix(strings.TrimSpace(row), "▸") && strings.Contains(row, text) {
				return nil
			}
		}

		return fmt.Errorf("the focused input does not hold %q: %q", text, rows)
	})
}

// deleteRuleInScheduled deletes the rule on the Scheduled view and waits for the reload.
func (w *world) deleteRuleInScheduled(description string) error {
	if err := w.press("3"); err != nil {
		return err
	}

	if err := w.selectRule(description); err != nil {
		return err
	}

	if err := w.press("d", "y"); err != nil {
		return err
	}

	return w.press("1")
}

// useUndeclaredCash is the household journal with a cash account that is used and never declared, and that
// ends at zero.
func (w *world) useUndeclaredCash(name, account string) error {
	text := fmt.Sprintf("\n2026-09-27 Cash in\n    %s  10,00 EUR\n    equity:opening-balances\n\n"+
		"2026-09-28 Cash out\n    %s  -10,00 EUR\n    equity:opening-balances\n", account, account)

	return w.useJournal(name, text)
}

func (w *world) expectNewDirective(directive, tag string) error {
	return w.eventually(func() error {
		for line := range strings.SplitSeq(allJournalText(w.journalFiles()), "\n") {
			if strings.HasPrefix(line, directive+" ") && strings.Contains(line, tag) {
				return nil
			}
		}

		return fmt.Errorf("no %q directive with %q", directive, tag)
	})
}

// useDirectiveInInclude moves the account's directive out of the main journal into an included file.
func (w *world) useDirectiveInInclude(name, account, file string) error {
	if err := w.useJournal(name, ""); err != nil {
		return err
	}

	text, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	var kept, moved []string
	for line := range strings.SplitSeq(string(text), "\n") {
		if strings.HasPrefix(line, "account "+account+" ") || line == "account "+account {
			moved = append(moved, line)
		} else {
			kept = append(kept, line)
		}
	}

	if err := os.WriteFile(filepath.Join(filepath.Dir(w.journal), file), []byte(strings.Join(moved, "\n")+"\n"), 0o644); err != nil {
		return err
	}

	return os.WriteFile(w.journal, []byte("include "+file+"\n"+strings.Join(kept, "\n")), 0o644)
}

// importAsTransfer opens the file, makes the row a transfer to the account and every other row a category,
// and writes the import.
func (w *world) importAsTransfer(file, description, target string) error {
	if err := w.openUnlessStaged(file); err != nil {
		return err
	}

	if err := w.categoriseOnce(description, "", target); err != nil {
		return err
	}

	if err := w.categoriseEveryRow(); err != nil {
		return err
	}

	return w.writeImport()
}

func (w *world) expectNoCounterpart(account, counterpart string) error {
	return w.eventually(func() error {
		for _, block := range w.journalBlocks(false) {
			if strings.Contains(block, "    "+account) && strings.Contains(block, "    "+counterpart) {
				return fmt.Errorf("a transaction posts %s against %s: %q", account, counterpart, block)
			}
		}

		return nil
	})
}

// openSavingsCSV writes a CSV for the savings account's IBAN and opens it in Import.
func (w *world) openSavingsCSV() error {
	if err := w.writeExport("savings.csv", postbankExport{
		IBAN: "DE00000000000000000005", ClosingBalance: "13.501,00",
		Rows: []postbankRow{{Date: "29.9.2026", Type: "Gutschrift", Payee: "Max", Purpose: "x", Amount: "1,00"}},
	}); err != nil {
		return err
	}

	return w.openInImport("savings.csv")
}

func (w *world) expectNetWorth(value string) error {
	if err := w.press("1"); err != nil {
		return err
	}

	return w.expectCard("NET WORTH", func(card statCard) error {
		if card.value != value {
			return fmt.Errorf("the net worth is %s, want %s", card.value, value)
		}

		return nil
	})
}

func (w *world) expectDashboardShows(account, text string) error {
	if err := w.press("1"); err != nil {
		return err
	}

	return w.expectAccounts(expectShows(account, text))
}

func registerLeftoverSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the focused input contains "([^"]*)"$`, w.expectFocusedInputHolds)
	step(sc, `^I see the row table with (\d+) new rows?$`, func(n int) error { return w.expectPanelRowCount(rowsTitle, n) })
	step(sc, `^I delete the rule "([^"]*)" in Scheduled$`, w.deleteRuleInScheduled)
	step(sc, `^the journal "([^"]*)" where "([^"]*)" is used in a transaction but never declared$`, w.useUndeclaredCash)
	step(sc, `^"([^"]*)" has a zero balance$`, func(string) error { return nil }) // the fixture's two transactions cancel
	step(sc, `^the journal contains a new "([^"]*)" directive with "([^"]*)"$`, w.expectNewDirective)
	step(sc, `^the journal "([^"]*)" where the "([^"]*)" directive lives in included file "([^"]*)"$`, w.useDirectiveInInclude)
	step(sc, `^I import "([^"]*)" categorising "([^"]*)" as the transfer to "([^"]*)"$`, w.importAsTransfer)
	step(sc, `^no transaction in "([^"]*)" has a counterpart "([^"]*)"$`, w.expectNoCounterpart)
	step(sc, `^I open a CSV for the savings IBAN in Import$`, w.openSavingsCSV)
	step(sc, `^the net worth is ([\d.,]+ EUR)$`, w.expectNetWorth)
	step(sc, `^on the dashboard "([^"]*)" shows "([^"]*)"$`, w.expectDashboardShows)
}
