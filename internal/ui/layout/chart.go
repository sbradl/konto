package layout

import (
	"fmt"
	"math"

	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	thousand   = 1000 // the y axis labels count thousands
	yLabelStep = 2    // rows between y labels
)

// ChartStyle is how every chart looks: dim axes, the y axis labelled in thousands every other row.
func ChartStyle() []linechart.Option {
	return []linechart.Option{
		linechart.WithXYSteps(1, yLabelStep),
		linechart.WithStyles(theme.Dim, theme.Dim, theme.Dim),
		linechart.WithYLabelFormatter(func(_ int, value float64) string {
			return fmt.Sprintf("%.1fk", value/thousand)
		}),
	}
}

// Number is the amount as a plain number of its commodity's units, for a chart.
func Number(amount domain.Amount) float64 {
	return float64(amount.Mantissa) / math.Pow10(amount.Places)
}
