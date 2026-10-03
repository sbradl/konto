package ui

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/imports"
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

// onImportKey hands a key to the Import view and runs what it asks for: listing a folder or opening a file.
func (m Model) onImportKey(key string) (Model, tea.Cmd) {
	if m.importer == nil {
		return m, nil
	}
	var action imports.Action
	m.imports, action = m.imports.Key(key)
	switch {
	case action.Browse != "":
		return m, func() tea.Msg {
			entries, err := m.importer.Browse(action.Browse)
			return browsedMsg{dir: action.Browse, entries: entries, err: err}
		}
	case action.Open != "":
		return m, func() tea.Msg {
			staged, err := m.importer.Open(context.Background(), action.Open, *m.ledger)
			return openedMsg{staged: staged, err: err}
		}
	}
	return m, nil
}

// onOpened stages the export, or says why it cannot be: a file konto cannot use is a message in the Import
// view; hledger failing on it also flags the error, so its words are in the checks panel.
func (m Model) onOpened(opened openedMsg) Model {
	var convertErr csvimport.ConvertError
	switch {
	case opened.err == nil:
		m.imports = m.imports.WithStaged(opened.staged)
	case errors.As(opened.err, &convertErr):
		m.refreshErr = convertErr
		first, _, _ := strings.Cut(strings.TrimSpace(convertErr.Error()), "\n")
		m.imports = m.imports.WithMessage("unreadable row or file: " + first)
	default:
		m.imports = m.imports.WithMessage(opened.err.Error())
	}
	return m
}
