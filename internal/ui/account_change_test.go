package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

type fakePlanner struct {
	err       error
	closed    []string
	reopened  []string
	changeset domain.Changeset
}

func (f *fakePlanner) Close(account domain.Account) (domain.Changeset, error) {
	f.closed = append(f.closed, account.Name)

	return f.changeset, f.err
}

func (f *fakePlanner) Reopen(account domain.Account) (domain.Changeset, error) {
	f.reopened = append(f.reopened, account.Name)

	return f.changeset, f.err
}

// fakeWriter prepares a changeset by adding carry to it, and records what it prepares and applies.
type fakeWriter struct {
	carry      []domain.Edit
	prepared   []domain.Changeset
	prepareErr error
	applied    []domain.Changeset
	err        error
}

func (f *fakeWriter) Prepare(_ context.Context, changeset domain.Changeset) (domain.Changeset, error) {
	f.prepared = append(f.prepared, changeset)

	return domain.Changeset{Edits: append(slices.Clone(changeset.Edits), f.carry...)}, f.prepareErr
}

func (f *fakeWriter) Apply(changeset domain.Changeset) error {
	f.applied = append(f.applied, changeset)

	return f.err
}

var closeSpare = domain.Changeset{Edits: []domain.Edit{domain.SetAccountTag{Account: "assets:checking", Key: "konto:closed", Value: "2026-09-30"}}}

func withAccounts(t *testing.T, planner *fakePlanner, writer *fakeWriter) Model {
	t.Helper()

	return withLedger(t, accountsLedger(), planner, writer)
}

func withLedger(t *testing.T, ledger domain.Ledger, planner *fakePlanner, writer *fakeWriter) Model {
	t.Helper()
	model := New(journalPath, Ports{
		Loader: fakeLoader{ledger: ledger}, Checker: fakeChecker{}, Creator: &fakeCreator{},
		Editor: recordingEditor(nil), Accounts: planner, Writer: writer,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())

	return updated.(Model)
}

func TestCAsksBeforeClosingTheSelectedAccount(t *testing.T) {
	planner, writer := &fakePlanner{changeset: closeSpare}, &fakeWriter{}
	model, _ := press(withAccounts(t, planner, writer), "c")

	if got := lastLine(model); !strings.Contains(got, "close assets:checking? (y/n)") {
		t.Errorf("status bar = %q, want the close question", got)
	}

	if len(writer.applied) != 0 {
		t.Error("nothing is written before the answer")
	}
}

func TestCAndOWaitForTheReloadBecauseTheAccountMayHaveChanged(t *testing.T) {
	planner := &fakePlanner{changeset: closeSpare}
	model := withAccounts(t, planner, &fakeWriter{})
	model.reloading = true

	for _, key := range []string{"c", "o"} {
		pressed, _ := press(model, key)

		if pressed.prompt != nil || !strings.Contains(lastLine(pressed), "try again in a moment") {
			t.Errorf("%s: status %q, prompt %v", key, lastLine(pressed), pressed.prompt)
		}
	}
}

func TestYAppliesThePlannedChangeAndRefreshes(t *testing.T) {
	planner, writer := &fakePlanner{changeset: closeSpare}, &fakeWriter{}
	model, _ := press(withAccounts(t, planner, writer), "c")

	model, cmd := press(model, "y")

	if model.prompt != nil || cmd == nil {
		t.Fatalf("prompt = %v, cmd = %v; want the prompt closed and the write started", model.prompt, cmd)
	}
	written := cmd()

	if len(writer.applied) != 1 || writer.applied[0].Edits[0] != closeSpare.Edits[0] {
		t.Errorf("applied = %+v, want the planned changeset", writer.applied)
	}

	updated, reload := model.Update(written)
	if !updated.(Model).busy() || reload == nil {
		t.Error("a successful write should start a refresh")
	}
}

func TestTheSpinnerShowsFromYOnSoTheWriteIsNeverMistakenForDone(t *testing.T) {
	model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, &fakeWriter{}), "c")

	model, _ = press(model, "y")

	if !model.busy() || !strings.Contains(uitest.Plain(model.View().Content), "⣾") {
		t.Error("the spinner should show while the write is in flight")
	}
}

func TestAFailedWriteIsReportedAndNotRefreshed(t *testing.T) {
	model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, &fakeWriter{}), "c")
	model, _ = press(model, "y")

	updated, cmd := model.Update(writtenMsg{err: errors.New("disk full")})

	if got := lastLine(updated.(Model)); !strings.Contains(got, "write failed: disk full") || cmd != nil {
		t.Errorf("status bar = %q, cmd = %v; want the failure shown and no refresh", got, cmd)
	}

	if updated.(Model).busy() {
		t.Error("the spinner should stop once the write has failed")
	}
}

func TestNEscAndOtherKeysDoNotWrite(t *testing.T) {
	for key, wantOpen := range map[string]bool{"n": false, "x": true, "2": true, "?": true, "r": true} {
		writer := &fakeWriter{}
		model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, writer), "c")

		model, cmd := press(model, key)

		if (model.prompt != nil) != wantOpen || cmd != nil || len(writer.applied) != 0 || model.active != 0 || model.popup != popupNone {
			t.Errorf("%q: prompt open = %v, cmd = %v, active = %d; want open = %v and nothing else", key, model.prompt != nil, cmd, model.active, wantOpen)
		}
	}

	model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, &fakeWriter{}), "c")
	if model = pressEsc(model); model.prompt != nil {
		t.Error("Esc should cancel the prompt")
	}
}

func TestQuitStillWorksWithAPromptOpen(t *testing.T) {
	model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, &fakeWriter{}), "c")

	if _, cmd := press(model, "q"); cmd == nil {
		t.Error("q should quit")
	}
}

func TestARefusalIsShownUntilTheNextKey(t *testing.T) {
	planner := &fakePlanner{err: domain.RefusalError{Reason: "balance is 13.240,00 EUR"}}
	model, _ := press(withAccounts(t, planner, &fakeWriter{}), "c")

	if got := lastLine(model); !strings.Contains(got, "cannot close assets:checking: balance is 13.240,00 EUR") || model.prompt != nil {
		t.Errorf("status bar = %q, prompt = %v; want the refusal and no prompt", got, model.prompt)
	}

	model, _ = press(model, "x")
	if got := lastLine(model); strings.Contains(got, "cannot close") {
		t.Errorf("status bar = %q, want the refusal gone after a key", got)
	}
}

func TestNothingToDoRefusalsStayQuiet(t *testing.T) {
	for key, err := range map[string]error{"c": domain.ErrAlreadyClosed, "o": domain.ErrNotClosed} {
		model, _ := press(withAccounts(t, &fakePlanner{err: err}, &fakeWriter{}), key)

		if model.notice != "" || model.prompt != nil {
			t.Errorf("%s: notice = %q, prompt = %v; want nothing", key, model.notice, model.prompt)
		}
	}
}

func TestOReopensTheSelectedAccount(t *testing.T) {
	planner := &fakePlanner{changeset: closeSpare}
	model, _ := press(withAccounts(t, planner, &fakeWriter{}), "o")

	if got := lastLine(model); !strings.Contains(got, "reopen assets:checking? (y/n)") || len(planner.reopened) != 1 {
		t.Errorf("status bar = %q, reopened = %v", got, planner.reopened)
	}
}

func TestAccountKeysDoNothingOutsideTheDashboardOrWithAPopupOpen(t *testing.T) {
	planner := &fakePlanner{changeset: closeSpare}
	other, _ := press(withAccounts(t, planner, &fakeWriter{}), "2")
	popup, _ := press(withAccounts(t, planner, &fakeWriter{}), "?")

	for name, model := range map[string]Model{"import view": other, "keys popup": popup} {
		for _, key := range []string{"c", "o"} {
			if got, _ := press(model, key); got.prompt != nil || got.notice != "" {
				t.Errorf("%s: %s opened a prompt or notice", name, key)
			}
		}
	}

	if len(planner.closed)+len(planner.reopened) != 0 {
		t.Error("the planner must not be asked")
	}
}

func TestAccountKeysOnAnEmptyDashboardDoNothing(t *testing.T) {
	model := New(journalPath, Ports{Loader: fakeLoader{}, Checker: fakeChecker{}, Accounts: &fakePlanner{}, Writer: &fakeWriter{}})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())

	if got, _ := press(updated.(Model), "c"); got.prompt != nil || got.notice != "" {
		t.Error("c with no account selected should do nothing")
	}
}

func TestAReloadLandingWhileAWriteIsInFlightKeepsTheModelBusy(t *testing.T) {
	model, _ := press(withAccounts(t, &fakePlanner{changeset: closeSpare}, &fakeWriter{}), "c")
	model, _ = press(model, "y")
	model, reload := press(model, "r")

	updated, _ := model.Update(reload())

	if !updated.(Model).busy() || !strings.Contains(lastLine(updated.(Model)), "⣾") {
		t.Errorf("status %q; want busy until the write lands", lastLine(updated.(Model)))
	}
}
