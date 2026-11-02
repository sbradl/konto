package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/dashboard"
)

// onDashboardKey hands a key to the Dashboard and carries out what it asks for: closing or reopening the
// selected account.
func (m Model) onDashboardKey(key string) (Model, tea.Cmd) {
	var action dashboard.Action
	m.dash, action = m.dash.Key(key)
	switch action.(type) {
	case dashboard.CloseSelected:
		return m.planAccountChange(m.accounts.Close, "close"), nil
	case dashboard.ReopenSelected:
		return m.planAccountChange(m.accounts.Reopen, "reopen"), nil
	}
	return m, nil
}

// planAccountChange plans closing or reopening the Dashboard's selected account: a refusal becomes a notice,
// a plan becomes a y/n prompt. Refusals that only mean "nothing to do" (closing a closed account, reopening
// an open one) stay quiet.
func (m Model) planAccountChange(plan func(domain.Account) (domain.Changeset, error), verb string) Model {
	account, ok := m.dash.Selected()
	if !ok {
		return m
	}
	if m.busy() {
		m.notice = reloadingNotice // the account may have just been closed or reopened
		return m
	}
	changeset, err := plan(account)
	if errors.Is(err, domain.ErrAlreadyClosed) || errors.Is(err, domain.ErrNotClosed) {
		return m
	}
	if err != nil {
		m.notice = "cannot " + verb + " " + account.Name + ": " + err.Error()
		return m
	}
	m.prompt = &prompt{question: verb + " " + account.Name + "?", write: pendingWrite{changeset: changeset}}
	return m
}
