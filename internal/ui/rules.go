package ui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/ui/scheduled"
)

// rulePlanner plans what the Scheduled form writes and what it shows for an existing rule.
type rulePlanner interface {
	Add(ctx context.Context, input periodic.Input, snapshot domain.Ledger) (domain.Changeset, error)
	Replace(
		ctx context.Context, input periodic.Input, rule domain.PeriodicRule, snapshot domain.Ledger,
	) (domain.Changeset, error)
	Form(ctx context.Context, rule domain.PeriodicRule, snapshot domain.Ledger) (periodic.Input, error)
}

// formReadyMsg carries what the form shows for the rule being edited, or why it cannot.
type formReadyMsg struct {
	input periodic.Input
	rule  domain.PeriodicRule
	err   error
}

// ruleSavedMsg carries the changeset planned for the form's rule, or why the rule was refused.
type ruleSavedMsg struct {
	changeset domain.Changeset
	editing   bool
	err       error
}

// onScheduledKey moves the rule cursor, opens the form with n and e, or on d asks whether to delete the
// selected rule.
func (m Model) onScheduledKey(key string) (Model, tea.Cmd) {
	m.scheduled = m.scheduled.Move(key)
	if key == "n" && m.rules != nil {
		m.scheduled = m.scheduled.OpenNew()
		return m, nil
	}
	rule, selected := m.scheduled.Selected()
	if !selected || (key != "e" && key != "d") {
		return m, nil
	}
	if m.refreshing {
		m.notice = "the journal is reloading — try again in a moment" // the rule's place may have moved
		return m, nil
	}
	if key == "e" && m.rules != nil {
		return m, m.readRule(rule)
	}
	if key == "d" {
		m.prompt = &prompt{question: `delete rule "` + rule.Description + `"?`, changeset: rule.Delete()}
	}
	return m, nil
}

// readRule asks what the form shows for the rule.
func (m Model) readRule(rule domain.PeriodicRule) tea.Cmd {
	return func() tea.Msg {
		input, err := m.rules.Form(context.Background(), rule, *m.ledger)
		return formReadyMsg{input: input, rule: rule, err: err}
	}
}

// onFormReady opens the form on the rule, or says why it cannot be edited.
func (m Model) onFormReady(ready formReadyMsg) Model {
	if ready.err != nil {
		m.notice = "cannot edit " + ready.rule.Description + ": " + ready.err.Error()
		return m
	}
	m.scheduled = m.scheduled.OpenEdit(ready.input, ready.rule)
	return m
}

// onFormKey hands a key to the open form; saving asks for the rule to be planned.
func (m Model) onFormKey(key string) (Model, tea.Cmd) {
	var action scheduled.Action
	m.scheduled, action = m.scheduled.FormKey(key)
	if action.Save == nil {
		return m, nil
	}
	save := *action.Save
	return m, func() tea.Msg {
		var (
			changeset domain.Changeset
			err       error
		)
		if save.Editing != nil {
			changeset, err = m.rules.Replace(context.Background(), save.Input, *save.Editing, *m.ledger)
		} else {
			changeset, err = m.rules.Add(context.Background(), save.Input, *m.ledger)
		}
		return ruleSavedMsg{changeset: changeset, editing: save.Editing != nil, err: err}
	}
}

// onRuleSaved writes the planned rule and closes the form, or keeps the form open with the reason the rule was
// refused; a failure of hledger or the journal is flagged as an error as well.
func (m Model) onRuleSaved(saved ruleSavedMsg) (Model, tea.Cmd) {
	if saved.err != nil {
		if _, refused := errors.AsType[domain.RefusalError](saved.err); !refused {
			m.refreshErr = saved.err
		}
		m.scheduled = m.scheduled.WithFormError(saved.err.Error())
		return m, nil
	}
	m.scheduled = m.scheduled.CloseForm()
	notice := "rule added"
	if saved.editing {
		notice = "rule updated"
	}
	return m, m.writeCmd(pendingWrite{changeset: saved.changeset, rule: notice})
}

// onCapturedKey hands a key to the view that has the keyboard: the Scheduled form or the Import dialog.
func (m Model) onCapturedKey(key string) (Model, tea.Cmd) {
	if m.active == scheduledTab {
		return m.onFormKey(key)
	}
	return m.onImportKey(key)
}
