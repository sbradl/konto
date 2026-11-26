package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
)

// countingLoader loads a journal with one more transaction on every load.
type countingLoader struct{ loads *int }

func (c countingLoader) Load(context.Context) (domain.Ledger, error) {
	*c.loads++

	return ledgerWith(*c.loads), nil
}

func generationModel(t *testing.T) Model {
	t.Helper()
	loads := 0
	model := New(journalPath, Ports{
		Loader: countingLoader{&loads}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
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
	if cmd != nil || !strings.Contains(firstLine(model), "3 txns") || model.busy() {
		t.Errorf("cmd %v, refreshing %v, tab bar %q; want the newer load kept", cmd, model.busy(), firstLine(model))
	}
}

func TestEachReloadIsNumberedOneOnFromTheFirstLoad(t *testing.T) {
	model, _ := press(generationModel(t), "r")

	updated, _ := model.Update(ledgerLoadedMsg{load: 1, ledger: ledgerWith(7)})

	if got := firstLine(updated.(Model)); !strings.Contains(got, "7 txns") {
		t.Errorf("tab bar %q; want the result of reload 1 taken in", got)
	}
}

func TestAReloadStillInFlightKeepsTheSpinnerWhenAnOlderOneLands(t *testing.T) {
	model := generationModel(t)
	model, first := press(model, "r")
	model, _ = press(model, "r")

	updated, _ := model.Update(first())

	if !updated.(Model).busy() {
		t.Error("the spinner must show until the newest reload lands")
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

func TestAnExportOpenedBeforeAReloadIsDropped(t *testing.T) {
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: sampleStaged()}
	model := startedWithImporter(t, importer)
	model, cmd := press(model, "n")
	model = deliver(model, cmd)
	model, _ = pressCode(model, tea.KeyDown)
	model, open := pressCode(model, tea.KeyEnter)
	model, reload := press(model, "r")
	model = deliver(model, reload)

	updated, _ := model.Update(open())

	if _, staged := updated.(Model).imports.Staged(); staged {
		t.Error("an export read against the journal before the reload was staged")
	}
}

// deliver feeds the messages a command produces back into the model until nothing is left.
func deliver(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		if _, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, c := range msg.(tea.BatchMsg) {
				m = deliver(m, c)
			}

			return m
		}
		updated, next := m.Update(msg)
		m, cmd = updated.(Model), next
	}

	return m
}
