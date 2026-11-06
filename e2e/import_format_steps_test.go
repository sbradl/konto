//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// expectNoRowHolding waits for the table to have no row that holds the text.
func (w *world) expectNoRowHolding(text string) error {
	return w.expectPanel(rowsTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, text) {
				return fmt.Errorf("a row holds %q: %q", text, row.text)
			}
		}

		return nil
	})
}

// newestNewBlock is the imported transaction with the latest date.
func (w *world) newestNewBlock() (string, error) {
	blocks := w.newBlocks(false)
	if len(blocks) == 0 {
		return "", errors.New("nothing was imported")
	}

	return slices.MaxFunc(blocks, func(a, b string) int { return strings.Compare(a[:len("2006-01-02")], b[:len("2006-01-02")]) }), nil
}

func (w *world) expectNewestAssertion(assertion, account string) error {
	return w.eventually(func() error {
		block, err := w.newestNewBlock()
		if err != nil {
			return err
		}

		if !postsWith(block, account, assertion) {
			return fmt.Errorf("the newest imported transaction %q has no %q on %s", block, assertion, account)
		}

		return nil
	})
}

func (w *world) expectNoAssertionMentioning(text string) error {
	return w.eventually(func() error {
		for line := range strings.SplitSeq(allJournalText(w.journalFiles()), "\n") {
			if strings.Contains(line, " = ") && strings.Contains(line, text) {
				return fmt.Errorf("an assertion mentions %q: %q", text, line)
			}
		}

		return nil
	})
}

func (w *world) expectEveryPostingIn(commodity string) error {
	return w.eventually(func() error {
		blocks := w.newBlocks(false)
		if len(blocks) == 0 {
			return errors.New("nothing was imported")
		}
		for _, block := range blocks {
			for line := range strings.SplitSeq(block, "\n") {
				if amount := strings.Fields(line); len(amount) > 1 && strings.ContainsAny(line, "0123456789") &&
					!strings.HasPrefix(line, "20") && !strings.Contains(line, commodity) {
					return fmt.Errorf("a posting is not in %s: %q", commodity, line)
				}
			}
		}

		return nil
	})
}

func registerFormatSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the row "([^"]*)" is dated "([^"]*)" and shows "([^"]*)"$`, func(name, date, text string) error {
		return w.expectImportRowHolds(name, date, text)
	})
	step(sc, `^the row "([^"]*)" is dated "([^"]*)"$`, func(name, date string) error { return w.expectImportRowHolds(name, date) })
	step(sc, `^no row is dated "([^"]*)"$`, w.expectNoRowHolding)
	step(sc, `^no row is named "([^"]*)"$`, w.expectNoRowHolding)
	step(sc, `^the row "([^"]*)" is shown unmangled$`, func(name string) error {
		if err := w.expectImportRowHolds(name, ""); err != nil {
			return err
		}

		return w.expectNoRowHolding("�")
	})
	step(sc, `^the newest imported transaction has the assertion "([^"]*)" on "([^"]*)"$`, w.expectNewestAssertion)
	step(sc, `^no assertion in the journal mentions "([^"]*)"$`, w.expectNoAssertionMentioning)
	step(sc, `^the journal has exactly (\d+) "([^"]*)" transactions on "([^"]*)"$`, func(n int, description, date string) error {
		return w.expectCountOnDate(n, description, "", date)
	})
	step(sc, `^in the journal "([^"]*)" on "([^"]*)" is above "([^"]*)" on "([^"]*)"$`, func(upper, upperDate, lower, lowerDate string) error {
		return w.expectAbove(upperDate+" "+upper, lowerDate+" "+lower)
	})
	step(sc, `^every imported posting is in "([^"]*)"$`, w.expectEveryPostingIn)
}
