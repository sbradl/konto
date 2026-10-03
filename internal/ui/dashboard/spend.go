package dashboard

import (
	"fmt"
	"math"
	"strings"

	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	axisRows     = 2    // the x axis and its labels
	labelPad     = 0.6  // x range beyond the last month, leaving room for its label
	headroom     = 1.1  // the y axis reaches this far above the highest month
	thousand     = 1000 // the y axis labels count thousands
	yLabelStep   = 2    // rows between y labels
	minChartSize = 8    // narrower or shorter than this the chart gives way
)

// spendPanel is the monthly spend chart: one braille line, a point per complete month.
func spendPanel(spend []domain.MonthSpend, width, height int) string {
	inner, rows := max(0, width-panelChrome), max(0, height-panelChrome)
	if len(spend) == 0 || inner < minChartSize || rows < minChartSize/2 {
		return layout.Panel("SPEND", width, height, "")
	}
	return layout.Panel("SPEND", width, height, spendChart(spend, inner, rows))
}

// spendChart plots the months' totals, in the commodity of the first month that spent anything.
func spendChart(spend []domain.MonthSpend, width, height int) string {
	values, top := spendValues(spend)
	last := float64(len(spend) - 1)
	chart := linechart.New(width, height, 0, last+labelPad, 0, top*headroom,
		linechart.WithXYSteps(1, yLabelStep),
		linechart.WithStyles(theme.Dim, theme.Dim, theme.Dim),
		linechart.WithYLabelFormatter(func(_ int, v float64) string { return fmt.Sprintf("%.1fk", v/thousand) }),
	)
	column := func(i int) int {
		return int(math.Round(float64(i) * float64(chart.GraphWidth()-1) / (last + labelPad)))
	}
	labels := make(map[int]string, len(spend))
	for i, month := range spend {
		labels[column(i)] = month.Month.Format("Jan")
	}
	chart.XLabelFormatter = func(i int, _ float64) string { return labels[i] }
	chart.DrawXYAxisAndLabel()
	for i := 1; i < len(values); i++ {
		chart.DrawBrailleLineWithStyle(
			canvas.Float64Point{X: float64(i - 1), Y: values[i-1]},
			canvas.Float64Point{X: float64(i), Y: values[i]}, theme.Series)
	}
	return strings.TrimRight(chart.View(), "\n")
}

// spendValues are the monthly totals as plain numbers, and the highest of them (at least 1).
func spendValues(spend []domain.MonthSpend) (values []float64, top float64) {
	commodity := ""
	for _, month := range spend {
		if len(month.Total) > 0 {
			commodity = month.Total[0].Commodity
			break
		}
	}
	values, top = make([]float64, len(spend)), 1.0
	for i, month := range spend {
		for _, amount := range month.Total {
			if amount.Commodity == commodity {
				values[i] = float64(amount.Mantissa) / math.Pow10(amount.Places)
			}
		}
		top = math.Max(top, values[i])
	}
	return values, top
}
