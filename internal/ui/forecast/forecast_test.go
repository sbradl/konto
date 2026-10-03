package forecast

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	forecastsvc "github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/ui/theme"
)

var (
	escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)
	braille = regexp.MustCompile(`[\x{2801}-\x{28FF}]`)
	style   = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}
)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

func eur(mantissa int64) domain.Amount {
	return domain.Amount{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}
}

// monthEnd is the 30th of the i-th month after March 2026, or the last day of a shorter month.
func monthEnd(i int) time.Time {
	first := time.Date(2026, time.April+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
	return time.Date(first.Year(), first.Month(), min(30, first.AddDate(0, 1, -1).Day()), 0, 0, 0, 0, time.UTC)
}

// course is a line over the 18 months of the chart, rising by step each month from start.
func course(account string, rank int, start, step int64) forecastsvc.Line {
	line := forecastsvc.Line{Account: account, Rank: rank}
	for i := range 18 {
		line.Points = append(line.Points, forecastsvc.Point{End: monthEnd(i), Balance: eur(start + int64(i)*step)})
	}
	line.Milestones = []domain.Amount{eur(start), eur(start + 3*step), eur(start + 6*step), eur(start + 12*step)}
	return line
}

func sample() forecastsvc.Forecast {
	return forecastsvc.Forecast{
		Today: day("2026-09-30"), Begin: day("2026-03-30"), End: day("2027-09-30"),
		Charts: []forecastsvc.Chart{{Commodity: "EUR", Lines: []forecastsvc.Line{
			course("assets:checking", 0, 650000, 150000), course("assets:savings", 1, 1350000, 50000),
			course("liabilities:creditcard", 2, -14000, 0),
		}}},
	}
}

func showing(width, height int) Model {
	return New().Resize(width, height).WithForecast(sample())
}

func TestNothingLoadedYetSaysSo(t *testing.T) {
	if view := plain(New().Resize(100, 30).View()); !strings.Contains(view, "loading the forecast") {
		t.Errorf("view:\n%s", view)
	}
}

func TestNoOpenAccountsSaysThereIsNothingToForecast(t *testing.T) {
	model := New().Resize(100, 30).WithForecast(forecastsvc.Forecast{})

	if view := plain(model.View()); !strings.Contains(view, "nothing to forecast") || model.HiddenPanels() != 0 {
		t.Errorf("view:\n%s", view)
	}
}

func TestOneChartTitledByItsCurrencyDrawnWithBrailleOverAMilestoneTable(t *testing.T) {
	view := plain(showing(180, 60).View())

	for _, want := range []string{"╭─ EUR ", "╭─ MILESTONES "} {
		if strings.Count(view, want) != 1 {
			t.Errorf("view has %q %d times, want once:\n%s", want, strings.Count(view, want), view)
		}
	}
	if !braille.MatchString(view) {
		t.Errorf("the chart is not drawn with braille characters:\n%s", view)
	}
}

func TestTheLegendNamesEveryAccountInItsColour(t *testing.T) {
	raw := showing(180, 60).View()

	for rank, account := range []string{"assets:checking", "assets:savings", "liabilities:creditcard"} {
		if !strings.Contains(raw, theme.Line(rank).Render("● "+account)) {
			t.Errorf("the legend lacks %q in rank %d's colour", account, rank)
		}
	}
}

func TestTheXAxisRunsFromTheMonthSixMonthsBackToTheMonthTwelveAhead(t *testing.T) {
	var labels string
	for row := range strings.SplitSeq(plain(showing(180, 60).View()), "\n") {
		if strings.Contains(row, "Mar") && strings.Contains(row, "Sep") {
			labels = row
			break
		}
	}

	if labels == "" || strings.Index(labels, "Mar") > strings.Index(labels, "Apr") ||
		strings.LastIndex(labels, "Sep") < strings.LastIndex(labels, "Aug") {
		t.Errorf("x labels row = %q, want Mar … Sep", labels)
	}
}

func TestYLabelsCountThousands(t *testing.T) {
	if !regexp.MustCompile(`\d+\.\dk`).MatchString(plain(showing(180, 60).View())) {
		t.Error("no y label is in thousands")
	}
}

func TestTheMilestoneTableShowsEachAccountAcrossTheMonths(t *testing.T) {
	view := plain(showing(180, 60).View())

	for _, want := range []string{
		"today", "+3 months", "+6 months", "+12 months",
		"assets:checking", "6.500,00 EUR", "11.000,00 EUR", "15.500,00 EUR", "24.500,00 EUR",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("table lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(strings.ToLower(view), "total") {
		t.Errorf("a total appears:\n%s", view)
	}
}

func TestNegativeBalancesInTheTableAreRed(t *testing.T) {
	raw := showing(180, 60).View()

	var table string
	for row := range strings.SplitSeq(raw, "\n") {
		if strings.Contains(row, "liabilities:creditcard") && strings.Contains(plain(row), "-140,00 EUR") {
			table = row
		}
	}
	if !strings.Contains(table, theme.Negative.Render("    -140,00 EUR")) {
		t.Errorf("row = %q, want the balances in red", table)
	}
}

func TestSeveralCurrenciesStackAChartEachAndGroupTheTable(t *testing.T) {
	forecast := sample()
	pln := course("assets:wallet", 3, 40000, -2000)
	for i := range pln.Points {
		pln.Points[i].Balance.Commodity = "PLN"
	}
	forecast.Charts = append(forecast.Charts, forecastsvc.Chart{Commodity: "PLN", Lines: []forecastsvc.Line{pln}})

	model := New().Resize(180, 60).WithForecast(forecast)
	view := plain(model.View())

	if strings.Count(view, "╭─ EUR ") != 1 || strings.Count(view, "╭─ PLN ") != 1 || model.HiddenPanels() != 0 {
		t.Errorf("view:\n%s", view)
	}
	if !strings.Contains(view, " EUR\n") && !regexp.MustCompile(`(?m)^│ EUR\s+│$`).MatchString(view) {
		t.Errorf("the table has no EUR group:\n%s", view)
	}
}

func TestChartsGiveWayBeforeTheTableWhenTheTerminalIsSmall(t *testing.T) {
	forecast := sample()
	for _, commodity := range []string{"PLN", "CZK"} {
		forecast.Charts = append(forecast.Charts, forecastsvc.Chart{Commodity: commodity, Lines: forecast.Charts[0].Lines[:1]})
	}
	model := New().Resize(100, 30).WithForecast(forecast)

	if model.HiddenPanels() != 1 {
		t.Errorf("hidden = %d, want the last chart", model.HiddenPanels())
	}
	view := plain(model.View())
	if strings.Count(view, "╭─ EUR ") != 1 || strings.Count(view, "╭─ PLN ") != 1 || strings.Contains(view, "╭─ CZK ") ||
		!strings.Contains(view, "╭─ MILESTONES ") {
		t.Errorf("view:\n%s", view)
	}
}

func TestNothingFitsAboveTheTableInATinyTerminal(t *testing.T) {
	model := New().Resize(100, 8).WithForecast(sample())

	if model.HiddenPanels() != 1 || strings.Contains(plain(model.View()), "╭─ EUR ") {
		t.Errorf("hidden = %d; view:\n%s", model.HiddenPanels(), plain(model.View()))
	}
}

func TestTheViewFillsTheTerminalExactly(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {120, 40}, {180, 60}, {240, 70}} {
		lines := strings.Split(showing(size[0], size[1]).View(), "\n")

		if len(lines) != size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(lines))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > size[0] {
				t.Errorf("%dx%d: a line is %d cells wide: %q", size[0], size[1], lipgloss.Width(line), plain(line))
				break
			}
		}
	}
}

func TestANarrowChartIsLeftOutButItsLegendStays(t *testing.T) {
	view := plain(New().Resize(20, 30).WithForecast(sample()).View())

	if braille.MatchString(view) || !strings.Contains(view, "assets:") {
		t.Errorf("view:\n%s", view)
	}
}

func TestFlatLinesAndEmptyRangesStillDraw(t *testing.T) {
	flat := forecastsvc.Forecast{
		Today: day("2026-09-30"), Begin: day("2026-03-30"), End: day("2027-09-30"),
		Charts: []forecastsvc.Chart{{Commodity: "EUR", Lines: []forecastsvc.Line{course("assets:spare", 0, 0, 0)}}},
	}

	view := plain(New().Resize(120, 40).WithForecast(flat).View())

	if !strings.Contains(view, "╭─ EUR ") || !strings.Contains(view, "0,00 EUR") {
		t.Errorf("view:\n%s", view)
	}
}

func TestNothingIsHiddenBeforeTheForecastIsLoaded(t *testing.T) {
	if New().Resize(100, 30).HiddenPanels() != 0 {
		t.Error("panels are hidden with nothing to show")
	}
}

func TestALongLegendWrapsOntoMoreRowsAndNamesEveryAccount(t *testing.T) {
	forecast := sample()
	lines := forecast.Charts[0].Lines
	for i := range 8 {
		lines = append(lines, course(fmt.Sprintf("assets:extra-%02d", i+1), 3+i, 100000, 0))
	}
	forecast.Charts[0].Lines = lines

	view := plain(New().Resize(100, 60).WithForecast(forecast).View())

	rows := 0
	for row := range strings.SplitSeq(view, "\n") {
		if strings.Contains(row, "●") {
			rows++
		}
	}
	if rows < 2 || strings.Count(view, "●") != len(lines) || !strings.Contains(view, "assets:extra-08") {
		t.Errorf("%d legend rows, %d entries of %d:\n%s", rows, strings.Count(view, "●"), len(lines), view)
	}
}
