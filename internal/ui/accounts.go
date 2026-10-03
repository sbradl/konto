package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
)

// prompt is a y/n question about a planned write; y applies the changeset.
type prompt struct {
	question  string
	changeset domain.Changeset
}

// onDashboardKey moves the Accounts cursor, or starts closing (c) or reopening (o) the selected account.
func (m Model) onDashboardKey(key string) Model {
	m.dash = m.dash.Move(key)
	if key == "c" || key == "o" {
		return m.planAccountChange(key)
	}
	return m
}

// planAccountChange handles c (close) and o (reopen) on the Dashboard's selected account:
// a refusal becomes a notice, a plan becomes a y/n prompt. Refusals that only mean "nothing to
// do" (closing a closed account, reopening an open one) stay quiet.
func (m Model) planAccountChange(key string) Model {
	account, ok := m.dash.Selected()
	if !ok {
		return m
	}
	plan, verb := m.accounts.Close, "close"
	if key == "o" {
		plan, verb = m.accounts.Reopen, "reopen"
	}
	changeset, err := plan(account)
	if errors.Is(err, domain.ErrAlreadyClosed) || errors.Is(err, domain.ErrNotClosed) {
		return m
	}
	if err != nil {
		m.notice = "cannot " + verb + " " + account.Name + ": " + err.Error()
		return m
	}
	m.prompt = &prompt{question: verb + " " + account.Name + "?", changeset: changeset}
	return m
}

// onPromptKey answers the open y/n prompt; every other key leaves it open.
func (m Model) onPromptKey(key string) (Model, tea.Cmd) {
	switch key {
	case "y":
		changeset := m.prompt.changeset
		m.prompt = nil
		return m, func() tea.Msg { return writtenMsg{err: m.writer.Apply(changeset)} }
	case "n", "esc":
		m.prompt = nil
	}
	return m, nil
}

// onWritten refreshes after a successful write and reports a failed one.
func (m Model) onWritten(written writtenMsg) (Model, tea.Cmd) {
	if written.err != nil {
		m.notice = "write failed: " + written.err.Error()
		return m, nil
	}
	m.refreshing = true
	return m, m.load
}

// onScheduledKey moves the rule cursor, or on d asks whether to delete the selected rule.
func (m Model) onScheduledKey(key string) Model {
	m.scheduled = m.scheduled.Move(key)
	if rule, ok := m.scheduled.Selected(); ok && key == "d" {
		m.prompt = &prompt{question: `delete rule "` + rule.Description + `"?`, changeset: rule.Delete()}
	}
	return m
}
