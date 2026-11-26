//go:build e2e

package e2e

import (
	"fmt"
	"regexp"
	"strings"
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

func rowFor(rows []recentRow, label string) (recentRow, error) {
	for _, row := range rows {
		if strings.Contains(row.text, label) {
			return row, nil
		}
	}

	return recentRow{}, fmt.Errorf("no row for %q", label)
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

var quotedText = regexp.MustCompile(`"([^"]*)"`)

// quoted are the texts a step quotes, in order.
func quotedTexts(list string) []string {
	matches := quotedText.FindAllStringSubmatch(list, -1)
	texts := make([]string, len(matches))
	for i, match := range matches {
		texts[i] = match[1]
	}

	return texts
}
