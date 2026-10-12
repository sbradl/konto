package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
)

type fakeRules struct {
	changeset domain.Changeset
	input     periodic.Input
	err       error
	formErr   error
	saved     []periodic.Input
	replaced  []domain.PeriodicRule
	shown     []domain.PeriodicRule
}

func (f *fakeRules) Add(_ context.Context, input periodic.Input, _ domain.Ledger) (domain.Changeset, error) {
	f.saved = append(f.saved, input)
	return f.changeset, f.err
}

func (f *fakeRules) Replace(
	_ context.Context, input periodic.Input, rule domain.PeriodicRule, _ domain.Ledger,
) (domain.Changeset, error) {
	f.saved = append(f.saved, input)
	f.replaced = append(f.replaced, rule)
	return f.changeset, f.err
}

func (f *fakeRules) Form(_ context.Context, rule domain.PeriodicRule, _ domain.Ledger) (periodic.Input, error) {
	f.shown = append(f.shown, rule)
	return f.input, f.formErr
}

var addRuleEdit = domain.Changeset{Edits: []domain.Edit{domain.AddRule{Lines: []string{"~ monthly from 2026-10-15  Book club"}}}}

type ruleRig struct {
	model    Model
	rules    *fakeRules
	writer   *fakeWriter
	verifier *fakeVerifier
}

func scheduledWithForm(t *testing.T, rules *fakeRules) ruleRig {
	t.Helper()
	writer, verifier := &fakeWriter{}, &fakeVerifier{}
	ledger := scheduledLedger()
	ledger.Commodities = []string{"EUR"}
	ledger.Accounts = []domain.Account{{Name: "assets:checking"}, {Name: "expenses:books"}}
	model := New(journalPath, Ports{
		Loader: fakeLoader{ledger: ledger}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Writer: writer, Verifier: verifier, Rules: rules,
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())
	opened, _ := press(updated.(Model), "3")
	return ruleRig{model: opened, rules: rules, writer: writer, verifier: verifier}
}

func keysFor(model Model, keys ...string) Model {
	for _, key := range keys {
		if key == "esc" {
			model = pressEsc(model)
			continue
		}
		model, _ = press(model, key)
	}
	return model
}

func TestNOpensTheFormAndItsKeysAreTextNotViewKeys(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{})

	model, _ := press(rig.model, "n")
	model = keysFor(model, "tab", "tab", "tab")
	for _, key := range []string{"q", "2", "r", "!", "?"} {
		var cmd tea.Cmd
		model, cmd = press(model, key)
		if cmd != nil || model.active != scheduledTab || model.popup != popupNone || model.refreshing {
			t.Fatalf("%q leaked out of the form", key)
		}
	}

	content := plain(model.View().Content)
	if !strings.Contains(content, "new periodic rule") || !strings.Contains(content, "description         q2r!?▏") {
		t.Errorf("view:\n%s", content)
	}
}

func TestEscClosesTheFormAndWritesNothing(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{})
	model, _ := press(rig.model, "n")

	model = pressEsc(model)

	if strings.Contains(plain(model.View().Content), "new periodic rule") || len(rig.writer.applied) != 0 {
		t.Error("Esc did not close the form without writing")
	}
	if model, _ = press(model, "1"); model.active != 0 {
		t.Error("view keys do not work again")
	}
}

func TestNAndEDoNothingWithoutAPlanner(t *testing.T) {
	model, _ := press(withRules(t, &fakeWriter{}), "n")
	model, cmd := press(model, "e")

	if cmd != nil || model.scheduled.Capturing() {
		t.Error("n or e opened a form with no planner")
	}
}

func TestSavingPlansTheRuleThenChecksAndWritesItAndRefreshes(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{changeset: addRuleEdit})
	model, _ := press(rig.model, "n")

	model, cmd := press(model, "enter")
	updated, write := model.Update(cmd())
	model = updated.(Model)
	if !model.refreshing {
		t.Error("the spinner should show while the rule is written")
	}
	written := write()
	done, reload := model.Update(written)

	if len(rig.rules.saved) != 1 || len(rig.verifier.checked) != 1 || len(rig.writer.applied) != 1 {
		t.Fatalf("planned %d, checked %d, written %d; want one each", len(rig.rules.saved), len(rig.verifier.checked), len(rig.writer.applied))
	}
	if model.scheduled.Capturing() || !done.(Model).refreshing || reload == nil {
		t.Error("the form should close and the journal reload")
	}
	if got := lastLine(done.(Model)); !strings.Contains(got, "rule added") {
		t.Errorf("status bar = %q", got)
	}
}

func TestARefusedRuleKeepsTheFormOpenWithTheReason(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{err: domain.RefusalError{Reason: "description is required"}})
	model, _ := press(rig.model, "n")
	model, cmd := press(model, "enter")

	updated, next := model.Update(cmd())

	shown := updated.(Model)
	content := plain(shown.View().Content)
	if next != nil || !shown.scheduled.Capturing() || !strings.Contains(content, "description is required") ||
		len(rig.writer.applied) != 0 || shown.refreshErr != nil {
		t.Errorf("next %v, error %v; view:\n%s", next, shown.refreshErr, content)
	}
}

func TestAFailureOfHledgerWhilePlanningIsShownInTheFormAndFlagged(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{err: errors.New("hledger failed")})
	model, _ := press(rig.model, "n")
	model, cmd := press(model, "enter")

	updated, _ := model.Update(cmd())

	shown := updated.(Model)
	if !strings.Contains(plain(shown.View().Content), "hledger failed") || shown.refreshErr == nil {
		t.Errorf("error %v; view:\n%s", shown.refreshErr, plain(shown.View().Content))
	}
}

func TestAFailedPreCheckOrWriteIsReported(t *testing.T) {
	for name, arrange := range map[string]func(ruleRig){
		"check": func(rig ruleRig) { rig.verifier.err = errors.New("the journal would fail its checks") },
		"write": func(rig ruleRig) { rig.writer.err = errors.New("disk full") },
	} {
		rig := scheduledWithForm(t, &fakeRules{changeset: addRuleEdit})
		arrange(rig)
		model, _ := press(rig.model, "n")
		model, cmd := press(model, "enter")
		updated, write := model.Update(cmd())

		done, _ := updated.Update(write())

		if got := lastLine(done.(Model)); !strings.Contains(got, "write failed") {
			t.Errorf("%s: status bar = %q", name, got)
		}
	}
}

func TestEOpensTheFormFilledFromTheRuleAndSavingReplacesIt(t *testing.T) {
	input := periodic.Input{
		Date: "2026-10-15", Frequency: periodic.Quarterly, Description: "Rent", Amount: "25,00", Currency: "EUR",
		From: "assets:checking", To: "expenses:books",
	}
	rig := scheduledWithForm(t, &fakeRules{input: input, changeset: addRuleEdit})

	model, cmd := press(rig.model, "e")
	model = deliver(model, cmd)

	content := plain(model.View().Content)
	if !strings.Contains(content, "edit periodic rule") || !strings.Contains(content, "[quarterly]") || !strings.Contains(content, "Rent") ||
		len(rig.rules.shown) != 1 || rig.rules.shown[0].Description != "Rent" {
		t.Fatalf("shown %v; view:\n%s", rig.rules.shown, content)
	}
	model, cmd = press(model, "enter")
	updated, write := model.Update(cmd())
	done, _ := updated.Update(write())

	if len(rig.rules.replaced) != 1 || rig.rules.replaced[0].Description != "Rent" || !strings.Contains(lastLine(done.(Model)), "rule updated") {
		t.Errorf("replaced %v, status %q", rig.rules.replaced, lastLine(done.(Model)))
	}
}

func TestERefusedOnARuleThatDoesNotFitSaysSoAndOpensNothing(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{formErr: periodic.ErrNotEditable})

	model, cmd := press(rig.model, "e")
	model = deliver(model, cmd)

	if model.scheduled.Capturing() || !strings.Contains(lastLine(model), "edit the journal by hand") ||
		!strings.Contains(lastLine(model), "cannot edit Rent") {
		t.Errorf("status bar = %q", lastLine(model))
	}
}

func TestEWithNoRuleSelectedDoesNothing(t *testing.T) {
	ledger := scheduledLedger()
	ledger.Rules, ledger.Upcoming = nil, nil
	model := New(journalPath, Ports{
		Loader: fakeLoader{ledger: ledger}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Rules: &fakeRules{},
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())
	opened, _ := press(updated.(Model), "3")

	if _, cmd := press(opened, "e"); cmd != nil {
		t.Error("e asked for a form with no rule")
	}
}

func TestCtrlCQuitsWhileTheFormIsOpen(t *testing.T) {
	rig := scheduledWithForm(t, &fakeRules{})
	model, _ := press(rig.model, "n")

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	quitCommand(t, cmd)
}

func TestEAndDWaitForTheReloadBecauseTheRulesMayHaveMoved(t *testing.T) {
	rules := &fakeRules{}
	rig := scheduledWithForm(t, rules)
	rig.model.refreshing = true

	for _, key := range []string{"e", "d"} {
		model, cmd := press(rig.model, key)

		if cmd != nil || model.prompt != nil || len(rules.shown) != 0 || !strings.Contains(lastLine(model), "reloading") {
			t.Errorf("%s: status %q, prompt %v", key, lastLine(model), model.prompt)
		}
	}
}
