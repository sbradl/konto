//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// screenPanel is one bordered panel of the screen: its title and the rows between its borders, as runes so
// that the index of a rune is its column.
type screenPanel struct {
	title       string
	top, bottom int // the screen rows of its borders
	right       int // the column of its right border
	rows        [][]rune
}

var panelTop = regexp.MustCompile(`^╭─ (\S.*?) ─*╮$`)

// panelsOnScreen finds every panel whose top-left corner starts a screen row.
func (w *world) panelsOnScreen() ([]screenPanel, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	lines := s.screenLines()
	var panels []screenPanel
	for y, line := range lines {
		match := panelTop.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || !strings.HasPrefix(line, "╭") {
			continue
		}
		panel := screenPanel{title: strings.TrimSpace(match[1]), top: y, right: len([]rune(line)) - 1}
		for yy := y + 1; yy < len(lines) && !strings.HasPrefix(lines[yy], "╰"); yy++ {
			panel.rows = append(panel.rows, []rune(lines[yy]))
			panel.bottom = yy + 1
		}
		panels = append(panels, panel)
	}
	return panels, nil
}

// onForecast waits for the panels of the screen to satisfy check.
func (w *world) onForecast(check func(panels []screenPanel) error) error {
	return w.eventually(func() error {
		panels, err := w.panelsOnScreen()
		if err != nil {
			return err
		}
		return check(panels)
	})
}

// onChart waits for the chart titled title to satisfy check.
func (w *world) onChart(title string, check func(chart screenPanel) error) error {
	return w.onForecast(func(panels []screenPanel) error {
		chart, err := chartTitled(panels, title)
		if err != nil {
			return err
		}
		return check(chart)
	})
}

func isChart(panel screenPanel) bool { return panel.title != "MILESTONES" && panel.title != "FORECAST" }

func chartTitled(panels []screenPanel, title string) (screenPanel, error) {
	for _, panel := range panels {
		if isChart(panel) && panel.title == title {
			return panel, nil
		}
	}
	return screenPanel{}, fmt.Errorf("no chart titled %q", title)
}

func (p screenPanel) text() string {
	lines := make([]string, len(p.rows))
	for i, row := range p.rows {
		lines[i] = string(row)
	}
	return strings.Join(lines, "\n")
}

// legendEntry is an account of a chart's legend and where its marker is drawn.
type legendEntry struct {
	account string
	x, y    int // the screen cell of the marker
}

var legendText = regexp.MustCompile(`● (\S+)`)

func (p screenPanel) legend() []legendEntry {
	var entries []legendEntry
	for i, row := range p.rows {
		line := string(row)
		for _, at := range legendText.FindAllStringSubmatchIndex(line, -1) {
			entries = append(entries, legendEntry{
				account: line[at[2]:at[3]], x: len([]rune(line[:at[0]])), y: p.top + 1 + i,
			})
		}
	}
	return entries
}

func (p screenPanel) accounts() []string {
	entries := p.legend()
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.account
	}
	return names
}

// colourOf is the colour the account's line is drawn in: the colour of its legend marker.
func (w *world) colourOf(chart screenPanel, account string) (int, error) {
	for _, entry := range chart.legend() {
		if entry.account == account {
			return w.s.cellColour(entry.x, entry.y), nil
		}
	}
	return 0, fmt.Errorf("no legend entry %q", account)
}

func (w *world) expectChartTitles(count int, titles string) error {
	return w.onForecast(func(panels []screenPanel) error {
		var charts []string
		for _, panel := range panels {
			if isChart(panel) {
				charts = append(charts, panel.title)
			}
		}
		if len(charts) != count {
			return fmt.Errorf("the charts are %v, want %d", charts, count)
		}
		for _, title := range quotedTexts(titles) {
			if !slices.Contains(charts, title) {
				return fmt.Errorf("the charts are %v, want one titled %q", charts, title)
			}
		}
		return nil
	})
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

func (w *world) expectChartLines(title string, want []string, exact bool) error {
	return w.onChart(title, func(chart screenPanel) error {
		have := chart.accounts()
		for _, account := range want {
			if !slices.Contains(have, account) {
				return fmt.Errorf("the %s chart has lines %v, lacks %s", title, have, account)
			}
		}
		if exact && len(have) != len(want) {
			return fmt.Errorf("the %s chart has %d lines %v, want %d", title, len(have), have, len(want))
		}
		return nil
	})
}

func (w *world) expectChartLacksLine(title, account string) error {
	return w.onChart(title, func(chart screenPanel) error {
		if slices.Contains(chart.accounts(), account) {
			return fmt.Errorf("the %s chart has a line for %s", title, account)
		}
		return nil
	})
}

func (w *world) expectLegendColour(account string, colour int) error {
	return w.onChart("EUR", func(chart screenPanel) error {
		got, err := w.colourOf(chart, account)
		if err == nil && got != colour {
			err = fmt.Errorf("the legend entry %q is coloured %d, want %d", account, got, colour)
		}
		return err
	})
}

func (w *world) expectLegendColourOrder(order []int) error {
	return w.onChart("EUR", func(chart screenPanel) error {
		entries := chart.legend()
		if len(entries) < len(order) {
			return fmt.Errorf("the legend has %d entries, want at least %d", len(entries), len(order))
		}
		colours := make([]int, len(order))
		for i := range order {
			colours[i] = w.s.cellColour(entries[i].x, entries[i].y)
		}
		if !slices.Equal(colours, order) {
			return fmt.Errorf("the legend colours are %v, want %v", colours, order)
		}
		return nil
	})
}

// plotArea is where a chart is drawn: the screen cell of its axis corner and the rows above it.
type plotArea struct {
	panel             screenPanel
	axisRow, axisCol  int // the screen row and column of the "└" corner
	firstRow, lastRow int // the screen rows of the plot, top and bottom
	firstCol, lastCol int
	labelRow          int
	yLabels           map[int]float64 // screen row → the value its y label gives
}

var yLabel = regexp.MustCompile(`^\s*(-?\d+\.\d)k`)

func plotOf(panel screenPanel) (plotArea, error) {
	area := plotArea{panel: panel, yLabels: map[int]float64{}, lastCol: panel.right - 1, firstRow: -1}
	for i, row := range panel.rows {
		y, line := panel.top+1+i, string(row)
		if corner, _, found := strings.Cut(line, "└"); found {
			area.axisRow, area.axisCol, area.lastRow, area.labelRow = y, len([]rune(corner)), y-1, y+1
			break
		}
		if value, ok := labelValue(line); ok {
			area.yLabels[y] = value
		}
		if area.firstRow < 0 {
			area.firstRow = y
		}
	}
	if area.axisRow == 0 || len(area.yLabels) < 2 {
		return plotArea{}, errors.New("the chart has no plot area:\n" + panel.text())
	}
	area.firstCol = area.axisCol + 1
	return area, nil
}

// labelValue is the balance a row's y label gives, if it has one.
func labelValue(line string) (float64, bool) {
	match := yLabel.FindStringSubmatch(strings.TrimPrefix(line, "│"))
	if match == nil {
		return 0, false
	}
	value, _ := strconv.ParseFloat(match[1], 64)
	return value * 1000, true
}

// perDot is the balance one dot row of the plot stands for.
func (a plotArea) perDot() float64 { return a.valueAt(a.firstRow, 0) - a.valueAt(a.firstRow, 1) }

// valueAt turns a screen row and a dot row within it into the balance it stands for.
func (a plotArea) valueAt(row, dot int) float64 {
	rows := make([]int, 0, len(a.yLabels))
	for y := range a.yLabels {
		rows = append(rows, y)
	}
	top, bottom := slices.Min(rows), slices.Max(rows)
	perDot := (a.yLabels[top] - a.yLabels[bottom]) / float64((bottom-top)*4)
	return a.yLabels[top] - float64((row-top)*4+dot-2)*perDot
}

// brailleDots are the dot rows, from the top, that a braille rune sets in its left and right column.
func brailleDots(r rune) (left, right []int) {
	bits := int(r - 0x2800)
	leftBits, rightBits := [4]int{0, 1, 2, 6}, [4]int{3, 4, 5, 7}
	for dot := range 4 {
		if bits&(1<<leftBits[dot]) != 0 {
			left = append(left, dot)
		}
		if bits&(1<<rightBits[dot]) != 0 {
			right = append(right, dot)
		}
	}
	return left, right
}

// lineCells are the plot cells drawn in the colour: their column and the balances of their dots.
func (w *world) lineCells(area plotArea, colour int) map[int][]float64 {
	cells := map[int][]float64{}
	for y := area.firstRow; y <= area.lastRow; y++ {
		row := area.panel.rows[y-area.panel.top-1]
		for x := area.firstCol; x < len(row) && x <= area.lastCol; x++ {
			if !isBraille(row[x]) || w.s.cellColour(x, y) != colour {
				continue
			}
			left, right := brailleDots(row[x])
			for _, dot := range slices.Concat(left, right) {
				cells[x] = append(cells[x], area.valueAt(y, dot))
			}
		}
	}
	return cells
}

func average(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// todayColumn is the column the dim marker line of today is drawn at: the column with the most dim braille.
func (w *world) todayColumn(area plotArea) (int, error) {
	best, most := -1, 0
	for x, values := range w.lineCells(area, 8) {
		if len(values) > most {
			best, most = x, len(values)
		}
	}
	if best < 0 {
		return 0, errors.New("no dim marker line in the chart:\n" + area.panel.text())
	}
	return best, nil
}

func (w *world) expectTodayLine(date string) error {
	return w.onChart("EUR", func(chart screenPanel) error {
		area, err := plotOf(chart)
		if err != nil {
			return err
		}
		column, err := w.todayColumn(area)
		if err != nil {
			return err
		}
		today, _ := time.Parse(time.DateOnly, date)
		begin, end := today.AddDate(0, -6, 0), today.AddDate(1, 0, 0)
		span := end.Sub(begin).Hours() / 24 * 1.06
		width := float64(area.lastCol - area.firstCol)
		want := area.firstCol + int(math.Round(today.Sub(begin).Hours()/24/span*width))
		if column < want-2 || column > want+2 {
			return fmt.Errorf("the dim line is at column %d, want about %d for %s", column, want, date)
		}
		return nil
	})
}

// accountLine is the cells an account's line is drawn in, with the plot they are in.
func (w *world) accountLine(chart screenPanel, account string) (plotArea, map[int][]float64, error) {
	area, err := plotOf(chart)
	if err != nil {
		return plotArea{}, nil, err
	}
	colour, err := w.colourOf(chart, account)
	if err != nil {
		return plotArea{}, nil, err
	}
	cells := w.lineCells(area, colour)
	if len(cells) == 0 {
		return plotArea{}, nil, fmt.Errorf("no line for %s", account)
	}
	return area, cells, nil
}

func (w *world) expectLineValue(account, month, want string) error {
	value, err := parseEuros(want)
	if err != nil {
		return err
	}
	return w.onChart("EUR", func(chart screenPanel) error {
		area, cells, err := w.accountLine(chart, account)
		if err != nil {
			return err
		}
		got := valueAtColumn(cells, w.lineColumn(area, cells, month))
		if math.Abs(got-value) > 1500 {
			return fmt.Errorf("the %s line at %s is about %.0f, want %.0f", account, month, got, value)
		}
		return nil
	})
}

// lineColumn is the column to read the line at for a month: the first column for the earliest history, the
// dim marker's column for today's month.
func (w *world) lineColumn(area plotArea, cells map[int][]float64, month string) int {
	if month == "2026-09" {
		if today, err := w.todayColumn(area); err == nil {
			return today
		}
	}
	columns := make([]int, 0, len(cells))
	for x := range cells {
		columns = append(columns, x)
	}
	return slices.Min(columns)
}

// valueAtColumn is the line's balance at a column; lines that cross share a cell and its colour, so a column
// without a cell of the line is read between the nearest cells on both sides of it.
func valueAtColumn(cells map[int][]float64, column int) float64 {
	if values, ok := cells[column]; ok {
		return average(values)
	}
	before, after := -1, -1
	for x := range cells {
		if x < column && x > before {
			before = x
		}
		if x > column && (after < 0 || x < after) {
			after = x
		}
	}
	switch {
	case before < 0:
		return average(cells[after])
	case after < 0:
		return average(cells[before])
	}
	share := float64(column-before) / float64(after-before)
	return average(cells[before]) + share*(average(cells[after])-average(cells[before]))
}

func parseEuros(text string) (float64, error) {
	number := strings.TrimSuffix(strings.TrimSpace(text), " EUR")
	number = strings.ReplaceAll(strings.ReplaceAll(number, ".", ""), ",", ".")
	return strconv.ParseFloat(number, 64)
}

func (w *world) expectLineFlat(account string) error {
	return w.onChart("EUR", func(chart screenPanel) error {
		area, cells, err := w.accountLine(chart, account)
		if err != nil {
			return err
		}
		var all []float64
		for _, values := range cells {
			all = append(all, values...)
		}
		// lines that share a cell share its colour, and the dim marker line of today runs through one cell
		slices.Sort(all)
		median := all[len(all)/2]
		far := 0
		for _, value := range all {
			if math.Abs(value-median) > area.perDot()*1.5 {
				far++
			}
		}
		if far*20 > len(all) {
			return fmt.Errorf("the %s line is not flat: %d of %d dots are far from %.0f", account, far, len(all), median)
		}
		return nil
	})
}

// monthName is the three-letter name of the month a date falls in.
func monthName(date time.Time) string { return date.Format("Jan") }

func (w *world) expectXRange() error {
	today, _ := time.Parse(time.DateOnly, w.env["KONTO_TODAY"])
	first, last := monthName(today.AddDate(0, -6, 0)), monthName(today.AddDate(1, 0, 0))
	return w.onChart("EUR", func(chart screenPanel) error {
		area, err := plotOf(chart)
		if err != nil {
			return err
		}
		labels := strings.Fields(strings.Trim(string(chart.rows[area.labelRow-chart.top-1]), "│"))
		if len(labels) < 2 || labels[0] != first || labels[len(labels)-1] != last {
			return fmt.Errorf("the x labels are %v, want %s … %s", labels, first, last)
		}
		return nil
	})
}

func (w *world) expectYLabelsInThousands() error {
	return w.onChart("EUR", func(chart screenPanel) error {
		area, err := plotOf(chart)
		if err == nil && len(area.yLabels) < 2 {
			err = errors.New("no y labels in thousands")
		}
		return err
	})
}

var milestoneFigure = regexp.MustCompile(`-?[\d.]+,\d\d [A-Z]{3}`)

// milestoneRow is the row of the milestone table that names the account, with the screen row it is on.
func milestoneRow(panels []screenPanel, account string) (text string, y int, err error) {
	for _, panel := range panels {
		if panel.title != "MILESTONES" {
			continue
		}
		for i, row := range panel.rows {
			if strings.Contains(string(row), " "+account+" ") {
				return string(row), panel.top + 1 + i, nil
			}
		}
	}
	return "", 0, fmt.Errorf("no milestone row for %s", account)
}

func (w *world) onMilestoneRow(account string, check func(text string, y int) error) error {
	return w.onForecast(func(panels []screenPanel) error {
		text, y, err := milestoneRow(panels, account)
		if err != nil {
			return err
		}
		return check(text, y)
	})
}

func (w *world) expectMilestones(account string, figures []string) error {
	return w.onMilestoneRow(account, func(text string, _ int) error {
		have := milestoneFigure.FindAllString(text, -1)
		if len(have) < len(figures) || !slices.Equal(have[:len(figures)], figures) {
			return fmt.Errorf("the milestone row of %s shows %v, want %v", account, have, figures)
		}
		return nil
	})
}

func (w *world) expectMilestoneToday(account, want string, same bool) error {
	return w.onMilestoneRow(account, func(text string, _ int) error {
		if today := milestoneFigure.FindString(text); (today == want) != same {
			return fmt.Errorf("the today figure of %s is %s, want it equal to %s: %v", account, today, want, same)
		}
		return nil
	})
}

func (w *world) expectMilestoneColour(figure, account string, colour int) error {
	return w.onMilestoneRow(account, func(text string, y int) error {
		at := strings.LastIndex(text, figure)
		if at < 0 {
			return fmt.Errorf("the row %q lacks %q", text, figure)
		}
		if got := w.s.cellColour(len([]rune(text[:at]))+1, y); got != colour {
			return fmt.Errorf("%q is coloured %d, want %d", figure, got, colour)
		}
		return nil
	})
}

// onMilestones waits for the milestone table to satisfy check.
func (w *world) onMilestones(check func(table screenPanel) error) error {
	return w.onForecast(func(panels []screenPanel) error {
		for _, panel := range panels {
			if panel.title == "MILESTONES" {
				return check(panel)
			}
		}
		return errors.New("no milestone table")
	})
}

func (w *world) expectMilestoneText(want bool, text string) error {
	return w.onMilestones(func(table screenPanel) error {
		if strings.Contains(table.text(), text) != want {
			return fmt.Errorf("the milestone table contains %q is %v, want %v", text, !want, want)
		}
		return nil
	})
}

func (w *world) expectMilestoneHeader(columns []string) error {
	return w.onMilestones(func(table screenPanel) error {
		header, at := string(table.rows[0]), 0
		for _, column := range columns {
			found := strings.Index(header[at:], column)
			if found < 0 {
				return fmt.Errorf("the table header %q lacks %q in order", header, column)
			}
			at += found + len(column)
		}
		return nil
	})
}

func (w *world) expectGroups(groups []string) error {
	return w.onMilestones(func(table screenPanel) error {
		var have []string
		for _, row := range table.rows {
			if text := strings.TrimSpace(strings.Trim(string(row), "│ ")); slices.Contains(groups, text) {
				have = append(have, text)
			}
		}
		if len(have) != len(groups) {
			return fmt.Errorf("the table groups are %v, want %v", have, groups)
		}
		return nil
	})
}

func (w *world) expectStacked() error {
	return w.onForecast(func(panels []screenPanel) error {
		last := -1
		for _, panel := range panels {
			if !isChart(panel) {
				continue
			}
			if panel.top <= last {
				return errors.New("the charts are not stacked vertically")
			}
			last = panel.top
		}
		return nil
	})
}

func (w *world) expectFullyVisible() error {
	return w.onForecast(func(panels []screenPanel) error {
		for _, panel := range panels {
			if !isChart(panel) {
				continue
			}
			area, err := plotOf(panel)
			if err != nil || area.lastRow-area.firstRow < 1 || len(panel.legend()) == 0 {
				return fmt.Errorf("the %s chart is cut off (%w):\n%s", panel.title, err, panel.text())
			}
		}
		return nil
	})
}

func (w *world) appendRule(rule string) error {
	file, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("\n" + rule + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (w *world) useManualAccount(name, account, amount, date, rule string) error {
	text := fmt.Sprintf("\naccount %s  ; konto:source:manual\n\n%s opening balance\n    %s  %s\n    equity:opening-balances\n\n%s",
		account, date, account, amount, unescape(rule))
	return w.useJournal(name, text)
}

var forecastDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

func (w *world) expectNoRecentAfter(date string) error {
	return w.expectRecent(func(rows []recentRow) error {
		for _, row := range rows {
			if found := forecastDate.FindString(row.text); found > date {
				return fmt.Errorf("a Recent row is dated %s: %q", found, row.text)
			}
		}
		return nil
	})
}

// expectViewLacks waits for the view, without the tab bar and the status bar, to hold none of the texts.
func (w *world) expectViewLacks(texts ...string) error {
	return w.eventually(func() error {
		lines := w.s.screenLines()
		view := strings.ToLower(strings.Join(lines[1:len(lines)-1], "\n"))
		for _, text := range texts {
			if strings.Contains(view, strings.ToLower(text)) {
				return fmt.Errorf("the view shows %q:\n%s", text, view)
			}
		}
		return nil
	})
}

func registerForecastSteps(sc *godog.ScenarioContext, w *world) {
	registerForecastChartSteps(sc, w)
	registerForecastLineSteps(sc, w)
	registerForecastTableSteps(sc, w)
	registerForecastStateSteps(sc, w)
}

func registerForecastChartSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see exactly (\d+) charts? titled ((?:"[^"]*"\s*)+)$`, w.expectChartTitles)
	step(sc, `^the chart is drawn with braille characters$`, func() error {
		return w.eventually(func() error {
			if strings.IndexFunc(w.s.screenText(), isBraille) < 0 {
				return errors.New("no braille on screen")
			}
			return nil
		})
	})
	step(sc, `^the EUR chart has lines for$`, func(table *godog.Table) error {
		accounts := make([]string, len(table.Rows))
		for i, row := range table.Rows {
			accounts[i] = strings.TrimSpace(row.Cells[0].Value)
		}
		return w.expectChartLines("EUR", accounts, false)
	})
	step(sc, `^the EUR chart has exactly (\d+) lines$`, func(n int) error {
		return w.onChart("EUR", func(chart screenPanel) error {
			if got := len(chart.legend()); got != n {
				return fmt.Errorf("the EUR chart has %d lines, want %d", got, n)
			}
			return nil
		})
	})
	step(sc, `^the "([^"]*)" chart has a line for "([^"]*)"$`, func(title, account string) error {
		return w.expectChartLines(title, []string{account}, false)
	})
	step(sc, `^the "([^"]*)" chart has no line for "([^"]*)"$`, w.expectChartLacksLine)
	step(sc, `^the charts are stacked vertically$`, w.expectStacked)
	step(sc, `^every chart is fully visible$`, w.expectFullyVisible)
}

func registerForecastLineSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the legend entry "([^"]*)" is coloured (\d+)$`, w.expectLegendColour)
	step(sc, `^the legend colours follow the order ([\d, ]+)$`, func(order string) error {
		fields := strings.Split(order, ",")
		colours := make([]int, len(fields))
		for i, field := range fields {
			colours[i], _ = strconv.Atoi(strings.TrimSpace(field))
		}
		return w.expectLegendColourOrder(colours)
	})
	step(sc, `^I see a legend below the "([^"]*)" chart listing every line's account name$`, func(title string) error {
		return w.expectChartLines(title,
			[]string{"assets:checking", "assets:savings", "assets:spare", "liabilities:creditcard"}, true)
	})
	step(sc, `^the EUR chart's x-range starts six months before today$`, w.expectXRange)
	step(sc, `^it ends twelve months after today$`, func() error { return nil }) // the same labels row says both
	step(sc, `^the EUR chart has a vertical line coloured 8 at the column for ([\d-]+)$`, w.expectTodayLine)
	step(sc, `^the forecast y-axis labels are formatted in thousands$`, w.expectYLabelsInThousands)
	step(sc, `^the checking line at ([\d-]+) is at ([\d.,]+ EUR)$`, func(month, value string) error {
		return w.expectLineValue("assets:checking", month, value)
	})
	step(sc, `^the "([^"]*)" line is flat$`, w.expectLineFlat)
}

func registerForecastTableSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the milestone table header is ((?:"[^"]*"\s*)+)$`, func(list string) error {
		return w.expectMilestoneHeader(quotedTexts(list))
	})
	step(sc, `^the milestone table has the groups ((?:"[^"]*"\s*)+)$`, func(list string) error {
		return w.expectGroups(quotedTexts(list))
	})
	step(sc, `^the milestone row "([^"]*)" (?:still )?shows ((?:"[^"]*"\s*)+)$`, func(account, list string) error {
		return w.expectMilestones(account, quotedTexts(list))
	})
	step(sc, `^"([^"]*)" in the milestone row "([^"]*)" is coloured (\d+)$`, w.expectMilestoneColour)
	step(sc, `^the milestone table has no "([^"]*)" row$`, func(text string) error { return w.expectMilestoneText(false, text) })
	step(sc, `^the milestone table has no row "([^"]*)"$`, func(account string) error {
		return w.expectMilestoneText(false, account)
	})
	step(sc, `^the "today" figure of "([^"]*)" differs from its journal balance of ([\d.,]+ EUR)$`, func(account, balance string) error {
		return w.expectMilestoneToday(account, balance, false)
	})
	step(sc, `^the "today" figure of "([^"]*)" is "([^"]*)"$`, func(account, figure string) error {
		return w.expectMilestoneToday(account, figure, true)
	})
}

func registerForecastStateSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the charts show no total line$`, func() error {
		return w.onForecast(func(panels []screenPanel) error {
			for _, panel := range panels {
				if isChart(panel) && strings.Contains(strings.ToLower(strings.Join(panel.accounts(), " ")), "total") {
					return errors.New("a chart has a total line")
				}
			}
			return nil
		})
	})
	step(sc, `^I do not see "net worth" on the forecast view$`, func() error { return w.expectViewLacks("net worth") })
	step(sc, `^I do not see "([^"]*)" or "([^"]*)" or "([^"]*)" or "([^"]*)" or "([^"]*)"$`, func(a, b, c, d, e string) error {
		return w.expectViewLacks(a, b, c, d, e)
	})
	step(sc, `^no value on the screen is a converted amount$`, func() error { return w.expectViewLacks("≈", "converted", "rate") })
	step(sc, `^I append a rule externally:$`, func(doc *godog.DocString) error { return w.appendRule(doc.Content) })
	step(sc, `^the journal "([^"]*)" with a manual account "([^"]*)" holding ([\d,]+ EUR) from ([\d-]+) and the rule "((?s:.*))"$`,
		w.useManualAccount)
	step(sc, `^no Recent row is dated after "([^"]*)"$`, w.expectNoRecentAfter)
	step(sc, `^I see a message that there is nothing to forecast$`, func() error {
		return w.expectScreen(true, "nothing to forecast")
	})
}
