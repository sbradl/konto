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

// formReadyMsg carries what the form shows for the rule being edited, or why it cannot; load is the load the
// rule was read from.
type formReadyMsg struct {
	load  int
	input periodic.Input
	rule  domain.PeriodicRule
	err   error
}

func (msg formReadyMsg) takeIn(m Model) Model { return m.onFormReady(msg) }

// ruleSavedMsg carries the changeset planned for the form's rule, or why the rule was refused.
type ruleSavedMsg struct {
	changeset domain.Changeset
	editing   bool
	err       error
}

// onScheduledKey hands a key to the rule list and carries out what it asks for: the form on a new rule or
// the selected one, or the question whether to delete the selected rule.
func (m Model) onScheduledKey(key string) (Model, tea.Cmd) {
	var action scheduled.ListAction
	m.scheduled, action = m.scheduled.Key(key)
	if action == scheduled.NewRule && m.rules != nil {
		m.scheduled = m.scheduled.OpenNew()
		return m, nil
	}
	rule, _ := m.scheduled.Selected() // edit and delete come only with a rule selected
	if action != scheduled.EditSelected && action != scheduled.DeleteSelected {
		return m, nil
	}
	if m.busy() {
		m.notice = reloadingNotice // the rule's place may have moved
		return m, nil
	}
	if action == scheduled.EditSelected && m.rules != nil {
		return m, m.readRule(rule)
	}
	if action == scheduled.DeleteSelected {
		question := `delete rule "` + rule.Description + `"?`
		m.prompt = &prompt{question: question, write: pendingWrite{changeset: rule.Delete()}}
	}
	return m, nil
}

// readRule asks what the form shows for the rule.
func (m Model) readRule(rule domain.PeriodicRule) tea.Cmd {
	return func() tea.Msg {
		input, err := m.rules.Form(context.Background(), rule, *m.ledger)
		return formReadyMsg{load: m.loads, input: input, rule: rule, err: err}
	}
}

// onFormReady opens the form on the rule, or says why it cannot be edited. A rule read before the journal
// changed, or while it is being changed, asked about or another form opened, is dropped: its place in the
// journal may be gone.
func (m Model) onFormReady(ready formReadyMsg) Model {
	switch {
	case ready.load != m.loads || m.busy() || m.prompt != nil || m.scheduled.Capturing():
		return m
	case ready.err != nil:
		m.notice = "cannot edit " + ready.rule.Description + ": " + ready.err.Error()
		return m
	}
	m.scheduled = m.scheduled.OpenEdit(ready.input, ready.rule)
	return m
}

// onFormKey hands a key to the open form; saving asks for the rule to be planned, once no write or reload
// is in flight.
func (m Model) onFormKey(key string) (Model, tea.Cmd) {
	var action scheduled.Action
	m.scheduled, action = m.scheduled.FormKey(key)
	if action.Save == nil {
		return m, nil
	}
	if m.busy() {
		m.scheduled = m.scheduled.WithFormError(reloadingNotice)
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

// onRuleSaved writes the planned rule, closing the form once it is written, or keeps the form open with the
// reason the rule was refused; a failure of hledger or the journal is flagged as an error as well.
func (m Model) onRuleSaved(saved ruleSavedMsg) (Model, tea.Cmd) {
	if saved.err != nil {
		if _, refused := errors.AsType[domain.RefusalError](saved.err); !refused {
			m.failure = saved.err
		}
		m.scheduled = m.scheduled.WithFormError(saved.err.Error())
		return m, nil
	}
	notice := "rule added"
	if saved.editing {
		notice = "rule updated"
	}
	return m.write(pendingWrite{changeset: saved.changeset, verify: true, notice: notice, written: closeForm})
}

// closeForm closes the Scheduled form on the rule just written.
func closeForm(m Model) Model {
	m.scheduled = m.scheduled.CloseForm()
	return m
}
