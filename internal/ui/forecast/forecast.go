// Package forecast is the Forecast view: one braille chart per currency over a table of milestone balances.
// It is read-only, with no cursor, and does no I/O; the root builds the forecast and hands it in.
package forecast

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	forecastsvc "github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	axisRows      = 2 // the x axis and its labels
	legendRows    = 1
	minPlotRows   = 4
	minChartPanel = layout.PanelChrome + legendRows + axisRows + minPlotRows
	minChartWidth = 24
	headroom      = 1.1  // the y axis reaches this far beyond the extreme balances
	labelRoom     = 1.06 // the x axis runs this far beyond the last month, so its label fits
	nameGap       = 2
	legendGap     = "   "
	tableTitle    = "MILESTONES"
)

// Model is the Forecast view: what it was last given and the room it has.
type Model struct {
	forecast *forecastsvc.Forecast
	width    int
	height   int
}

// New returns a Forecast view with nothing to show yet.
func New() Model { return Model{} }

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	m.width, m.height = width, height
	return m
}

// WithForecast shows a freshly built forecast.
func (m Model) WithForecast(forecast forecastsvc.Forecast) Model {
	m.forecast = &forecast
	return m
}

// HiddenPanels is how many charts the terminal is too small to show; the milestone table always stays.
func (m Model) HiddenPanels() int {
	if m.forecast == nil {
		return 0
	}
	return len(m.forecast.Charts) - m.shownCharts()
}

// tableLines are the rows of the milestone table: its header, then each currency's heading and accounts.
func (m Model) tableLines() int {
	lines := 1
	for _, chart := range m.forecast.Charts {
		lines += 1 + len(chart.Lines)
	}
	return lines
}

// chartRoom is the height the charts share above the milestone table.
func (m Model) chartRoom() int { return m.height - m.tableLines() - layout.PanelChrome }

// shownCharts is how many charts fit above the milestone table.
func (m Model) shownCharts() int {
	return max(0, min(len(m.forecast.Charts), m.chartRoom()/minChartPanel))
}

// View draws the charts, then the milestone table.
func (m Model) View() string {
	switch {
	case m.forecast == nil:
		return layout.Panel("FORECAST", m.width, m.height, theme.Dim.Render(" loading the forecast…"))
	case len(m.forecast.Charts) == 0:
		return layout.Panel("FORECAST", m.width, m.height,
			theme.Dim.Render(" nothing to forecast — no open asset or liability account"))
	}
	shown, room := m.shownCharts(), m.chartRoom()
	tableHeight := m.height - room
	panels := make([]string, 0, shown+1)
	for i := range shown {
		height := room / shown
		if i < room%shown {
			height++
		}
		chart := m.forecast.Charts[i]
		body := m.chartBody(chart, m.width-layout.PanelChrome, height-layout.PanelChrome)
		panels = append(panels, layout.Panel(chart.Commodity, m.width, height, body))
	}
	table := layout.Panel(tableTitle, m.width, min(tableHeight, m.height), m.table())
	return strings.Join(append(panels, table), "\n")
}

// chartBody is the plot with its legend below.
func (m Model) chartBody(chart forecastsvc.Chart, width, height int) string {
	legend := legendLines(chart, width)
	plot := ""
	if width >= minChartWidth {
		plot = m.plot(chart, width, height-len(legend))
	}
	return plot + "\n" + strings.Join(legend, "\n")
}

// legendLines name each line in its colour, on as many rows as the width needs.
func legendLines(chart forecastsvc.Chart, width int) []string {
	rows := make([]string, 0, 1)
	row := make([]string, 0, len(chart.Lines))
	used := 0
	flush := func() {
		rows = append(rows, " "+strings.Join(row, legendGap))
		row, used = row[:0], 0
	}
	for _, line := range chart.Lines {
		text := "● " + line.Account
		if len(row) > 0 && used+len(legendGap)+lipgloss.Width(text) > width-1 {
			flush()
		}
		if len(row) > 0 {
			used += len(legendGap)
		}
		row, used = append(row, theme.Line(line.Rank).Render(text)), used+lipgloss.Width(text)
	}
	flush()
	return rows
}

// plot draws every account's line over the six months behind and the twelve ahead, with a dim line at today.
func (m Model) plot(chart forecastsvc.Chart, width, height int) string {
	low, high := bounds(chart)
	begin, end := m.forecast.Begin, m.forecast.End
	span := elapsed(begin, end)
	canvasHeight := max(axisRows+1, height)
	plot := linechart.New(width, canvasHeight, 0, span*labelRoom, low, high, layout.ChartStyle()...)
	labels := monthLabels(plot, begin, chart.Lines[0].Points, span)
	plot.XLabelFormatter = func(i int, _ float64) string { return labels[i] }
	plot.DrawXYAxisAndLabel()
	today := elapsed(begin, m.forecast.Today)
	marker := [2]canvas.Float64Point{{X: today, Y: low}, {X: today, Y: high}}
	plot.DrawBrailleLineWithStyle(marker[0], marker[1], theme.Dim)
	for _, line := range chart.Lines {
		drawLine(plot, line, begin)
	}
	return strings.TrimRight(plot.View(), "\n")
}

// elapsed is the x coordinate of a date: the hours since the chart begins.
func elapsed(from, to time.Time) float64 { return to.Sub(from).Hours() }

// bounds are the y range of the chart: from the lowest balance, or zero, to a little above the highest.
func bounds(chart forecastsvc.Chart) (low, high float64) {
	for _, line := range chart.Lines {
		for _, point := range line.Points {
			low, high = math.Min(low, layout.Number(point.Balance)), math.Max(high, layout.Number(point.Balance))
		}
	}
	low *= headroom // never above zero
	return low, math.Max(high*headroom, low+1)
}

// monthLabels name the months along the x axis: the month the chart begins in, then each month by the
// column its balance is plotted at.
func monthLabels(plot linechart.Model, begin time.Time, points []forecastsvc.Point, span float64) map[int]string {
	column := func(day float64) int {
		return int(math.Round(day * float64(plot.GraphWidth()-1) / (span * labelRoom)))
	}
	labels := make(map[int]string, len(points))
	labels[0] = begin.Format("Jan")
	for _, point := range points {
		labels[column(elapsed(begin, point.End))] = point.End.AddDate(0, 0, -1).Format("Jan")
	}
	return labels
}

// drawLine joins the balances of one account month by month, in the colour of its rank.
func drawLine(plot linechart.Model, line forecastsvc.Line, begin time.Time) {
	style := theme.Line(line.Rank)
	for i := 1; i < len(line.Points); i++ {
		from, to := line.Points[i-1], line.Points[i]
		plot.DrawBrailleLineWithStyle(
			canvas.Float64Point{X: elapsed(begin, from.End), Y: layout.Number(from.Balance)},
			canvas.Float64Point{X: elapsed(begin, to.End), Y: layout.Number(to.Balance)}, style)
	}
}

// table is the milestone balances: a heading per currency, a row per account, negative balances red.
func (m Model) table() string {
	nameWidth, cell := m.tableWidths()
	var header strings.Builder
	header.WriteString("  " + strings.Repeat(" ", nameWidth))
	for _, months := range forecastsvc.Milestones() {
		header.WriteString(layout.AlignRight(milestoneTitle(months), cell))
	}
	rows := []string{theme.Dim.Render(header.String())}
	for _, chart := range m.forecast.Charts {
		rows = append(rows, " "+theme.Bold.Render(chart.Commodity))
		for _, line := range chart.Lines {
			rows = append(rows, tableRow(line, nameWidth, cell))
		}
	}
	return strings.Join(rows, "\n")
}

// tableWidths are the width of the account column and of each balance column.
func (m Model) tableWidths() (name, cell int) {
	cell = len(milestoneTitle(forecastsvc.Milestones()[len(forecastsvc.Milestones())-1]))
	for _, chart := range m.forecast.Charts {
		for _, line := range chart.Lines {
			name = max(name, len(line.Account))
			for _, milestone := range line.Milestones {
				cell = max(cell, len(milestone.Format()))
			}
		}
	}
	return name, cell + nameGap
}

// tableRow is an account and its balances at the milestones.
func tableRow(line forecastsvc.Line, nameWidth, cell int) string {
	var row strings.Builder
	fmt.Fprintf(&row, "  %-*s", nameWidth, line.Account)
	for _, milestone := range line.Milestones {
		text := layout.AlignRight(milestone.Format(), cell)
		if milestone.IsNegative() {
			text = theme.Negative.Render(text)
		}
		row.WriteString(text)
	}
	return row.String()
}

// milestoneTitle names a column of the table: today, or so many months from it.
func milestoneTitle(months int) string {
	if months == 0 {
		return "today"
	}
	return fmt.Sprintf("+%d months", months)
}
