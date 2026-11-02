//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// recentRow is one row of the Recent panel as drawn.
type recentRow struct {
	y, x int // the screen row, and the column of the panel's left border
	text string
}

// recentPanel reads the Recent panel's rows: those between its title and its bottom border.
func (w *world) recentPanel() ([]recentRow, error) { return w.listPanel("RECENT") }

// listPanel reads the rows of the panel titled title.
func (w *world) listPanel(title string) ([]recentRow, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	lines := s.screenLines()
	var rows []recentRow
	left, inside := 0, false
	for y, line := range lines {
		runes := []rune(line)
		if before, _, found := strings.Cut(line, "╭─ "+title+" "); found {
			left, inside = len([]rune(before)), true
			continue
		}
		if !inside {
			continue
		}
		if left < len(runes) && runes[left] == '╰' {
			return rows, nil
		}
		if left < len(runes) {
			text, _, _ := strings.Cut(string(runes[left+1:]), "│")
			rows = append(rows, recentRow{y: y, x: left, text: text})
		}
	}
	return nil, errors.New("no " + title + " panel on screen:\n" + s.screenText())
}

// expectRecent waits until check accepts the panel's non-empty rows.
func (w *world) expectRecent(check func(rows []recentRow) error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		rows, perr := w.recentPanel()
		if perr != nil {
			failure = perr
			return false
		}
		var filled []recentRow
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

// recentFor is the row with the description (and date, when given).
func recentFor(rows []recentRow, description, date string) (recentRow, error) {
	for _, row := range rows {
		if strings.Contains(row.text, description) && (date == "" || strings.Contains(row.text, date)) {
			return row, nil
		}
	}
	return recentRow{}, fmt.Errorf("no Recent row for %q on %q", description, date)
}

func (w *world) expectRecentOrder(count int, date, description string) error {
	return w.expectRecent(func(rows []recentRow) error {
		for i := range count {
			if i >= len(rows) || !strings.Contains(rows[i].text, date) || !strings.Contains(rows[i].text, description) {
				return fmt.Errorf("row %d is not dated %s and %s", i+1, date, description)
			}
		}
		return nil
	})
}

func (w *world) expectRecentDatesDescend() error {
	return w.expectRecent(func(rows []recentRow) error {
		var dates []string
		for _, row := range rows {
			if fields := strings.Fields(row.text); len(fields) > 0 && len(fields[0]) == len("2026-09-26") {
				dates = append(dates, fields[0])
			}
		}
		if !slices.IsSortedFunc(dates, func(a, b string) int { return strings.Compare(b, a) }) {
			return fmt.Errorf("dates are not in non-increasing order: %v", dates)
		}
		return nil
	})
}

func (w *world) expectRecentCategory(description, date, category string) error {
	return w.expectRecent(func(rows []recentRow) error {
		row, err := recentFor(rows, description, date)
		if err != nil {
			return err
		}
		w.focusedRecent = row
		if !strings.Contains(row.text, category) {
			return fmt.Errorf("row %q does not show category %q", row.text, category)
		}
		return nil
	})
}

func (w *world) expectRecentAmountColour(text string, want int, negate bool) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	row := w.focusedRecent
	before, _, found := strings.Cut(row.text, text)
	if !found {
		return fmt.Errorf("row %q does not show %q", row.text, text)
	}
	got := s.cellColour(row.x+1+len([]rune(before)), row.y)
	if (got == want) == negate {
		return fmt.Errorf("%q is coloured %d, negated=%v want %d", text, got, negate, want)
	}
	return nil
}

func (w *world) expectEveryRecentRowShows(description, text string) error {
	return w.expectRecent(func(rows []recentRow) error {
		seen := 0
		for _, row := range rows {
			if strings.Contains(row.text, description) {
				seen++
				if !strings.Contains(row.text, text) {
					return fmt.Errorf("row %q does not show %q", row.text, text)
				}
			}
		}
		if seen == 0 {
			return fmt.Errorf("no Recent row for %q", description)
		}
		return nil
	})
}

func (w *world) expectNoRecentHighlight() error {
	w.s.settle(w.suite.timeout)
	rows, err := w.recentPanel()
	if err != nil {
		return err
	}
	for _, row := range rows {
		for i := range []rune(row.text) {
			if w.s.cellBackground(row.x+1+i, row.y) == selectionBackground {
				return fmt.Errorf("Recent row %q is highlighted", row.text)
			}
		}
	}
	return nil
}

func (w *world) expectNoQuestionMarkInRecent() error {
	return w.expectRecent(func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, "?") {
				return fmt.Errorf("Recent row %q has a ?", row.text)
			}
		}
		return nil
	})
}

func (w *world) expectRecentOverflow() error {
	return w.expectRecent(func(rows []recentRow) error {
		last := strings.TrimSpace(rows[len(rows)-1].text)
		var hidden int
		if _, err := fmt.Sscanf(last, "… %d more", &hidden); err != nil || hidden <= 0 {
			return fmt.Errorf("the last Recent row is %q, want … N more with N > 0", last)
		}
		return nil
	})
}

func registerRecentSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the Recent panel's first row is dated "([^"]*)"$`, func(date string) error {
		return w.expectRecent(func(rows []recentRow) error {
			if len(rows) == 0 || !strings.Contains(rows[0].text, date) {
				return fmt.Errorf("the first Recent row is not dated %s", date)
			}
			return nil
		})
	})
	step(sc, `^the Recent panel's first two rows are dated "([^"]*)" and describe "([^"]*)"$`, func(date, description string) error {
		return w.expectRecentOrder(2, date, description)
	})
	step(sc, `^the third row is dated "([^"]*)" and describes "([^"]*)"$`, func(date, description string) error {
		return w.expectRecent(func(rows []recentRow) error {
			if len(rows) < 3 || !strings.Contains(rows[2].text, date) || !strings.Contains(rows[2].text, description) {
				return fmt.Errorf("the third row is not dated %s and %s", date, description)
			}
			return nil
		})
	})
	step(sc, `^the rows are in non-increasing date order$`, w.expectRecentDatesDescend)
	step(sc, `^the Recent row for "([^"]*)" on "([^"]*)" shows category "([^"]*)"$`, w.expectRecentCategory)
	step(sc, `^it shows "([^"]*)" coloured (\d+)$`, func(text string, colour int) error {
		return w.expectRecentAmountColour(text, colour, false)
	})
	step(sc, `^its amount is not coloured (\d+)$`, func(colour int) error {
		fields := strings.Fields(w.focusedRecent.text)
		return w.expectRecentAmountColour(strings.Join(fields[len(fields)-2:], " "), colour, true)
	})
	step(sc, `^the Recent rows for "([^"]*)" show "([^"]*)"$`, w.expectEveryRecentRowShows)
	step(sc, `^no Recent row is highlighted with background (\d+)$`, func(int) error { return w.expectNoRecentHighlight() })
	step(sc, `^I do not see "\?" in any Recent row$`, w.expectNoQuestionMarkInRecent)
	step(sc, `^the Recent panel ends with "… N more" where N > 0$`, w.expectRecentOverflow)
	step(sc, `^the journal "([^"]*)" with (\d+) transactions$`, func(name string, n int) error {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, "\n2026-09-%02d Bulk %03d\n    expenses:groceries  1,00 EUR\n    equity:opening-balances\n", 1+i%28, i)
		}
		return w.useJournal(name, b.String())
	})
}

// expectUpcoming waits until check accepts the Upcoming panel's non-empty rows.
func (w *world) expectUpcoming(check func(rows []recentRow) error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		rows, perr := w.listPanel("UPCOMING")
		if perr != nil {
			failure = perr
			return false
		}
		var filled []recentRow
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

func (w *world) expectUpcomingOrder(table *godog.Table) error {
	return w.expectUpcoming(func(rows []recentRow) error {
		for i, want := range table.Rows {
			if i >= len(rows) {
				return fmt.Errorf("Upcoming has %d rows, want at least %d", len(rows), len(table.Rows))
			}
			for _, cell := range want.Cells {
				if !strings.Contains(rows[i].text, cell.Value) {
					return fmt.Errorf("Upcoming row %d is %q, want it to hold %q", i+1, rows[i].text, cell.Value)
				}
			}
		}
		return nil
	})
}

func (w *world) expectUpcomingEmpty() error {
	return w.expectUpcoming(func(rows []recentRow) error {
		if len(rows) != 0 {
			return fmt.Errorf("Upcoming has %d rows, want none", len(rows))
		}
		return nil
	})
}

func (w *world) expectNoUpcomingHighlight() error {
	w.s.settle(w.suite.timeout)
	rows, err := w.listPanel("UPCOMING")
	if err != nil {
		return err
	}
	for _, row := range rows {
		for i := range []rune(row.text) {
			if w.s.cellBackground(row.x+1+i, row.y) == selectionBackground {
				return fmt.Errorf("Upcoming row %q is highlighted", row.text)
			}
		}
	}
	return nil
}

func registerUpcomingSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the Upcoming panel lists in order$`, w.expectUpcomingOrder)
	step(sc, `^the Upcoming panel is empty$`, w.expectUpcomingEmpty)
	step(sc, `^no Upcoming row is highlighted with background (\d+)$`, func(int) error { return w.expectNoUpcomingHighlight() })
	step(sc, `^the transaction count in the tab bar is still (\d+)$`, func(count string) error {
		return w.expectLine(0, "tab bar", count+" txns", strings.Contains)
	})
	step(sc, `^the status bar shows no error indicator$`, func() error {
		return w.expectLine(-1, "status bar", "✗", func(line, marker string) bool { return !strings.Contains(line, marker) })
	})
}
