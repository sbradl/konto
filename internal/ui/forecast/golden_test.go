package forecast

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	forecastsvc "github.com/sbradl/konto/internal/forecast"
)

// golden compares the plain view with testdata/<name>.txt; KONTO_UPDATE_GOLDEN=1 rewrites the file.
func golden(t *testing.T, name string, model Model) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")
	got := plain(model.View()) + "\n"
	if os.Getenv("KONTO_UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from %s:\n%s\nwant\n%s", name, path, got, want)
	}
}

func twoCurrencies() forecastsvc.Forecast {
	forecast := sample()
	pln := course("assets:wallet", 3, 40000, -2000)
	for i := range pln.Points {
		pln.Points[i].Balance.Commodity = "PLN"
	}
	forecast.Charts = append(forecast.Charts, forecastsvc.Chart{Commodity: "PLN", Lines: []forecastsvc.Line{pln}})
	return forecast
}

func manyLines() forecastsvc.Forecast {
	forecast := sample()
	for i := range 8 {
		forecast.Charts[0].Lines = append(forecast.Charts[0].Lines,
			course(fmt.Sprintf("assets:extra-%02d", i+1), 3+i, 100000, 0))
	}
	return forecast
}

func TestTheViewIsLaidOutAsTheGoldenFilesShow(t *testing.T) {
	smallest := New().Resize(minChartWidth+panelChrome, 0).WithForecast(sample())
	smallest = smallest.Resize(minChartWidth+panelChrome, smallest.tableLines()+panelChrome+minChartPanel)
	tooNarrow := New().Resize(minChartWidth+panelChrome-1, 30).WithForecast(sample())
	oddSplit := New().Resize(100, 41).WithForecast(twoCurrencies())

	flat := forecastsvc.Forecast{
		Today: day("2026-09-30"), Begin: day("2026-03-30"), End: day("2027-09-30"),
		Charts: []forecastsvc.Chart{{Commodity: "EUR", Lines: []forecastsvc.Line{course("assets:spare", 0, 0, 0)}}},
	}
	short := New().Resize(60, 0).WithForecast(manyLines())
	short = short.Resize(60, short.tableLines()+panelChrome+minChartPanel)

	golden(t, "one-chart-100x30", showing(100, 30))
	golden(t, "legend-fits-exactly", New().Resize(39, 40).WithForecast(sample()))
	golden(t, "legend-wraps-one-short", New().Resize(38, 40).WithForecast(sample()))
	golden(t, "legend-entry-wider-than-the-panel", New().Resize(15, 40).WithForecast(sample()))
	golden(t, "flat-line", New().Resize(60, 24).WithForecast(flat))
	golden(t, "many-legend-lines-short", short)
	golden(t, "one-chart-smallest", smallest)
	golden(t, "one-chart-too-narrow", tooNarrow)
	golden(t, "two-currencies-100x41", oddSplit)
	golden(t, "two-currencies-180x60", New().Resize(180, 60).WithForecast(twoCurrencies()))
	golden(t, "many-legend-lines", New().Resize(100, 60).WithForecast(manyLines()))
}

func TestOneRowLessThanTheSmallestChartShowsNoChart(t *testing.T) {
	smallest := New().Resize(60, 0).WithForecast(sample())
	height := smallest.tableLines() + panelChrome + minChartPanel

	if got := smallest.Resize(60, height).HiddenPanels(); got != 0 {
		t.Errorf("hidden at the smallest height = %d", got)
	}
	if got := smallest.Resize(60, height-1).HiddenPanels(); got != 1 {
		t.Errorf("hidden one row below = %d", got)
	}
}
