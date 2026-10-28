package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	forecastsvc "github.com/sbradl/konto/internal/forecast"
)

// countingLoader loads a journal with one more transaction on every load.
type countingLoader struct{ loads *int }

func (c countingLoader) Load(context.Context) (domain.Ledger, error) {
	*c.loads++
	return ledgerWith(*c.loads), nil
}

// countingReporter builds a report whose single row says which build it was.
type countingReporter struct{ builds *int }

func (c countingReporter) Report(_ context.Context, period domain.Period) (domain.Report, error) {
	*c.builds++
	return domain.Report{Period: period, IncomeStatement: []domain.ReportRow{
		{Kind: domain.RowLeaf, Label: fmt.Sprintf("build %d", *c.builds)},
	}}, nil
}

// countingForecaster projects a forecast whose commodity says which build it was.
type countingForecaster struct{ builds *int }

func (c countingForecaster) Forecast(context.Context, domain.Ledger) (forecastsvc.Forecast, error) {
	*c.builds++
	return oneChart(fmt.Sprintf("B%d", *c.builds)), nil
}

// commands are the commands a batch runs, run one by one so a test can order what they find out.
func commands(cmd tea.Cmd) []tea.Cmd {
	if batch, ok := cmd().(tea.BatchMsg); ok {
		return batch
	}
	return []tea.Cmd{cmd}
}

// run runs the commands in order and collects what they found out.
func run(cmds []tea.Cmd) []tea.Msg {
	msgs := make([]tea.Msg, len(cmds))
	for i, cmd := range cmds {
		msgs[i] = cmd()
	}
	return msgs
}

func generationModel(t *testing.T) Model {
	t.Helper()
	loads, reports, forecasts := 0, 0, 0
	model := New(journalPath, Ports{
		Loader: countingLoader{&loads}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Reports: countingReporter{&reports}, Forecast: countingForecaster{&forecasts},
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return deliver(updated.(Model), model.Init())
}

func TestAnOlderReloadLandingLastIsDropped(t *testing.T) {
	model := generationModel(t)
	model, first := press(model, "r")
	model, second := press(model, "r")
	older, newer := first(), second()

	updated, _ := model.Update(newer)
	updated, cmd := updated.Update(older)

	model = updated.(Model)
	if cmd != nil || !strings.Contains(firstLine(model), "3 txns") || model.refreshing {
		t.Errorf("cmd %v, refreshing %v, tab bar %q; want the newer load kept", cmd, model.refreshing, firstLine(model))
	}
}

func TestAReloadStillInFlightKeepsTheSpinnerWhenAnOlderOneLands(t *testing.T) {
	model := generationModel(t)
	model, first := press(model, "r")
	model, _ = press(model, "r")

	updated, _ := model.Update(first())

	if !updated.(Model).refreshing {
		t.Error("the spinner must show until the newest reload lands")
	}
}

func TestReportsAndForecastsOfAnOlderReloadAreDropped(t *testing.T) {
	model := generationModel(t)
	model, first := press(model, "r")
	updated, older := model.Update(first())
	model, second := press(updated.(Model), "r")
	updated, newer := model.Update(second())

	olderResults := run(commands(older))
	for _, msg := range append(run(commands(newer)), olderResults...) {
		updated, _ = updated.Update(msg)
	}

	model, _ = press(updated.(Model), "5")
	if content := plain(model.View().Content); !strings.Contains(content, "build 3") {
		t.Errorf("reports view:\n%s", content)
	}
	model, _ = press(model, "4")
	if content := plain(model.View().Content); !strings.Contains(content, "╭─ B3 ") {
		t.Errorf("forecast view:\n%s", content)
	}
}

func TestAnExportReadAgainForAnOlderReloadIsDropped(t *testing.T) {
	importer := listing()
	importer.reread = reread()
	model := stagedModel(t, importer)
	earlier, _ := model.imports.Staged()
	model, _ = press(model, "r")

	stale := reread()
	stale.Rows = stale.Rows[:1]

	updated, _ := model.Update(openedMsg{staged: stale, earlier: &earlier})

	if staged, _ := updated.(Model).imports.Staged(); len(staged.Rows) != len(earlier.Rows) {
		t.Errorf("rows %d, want the %d staged before the newer reload", len(staged.Rows), len(earlier.Rows))
	}
}
