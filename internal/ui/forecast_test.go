package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	forecastsvc "github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/ui/uitest"
)

type fakeForecaster struct {
	ledgers  []domain.Ledger
	forecast forecastsvc.Forecast
	err      error
}

func (f *fakeForecaster) Forecast(_ context.Context, snapshot domain.Ledger) (forecastsvc.Forecast, error) {
	f.ledgers = append(f.ledgers, snapshot)

	return f.forecast, f.err
}

func oneChart(commodities ...string) forecastsvc.Forecast {
	style := domain.DefaultAmount("EUR").Style
	forecast := forecastsvc.Forecast{
		Today: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), Begin: time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		End: time.Date(2027, 9, 30, 0, 0, 0, 0, time.UTC),
	}
	for _, commodity := range commodities {
		amount := domain.Amount{Commodity: commodity, Mantissa: 1324000, Places: 2, Style: style}
		line := forecastsvc.Line{
			Account: "assets:checking", Milestones: []domain.Amount{amount, amount, amount, amount},
			Points: []forecastsvc.Point{
				{End: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), Balance: amount},
				{End: time.Date(2027, 9, 30, 0, 0, 0, 0, time.UTC), Balance: amount},
			},
		}
		forecast.Charts = append(forecast.Charts, forecastsvc.Chart{Commodity: commodity, Lines: []forecastsvc.Line{line}})
	}

	return forecast
}

func withForecaster(t *testing.T, forecaster *fakeForecaster) Model {
	t.Helper()
	model := New(journalPath, Ports{
		Loader: fakeLoader{ledger: accountsLedger()}, Checker: fakeChecker{}, Creator: &fakeCreator{},
		Editor: recordingEditor(nil), Forecast: forecaster,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, cmd := updated.Update(model.load())

	return deliver(updated.(Model), cmd)
}

func TestTheLoadedJournalIsForecastAndTheForecastViewShowsIt(t *testing.T) {
	forecaster := &fakeForecaster{forecast: oneChart("EUR")}
	model := withForecaster(t, forecaster)

	model, _ = press(model, "4")

	if len(forecaster.ledgers) != 1 {
		t.Fatalf("forecast built %d times, want once", len(forecaster.ledgers))
	}
	content := uitest.Plain(model.View().Content)
	for _, want := range []string{"╭─ EUR ", "╭─ MILESTONES ", "13.240,00 EUR"} {
		if !strings.Contains(content, want) {
			t.Errorf("view lacks %q:\n%s", want, content)
		}
	}

	if got := lastLine(model); !strings.HasPrefix(strings.TrimSpace(got), "forecast") || regexpPosition.MatchString(got) {
		t.Errorf("status bar = %q, want the view's name and no position", got)
	}
}

func TestRefreshProjectsTheReloadedJournalAgain(t *testing.T) {
	forecaster := &fakeForecaster{forecast: oneChart("EUR")}
	model := withForecaster(t, forecaster)

	model, cmd := press(model, "r")
	deliver(model, cmd)

	if len(forecaster.ledgers) != 2 {
		t.Errorf("forecast built %d times, want again after r", len(forecaster.ledgers))
	}
}

func TestAFailedForecastKeepsTheLastOneAndFlagsTheError(t *testing.T) {
	forecaster := &fakeForecaster{forecast: oneChart("EUR")}
	model := withForecaster(t, forecaster)
	forecaster.err = errors.New("hledger failed")

	model, cmd := press(model, "r")
	model = deliver(model, cmd)
	model, _ = press(model, "4")

	if !strings.Contains(lastLine(model), "✗ error") || !strings.Contains(uitest.Plain(model.View().Content), "13.240,00 EUR") {
		t.Errorf("status %q; view:\n%s", lastLine(model), uitest.Plain(model.View().Content))
	}
}

func TestChartsTheTerminalHasNoRoomForAreCountedInTheStatusBar(t *testing.T) {
	forecaster := &fakeForecaster{forecast: oneChart("EUR", "PLN", "CZK", "USD")}
	model := withForecaster(t, forecaster)
	small, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	shown, _ := press(small.(Model), "4")

	if got := lastLine(shown); !strings.Contains(got, "panels hidden · widen terminal") {
		t.Errorf("status bar = %q", got)
	}
}

func TestOneHiddenChartIsCountedInTheSingular(t *testing.T) {
	model := withForecaster(t, &fakeForecaster{forecast: oneChart("EUR", "PLN", "CZK")})
	small, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	shown, _ := press(small.(Model), "4")

	if got := lastLine(shown); !strings.Contains(got, "1 panel hidden · widen terminal") {
		t.Errorf("status bar = %q", got)
	}
}

func TestOtherViewsReportNoHiddenForecastPanels(t *testing.T) {
	model := withForecaster(t, &fakeForecaster{forecast: oneChart("EUR", "PLN", "CZK", "USD")})
	small, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	scheduled, _ := press(small.(Model), "3")

	if got := lastLine(scheduled); strings.Contains(got, "hidden") {
		t.Errorf("status bar = %q", got)
	}
}

func TestWithoutAForecasterTheForecastViewStaysEmpty(t *testing.T) {
	model, _ := press(withAccounts(t, &fakePlanner{}, &fakeWriter{}), "4")

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "loading the forecast") {
		t.Errorf("view:\n%s", content)
	}
}

func TestARefreshReplacesTheForecastOnScreen(t *testing.T) {
	forecaster := &fakeForecaster{forecast: oneChart("EUR")}
	model := withForecaster(t, forecaster)
	forecaster.forecast = oneChart("PLN")

	model, cmd := press(model, "r")
	model = deliver(model, cmd)
	model, _ = press(model, "4")

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "13.240,00 PLN") || strings.Contains(content, "13.240,00 EUR") {
		t.Errorf("view:\n%s", content)
	}
}
