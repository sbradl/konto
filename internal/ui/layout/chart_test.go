package layout

import (
	"strings"
	"testing"

	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

func TestNumberIsTheAmountInUnitsOfItsCommodity(t *testing.T) {
	if got := Number(domain.Amount{Mantissa: -1234, Places: 2}); got != -12.34 {
		t.Errorf("Number = %v, want -12.34", got)
	}
}

func TestChartLabelsTheYAxisInThousandsEveryOtherRow(t *testing.T) {
	chart := linechart.New(30, 8, 0, 1, 0, 4000, ChartStyle()...)
	chart.DrawXYAxisAndLabel()

	view := uitest.Plain(chart.View())
	if !strings.Contains(view, "4.0k") || !strings.Contains(view, "0.0k") || strings.Count(view, "k") != 4 {
		t.Errorf("chart:\n%s", view)
	}
}
