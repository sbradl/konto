package dashboard

import (
	"math"
	"strings"

	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	axisRows     = 2   // the x axis and its labels
	labelPad     = 0.6 // x range beyond the last month, leaving room for its label
	headroom     = 1.1 // the y axis reaches this far above the highest month
	minChartSize = 8   // narrower or shorter than this the chart gives way
)

// spendPanel is the monthly spend chart: one braille line, a point per complete month.
func spendPanel(spend []domain.MonthSpend, width, height int) string {
	inner, rows := max(0, width-layout.PanelChrome), max(0, height-layout.PanelChrome)
	if len(spend) == 0 || inner < minChartSize || rows < minChartSize/2 {
		return layout.Panel("SPEND", width, height, "")
	}

	return layout.Panel("SPEND", width, height, spendChart(spend, inner, rows))
}

// spendChart plots the months' totals, in the commodity of the first month that spent anything.
func spendChart(spend []domain.MonthSpend, width, height int) string {
	values, top := spendValues(spend)
	xMax := float64(len(spend)-1) + labelPad
	chart := linechart.New(width, height, 0, xMax, 0, top*headroom, layout.ChartStyle()...)
	labels := monthLabels(spend, chart.GraphWidth(), xMax)
	chart.XLabelFormatter = func(column int, _ float64) string { return labels[column] }
	chart.DrawXYAxisAndLabel()
	for i := 1; i < len(values); i++ {
		chart.DrawBrailleLineWithStyle(
			canvas.Float64Point{X: float64(i - 1), Y: values[i-1]},
			canvas.Float64Point{X: float64(i), Y: values[i]}, theme.Series)
	}

	return strings.TrimRight(chart.View(), "\n")
}

// monthLabels are the months' short names by the graph column each month's point falls in.
func monthLabels(spend []domain.MonthSpend, graphWidth int, xMax float64) map[int]string {
	labels := make(map[int]string, len(spend))
	for i, month := range spend {
		column := int(math.Round(float64(i) * float64(graphWidth-1) / xMax))
		labels[column] = month.Month.Format("Jan")
	}

	return labels
}

// spendValues are the monthly totals as plain numbers, and the highest of them (at least 1).
func spendValues(spend []domain.MonthSpend) (values []float64, top float64) {
	commodity := spendCommodity(spend)
	values, top = make([]float64, len(spend)), 1.0
	for i, month := range spend {
		for _, amount := range month.Total {
			if amount.Commodity == commodity {
				values[i] = layout.Number(amount)
			}
		}
		top = math.Max(top, values[i])
	}

	return values, top
}

// spendCommodity is the commodity of the first month that spent anything; empty when none did.
func spendCommodity(spend []domain.MonthSpend) string {
	for _, month := range spend {
		if len(month.Total) > 0 {
			return month.Total[0].Commodity
		}
	}

	return ""
}
