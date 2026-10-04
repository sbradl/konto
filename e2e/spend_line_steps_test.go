//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// The chart's y axis reaches 1.1 times the highest month; a braille cell holds 4 dot rows.
const (
	spendHeadroom = 1.1
	dotsPerCell   = 4
	dotTolerance  = 1.5 // dots a decoded month may stray from the expected total
	vertexPad     = 0.6 // the x range reaches this far past the last month
)

// householdSpend are the monthly expense totals of the household fixtures, Mar to Aug.
var householdSpend = []float64{1175, 1120, 1195, 1150, 1205, 1130}

// brailleDot is the (column, row) of each dot of a braille cell, by bit.
var brailleDot = [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}

// chartGrid is the plotting area of the SPEND chart: its rows down to the x axis, right of the y axis.
type chartGrid struct {
	rows [][]rune
	left int // index of the first plotting column
}

func (w *world) spendGrid() (chartGrid, error) {
	rows, err := w.spendPanel()
	if err != nil {
		return chartGrid{}, err
	}
	const labelRow = 1 // the axis row stays: it is the zero of the y scale
	grid := chartGrid{}
	for _, row := range rows[:max(0, len(rows)-labelRow)] {
		cells := make([]rune, len(row))
		for i, c := range row {
			cells[i] = c.r
		}
		grid.rows = append(grid.rows, cells)
	}
	if len(grid.rows) == 0 {
		return chartGrid{}, errors.New("the SPEND panel is too small to plot")
	}
	grid.left = strings.IndexRune(string(grid.rows[0]), '│')
	return grid, nil
}

// width is the number of plotting columns, the panel's right border excluded.
func (g chartGrid) width() int { return len(g.rows[0]) - g.left - 2 }

// dotsAt are the lit dot rows (from the top) of the cells at column col.
func (g chartGrid) dotsAt(col int) []int {
	var dots []int
	for y, row := range g.rows {
		if col < 0 || col >= len(row) || !isBraille(row[col]) {
			continue
		}
		for bit, at := range brailleDot {
			if (row[col]-0x2800)&(1<<bit) != 0 {
				dots = append(dots, y*dotsPerCell+at[1])
			}
		}
	}
	return dots
}

// plotted reads the month totals back from the line, given that the highest month is top.
func (g chartGrid) plotted(months int, top float64) ([]float64, error) {
	span := float64(len(g.rows)*dotsPerCell - 1)
	values := make([]float64, months)
	for i := range values {
		col := g.left + 1 + int(math.Round(float64(i)*float64(g.width()-1)/(float64(months-1)+vertexPad)))
		var dots []int
		for reach := 0; len(dots) == 0 && reach < 2; reach++ {
			dots = append(g.dotsAt(col-reach), g.dotsAt(col+reach)...)
		}
		if len(dots) == 0 {
			return nil, fmt.Errorf("the line has no dot at column %d (month %d)", col, i+1)
		}
		sum := 0
		for _, d := range dots {
			sum += d
		}
		values[i] = top * spendHeadroom * (span - float64(sum)/float64(len(dots))) / span
	}
	return values, nil
}

func (w *world) expectPlottedTotals(want []float64) error {
	top := slicesMax(want)
	var failure error
	err := w.expectSpend(func(string) error {
		grid, gerr := w.spendGrid()
		if gerr != nil {
			return gerr
		}
		got, perr := grid.plotted(len(want), top)
		if perr != nil {
			return perr
		}
		step := top * spendHeadroom / float64(len(grid.rows)*dotsPerCell-1)
		for i := range want {
			if math.Abs(got[i]-want[i]) > dotTolerance*step {
				failure = fmt.Errorf("month %d is plotted at about %.0f, want %.0f (±%.0f)",
					i+1, got[i], want[i], dotTolerance*step)
				return failure
			}
		}
		return nil
	})
	return err
}

func slicesMax(values []float64) float64 {
	top := values[0]
	for _, v := range values {
		top = math.Max(top, v)
	}
	return top
}

var plottedNumber = regexp.MustCompile(`\d+`)

func (w *world) expectPlottedList(list string) error {
	numbers := plottedNumber.FindAllString(list, -1)
	want := make([]float64, len(numbers))
	for i, n := range numbers {
		want[i], _ = strconv.ParseFloat(n, 64)
	}
	return w.expectPlottedTotals(want)
}

func (w *world) expectOneSeries() error {
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
	colours := map[int]bool{}
	for _, row := range rows {
		for _, c := range row {
			if isBraille(c.r) {
				colours[s.cellColour(c.x, c.y)] = true
			}
		}
	}
	if len(colours) != 1 {
		return fmt.Errorf("the chart line uses %d colours, want one series in one colour", len(colours))
	}
	return nil
}

func registerSpendLineSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the plotted monthly totals are ([\d, ]+)$`, w.expectPlottedList)
	sc.Step(`^the chart has exactly 1 data series$`, w.expectOneSeries)
	// Transfers and income would lift the line by thousands; the plotted totals being the expense totals rules them out.
	sc.Step(`^the plotted totals do not include the monthly 500,00 EUR transfer$`,
		func() error { return w.expectPlottedTotals(householdSpend) })
	sc.Step(`^the plotted totals do not include salary$`,
		func() error { return w.expectPlottedTotals(householdSpend) })
}
