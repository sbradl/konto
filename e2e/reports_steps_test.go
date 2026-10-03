//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// tableRows are the rows of the panel titled title, trailing blanks dropped.
func (w *world) tableRows(title string) ([]recentRow, error) {
	rows, err := w.listPanel(title)
	if err != nil {
		return nil, err
	}
	for len(rows) > 0 && strings.TrimSpace(rows[len(rows)-1].text) == "" {
		rows = rows[:len(rows)-1]
	}
	return rows, nil
}

// expectTable waits until check accepts the rows of the panel titled title.
func (w *world) expectTable(title string, check func(rows []recentRow) error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		rows, perr := w.tableRows(title)
		if perr != nil {
			failure = perr
			return false
		}
		failure = check(rows)
		return failure == nil
	})
	if failure != nil {
		return fmt.Errorf("%w\n%s", failure, s.screenText())
	}
	return nil
}

const (
	sheetTitle     = "BALANCE SHEET"
	statementTitle = "INCOME STATEMENT"
)

func rowFor(rows []recentRow, label string) (recentRow, error) {
	for _, row := range rows {
		if strings.Contains(row.text, label) {
			return row, nil
		}
	}
	return recentRow{}, fmt.Errorf("no row for %q", label)
}

// expectTableShows checks each table row has its account and every figure on one row.
func (w *world) expectTableShows(title string, table *godog.Table) error {
	return w.expectTable(title, func(rows []recentRow) error {
		for _, want := range table.Rows[1:] {
			row, err := rowFor(rows, want.Cells[0].Value)
			if err != nil {
				return err
			}
			for _, cell := range want.Cells[1:] {
				if !strings.Contains(row.text, cell.Value) {
					return fmt.Errorf("row %q lacks %q", row.text, cell.Value)
				}
			}
		}
		return nil
	})
}

func (w *world) expectRowHolds(title, label string, figures ...string) error {
	return w.expectTable(title, func(rows []recentRow) error {
		row, err := rowFor(rows, label)
		if err != nil {
			return err
		}
		for _, figure := range figures {
			if !strings.Contains(row.text, figure) {
				return fmt.Errorf("row %q lacks %q", row.text, figure)
			}
		}
		return nil
	})
}

func (w *world) expectNotListed(title string, labels ...string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	rows, err := w.tableRows(title)
	if err != nil {
		return err
	}
	for _, label := range labels {
		if _, found := rowFor(rows, label); found == nil {
			return fmt.Errorf("%s lists %q", title, label)
		}
	}
	return nil
}

// expectFigureColour checks the colour of figure within the row for label.
func (w *world) expectFigureColour(title, label, figure string, want int) error {
	if err := w.expectRowHolds(title, label, figure); err != nil {
		return err
	}
	rows, err := w.tableRows(title)
	if err != nil {
		return err
	}
	row, _ := rowFor(rows, label)
	before := row.text[:strings.LastIndex(row.text, figure)]
	s, err := w.session()
	if err != nil {
		return err
	}
	if got := s.cellColour(row.x+1+len([]rune(before)), row.y); got != want {
		return fmt.Errorf("%q in row %q is coloured %d, want %d", figure, row.text, got, want)
	}
	return nil
}

func (w *world) expectHeaderStyle(label string, colour int) error {
	if err := w.expectRowHolds(sheetTitle, label); err != nil {
		return err
	}
	rows, _ := w.tableRows(sheetTitle)
	row, _ := rowFor(rows, label)
	before, _, _ := strings.Cut(row.text, label)
	x := row.x + 1 + len([]rune(before))
	if got := w.s.cellColour(x, row.y); got != colour || !w.s.cellBold(x, row.y) {
		return fmt.Errorf("header %q is coloured %d (bold %v), want %d and bold", label, got, w.s.cellBold(x, row.y), colour)
	}
	return nil
}

func (w *world) expectIncomeNet(want string) error {
	return w.expectTable(statementTitle, func(rows []recentRow) error {
		if len(rows) == 0 && want == "0,00 EUR" {
			return nil // a period without activity has an empty statement, which nets to nothing
		}
		row, err := rowFor(rows, "Net")
		if err != nil {
			return err
		}
		if !strings.Contains(row.text, want) {
			return fmt.Errorf("net row %q lacks %q", row.text, want)
		}
		return nil
	})
}

func (w *world) expectPeriod(want string) error { return w.expectScreen(true, "period  "+want) }

func (w *world) expectBothTablesScrolled() error {
	return w.expectTable(statementTitle, func(rows []recentRow) error {
		if len(rows) == 0 || strings.Contains(rows[0].text, "Income") {
			return errors.New("the income statement has not scrolled")
		}
		return nil
	})
}

func (w *world) expectFirstRowsVisible() error {
	return w.expectTable(sheetTitle, func(rows []recentRow) error {
		if _, err := rowFor(rows, "Assets"); err != nil {
			return err
		}
		return nil
	})
}

func (w *world) expectLastRowsVisible() error {
	if err := w.expectTable(statementTitle, func(rows []recentRow) error {
		_, err := rowFor(rows, "Net")
		return err
	}); err != nil {
		return err
	}
	return w.expectTable(sheetTitle, func(rows []recentRow) error {
		_, err := rowFor(rows, "Net worth")
		return err
	})
}

func (w *world) expectTablesOverflow() error {
	return w.expectTable(statementTitle, func(rows []recentRow) error {
		if len(rows) == 0 {
			return errors.New("the statement has no rows yet")
		}
		last := strings.TrimSpace(rows[len(rows)-1].text)
		var hidden int
		if _, err := fmt.Sscanf(last, "… %d more", &hidden); err != nil || hidden <= 0 {
			return fmt.Errorf("the last statement row is %q, want … N more with N > 0", last)
		}
		return nil
	})
}

func (w *world) expectNoHighlightInBody() error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	lines := s.screenLines()
	for y := 2; y < len(lines)-1; y++ {
		for x := range []rune(lines[y]) {
			if s.cellBackground(x, y) == selectionBackground {
				return fmt.Errorf("cell %d,%d is highlighted", x, y)
			}
		}
	}
	return nil
}

var isoDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

func (w *world) expectNoDateAfter(limit string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	for _, date := range isoDate.FindAllString(s.screenText(), -1) {
		if date > limit && !strings.Contains(w.journal, date) {
			return fmt.Errorf("the screen shows %s, after %s", date, limit)
		}
	}
	return nil
}

func (w *world) expectSameRow(a, b string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	if !s.waitFor(w.suite.timeout, func(string) bool {
		for _, line := range s.screenLines() {
			if strings.Contains(line, a) && strings.Contains(line, b) {
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("%q and %q are not on one row:\n%s", a, b, s.screenText())
	}
	return nil
}

func (w *world) sheetRowHolds(label, figure string) error {
	return w.expectRowHolds(sheetTitle, label, figure)
}

func (w *world) sheetColumnAbsent(title string) error {
	return w.expectTable(sheetTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, title) {
				return fmt.Errorf("the balance sheet has a %q column", title)
			}
		}
		return nil
	})
}

func (w *world) statementRowsLack(chars string) error {
	return w.expectTable(statementTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.ContainsAny(row.text, chars) {
				return fmt.Errorf("row %q shows one of %q", row.text, chars)
			}
		}
		return nil
	})
}

func (w *world) statementIsEmpty() error {
	return w.expectTable(statementTitle, func(rows []recentRow) error {
		if len(rows) != 0 {
			return fmt.Errorf("the income statement has %d rows", len(rows))
		}
		return nil
	})
}

func (w *world) expectNoneOf(a, b, c, d string) error {
	for _, text := range []string{a, b, c, d} {
		if err := w.expectScreen(false, text); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) expectWalletCurrencies(label, text string) error {
	return w.expectTable(sheetTitle, func(rows []recentRow) error {
		row, err := rowFor(rows, label)
		if err != nil {
			return err
		}
		allCurrencies := strings.Contains(row.text, "PLN") && strings.Contains(row.text, "CZK")
		if !strings.Contains(row.text, text) && !allCurrencies {
			return fmt.Errorf("row %q shows neither %q nor every currency", row.text, text)
		}
		return nil
	})
}

func (w *world) useManyCategories(name string, n int) error {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "\naccount expenses:cat%02d\n", i)
	}
	for i := range n {
		fmt.Fprintf(&b, "\n2026-09-%02d Shop %02d\n    expenses:cat%02d  1,00 EUR\n    equity:opening-balances\n", 1+i%28, i, i)
	}
	return w.useJournal(name, b.String())
}

func (w *world) sheetDelta(figure, label string, colour int) error {
	return w.expectFigureColour(sheetTitle, label, figure, colour)
}

func (w *world) liabilityColour(figure string, colour int) error {
	return w.expectFigureColour(sheetTitle, "liabilities:", figure, colour)
}

func (w *world) grandTotalWhite() error {
	return w.expectFigureColour(sheetTitle, "Net worth", "Net worth", 15)
}

func (w *world) netWorthShows(figure string) error { return w.sheetRowHolds("Net worth", figure) }

func (w *world) statementRow(label, figure string) error {
	return w.expectRowHolds(statementTitle, label, figure)
}

func (w *world) totalIncome(figure string) error { return w.statementRow("Total income", figure) }

func (w *world) totalExpenses(figure string) error { return w.statementRow("Total expenses", figure) }

func (w *world) sheetNotListed(label string) error { return w.expectNotListed(sheetTitle, label) }

func (w *world) statementNotListed(label string) error {
	return w.expectNotListed(statementTitle, label)
}

func (w *world) statementNotListedEither(a, b string) error {
	return w.expectNotListed(statementTitle, a, b)
}

func (w *world) sheetShows(t *godog.Table) error { return w.expectTableShows(sheetTitle, t) }

func (w *world) statementShows(t *godog.Table) error { return w.expectTableShows(statementTitle, t) }

func (w *world) anyNoHighlight(int) error { return w.expectNoHighlightInBody() }

func registerReportsTableSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the report period reads "([^"]*)"$`, w.expectPeriod)
	sc.Step(`^the balance sheet shows$`, w.sheetShows)
	sc.Step(`^the income statement shows$`, w.statementShows)
	sc.Step(`^the balance sheet shows "([^"]*)" with "([^"]*)"$`, w.sheetRowHolds)
	sc.Step(`^"([^"]*)" shows the balance "([^"]*)"$`, w.sheetRowHolds)
	sc.Step(`^the balance of "([^"]*)" is "([^"]*)"$`, w.sheetRowHolds)
	sc.Step(`^the balance sheet lists "([^"]*)" with "([^"]*)"$`, w.sheetRowHolds)
	sc.Step(`^the balance sheet's grand total shows "([^"]*)"$`, w.netWorthShows)
	sc.Step(`^the balance sheet has no "([^"]*)" column$`, w.sheetColumnAbsent)
	sc.Step(`^the balance sheet does not list "([^"]*)"$`, w.sheetNotListed)
	sc.Step(`^the balance of "([^"]*)" shows "([^"]*)" or all its currencies$`, w.expectWalletCurrencies)
}

func registerReportsStatementSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the income statement does not list "([^"]*)" or "([^"]*)"$`, w.statementNotListedEither)
	sc.Step(`^the income statement does not list "([^"]*)"$`, w.statementNotListed)
	sc.Step(`^the income statement total income is "([^"]*)"$`, w.totalIncome)
	sc.Step(`^the income statement total expenses is "([^"]*)"$`, w.totalExpenses)
	sc.Step(`^the income statement net is "([^"]*)"$`, w.expectIncomeNet)
	sc.Step(`^the income statement is empty$`, w.statementIsEmpty)
	sc.Step(`^the income statement has exactly 2 columns$`, func() error { return w.statementRowsLack("Δ%") })
	sc.Step(`^I do not see "%" in the income statement$`, func() error { return w.statementRowsLack("%") })
}

func registerReportsStyleSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)" in the Δ column of "([^"]*)" is coloured (\d+)$`, w.sheetDelta)
	sc.Step(`^the Δ of "([^"]*)" is "([^"]*)" coloured (\d+)$`, func(label, figure string, colour int) error {
		return w.sheetDelta(figure, label, colour)
	})
	sc.Step(`^the section header "([^"]*)" is coloured (\d+) and bold$`, w.expectHeaderStyle)
	sc.Step(`^the liability balance "([^"]*)" is coloured (\d+)$`, w.liabilityColour)
	sc.Step(`^the grand total is bright white$`, w.grandTotalWhite)
}

func registerReportsLayoutSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal "([^"]*)" with (\d+) expense categories used this month$`, w.useManyCategories)
	sc.Step(`^both tables have scrolled by 3 rows$`, w.expectBothTablesScrolled)
	sc.Step(`^the first row of each table is visible$`, w.expectFirstRowsVisible)
	sc.Step(`^the last row of each table is visible$`, w.expectLastRowsVisible)
	sc.Step(`^the tables end with "… N more" where N > 0$`, w.expectTablesOverflow)
	sc.Step(`^no row is highlighted with background (\d+)$`, w.anyNoHighlight)
	sc.Step(`^I see no figure dated after "([^"]*)"$`, w.expectNoDateAfter)
	sc.Step(`^I see "([^"]*)" and "([^"]*)" on the same screen row$`, w.expectSameRow)
	sc.Step(`^I do not see "([^"]*)" or "([^"]*)" or "([^"]*)" or "([^"]*)"$`, w.expectNoneOf)
	sc.Step(`^I do not see the word "([^"]*)"$`, func(word string) error { return w.expectScreen(false, word) })
}

func registerReportsSteps(sc *godog.ScenarioContext, w *world) {
	registerReportsTableSteps(sc, w)
	registerReportsStatementSteps(sc, w)
	registerReportsStyleSteps(sc, w)
	registerReportsLayoutSteps(sc, w)
}
