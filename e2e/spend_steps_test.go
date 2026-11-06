//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// spendCell is one drawn cell of the SPEND panel, with its screen position.
type spendCell struct {
	x, y int
	r    rune
}

// spendPanel reads the interior of the SPEND panel: the rows between its title and its bottom border.
func (w *world) spendPanel() ([][]spendCell, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	var rows [][]spendCell
	left, inside := 0, false
	for y, line := range s.screenLines() {
		runes := []rune(line)
		if before, _, found := strings.Cut(line, "╭─ SPEND "); found {
			left, inside = len([]rune(before)), true

			continue
		}

		if !inside {
			continue
		}

		if left < len(runes) && runes[left] == '╰' {
			return rows, nil
		}
		var row []spendCell
		for x := left + 1; x < len(runes); x++ {
			row = append(row, spendCell{x: x, y: y, r: runes[x]})
		}
		rows = append(rows, row)
	}

	return nil, errors.New("no SPEND panel on screen:\n" + s.screenText())
}

// spendText is the panel's interior as lines of text, border cells included.
func (w *world) spendText() (string, error) {
	rows, err := w.spendPanel()
	if err != nil {
		return "", err
	}
	lines := make([]string, len(rows))
	for i, row := range rows {
		for _, c := range row {
			lines[i] += string(c.r)
		}
	}

	return strings.Join(lines, "\n"), nil
}

func isBraille(r rune) bool { return r >= 0x2800 && r <= 0x28FF }

// expectSpend waits until check accepts the chart's text.
func (w *world) expectSpend(check func(text string) error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		text, perr := w.spendText()
		if perr != nil {
			failure = perr

			return false
		}
		failure = check(text)

		return failure == nil
	})

	if failure != nil {
		return fmt.Errorf("%w\n%s", failure, s.screenText())
	}

	return nil
}

func (w *world) expectBrailleChart() error {
	return w.expectSpend(func(text string) error {
		if !strings.ContainsFunc(text, isBraille) {
			return errors.New("the SPEND panel draws no braille line")
		}

		return nil
	})
}

var spendQuoted = regexp.MustCompile(`"([^"]*)"`)

func (w *world) expectXAxisLabels(list string) error {
	return w.expectSpend(func(text string) error {
		axis := text[strings.LastIndex(strings.TrimRight(text, "\n"), "\n")+1:]
		position := 0
		for _, match := range spendQuoted.FindAllStringSubmatch(list, -1) {
			at := strings.Index(axis[position:], match[1])
			if at < 0 {
				return fmt.Errorf("x axis %q lacks %q in order", axis, match[1])
			}
			position += at + len(match[1])
		}

		return nil
	})
}

func (w *world) expectYAxisInThousands() error {
	return w.expectSpend(func(text string) error {
		if !regexp.MustCompile(`\d\.\dk│`).MatchString(text) {
			return errors.New("y axis labels are not formatted like 1.2k")
		}

		return nil
	})
}

func (w *world) expectChartColoursBelow16() error {
	if err := w.expectBrailleChart(); err != nil {
		return err
	}

	s, err := w.session()
	if err != nil {
		return err
	}

	rows, err := w.spendPanel()
	if err != nil {
		return err
	}
	for _, row := range rows {
		for _, c := range row {
			if isBraille(c.r) && s.cellColour(c.x, c.y) < 0 {
				return fmt.Errorf("chart cell at %d,%d is not an ANSI-16 colour", c.x, c.y)
			}
		}
	}

	return nil
}

func registerSpendSteps(sc *godog.ScenarioContext, w *world) {
	registerSpendLineSteps(sc, w)
	step(sc, `^I see a braille chart of monthly spend$`, w.expectBrailleChart)
	step(sc, `^the x-axis labels are((?: "[^"]*")+)$`, w.expectXAxisLabels)
	step(sc, `^the x-axis does not include "([^"]*)"$`, func(label string) error {
		return w.expectSpend(func(text string) error {
			if strings.Contains(text, label) {
				return fmt.Errorf("the SPEND panel shows %q", label)
			}

			return nil
		})
	})
	step(sc, `^the y-axis labels are formatted in thousands$`, w.expectYAxisInThousands)
	step(sc, `^every chart cell colour is an ANSI index below 16$`, w.expectChartColoursBelow16)
}
