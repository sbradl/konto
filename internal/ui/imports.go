package ui

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/imports"
	"github.com/sbradl/konto/internal/ui/layout"
)

// onImportKey hands a key to the Import view and runs what it asks for: listing a folder or opening a file.
func (m Model) onImportKey(key string) (Model, tea.Cmd) {
	if m.importer == nil {
		return m, nil
	}
	var action imports.Action
	m.imports, action = m.imports.Key(key)
	if action.Browse != "" {
		return m, func() tea.Msg {
			entries, err := m.importer.Browse(action.Browse)
			return browsedMsg{dir: action.Browse, entries: entries, err: err}
		}
	}
	if action.Open != "" {
		return m.openExport(action.Open)
	}
	if action.Write {
		return m.planImport(), nil
	}
	return m, nil
}

// openExport reads the export at path against the journal, once no write or reload is in flight.
func (m Model) openExport(path string) (Model, tea.Cmd) {
	if m.busy() {
		m.imports = m.imports.WithMessage("still reloading the journal — open the file again in a moment")
		return m, nil
	}
	return m, func() tea.Msg {
		staged, err := m.importer.Open(context.Background(), path, *m.ledger)
		return openedMsg{staged: staged, err: err}
	}
}

// planImport asks y/n before writing the staged import, showing what will be written; what keeps it from
// being written (rows without a category, a balance the bank does not share) is a message in the view.
func (m Model) planImport() Model {
	if m.busy() {
		m.imports = m.imports.WithMessage(reloadingNotice) // the staged rows are about to be read again
		return m
	}
	staged, _ := m.imports.Staged() // the view asks to write only with an import staged
	plan, err := staged.Plan()
	if err != nil {
		m.imports = m.imports.WithMessage(err.Error())
		return m
	}
	transactions := layout.Plural(plan.Count, "transaction")
	m.prompt = &prompt{
		question: "write " + transactions + " into " + plan.Account + "?", preview: plan.Preview,
		write: pendingWrite{
			changeset: plan.Changeset, verify: true, notice: "imported " + transactions + " into " + plan.Account,
			written: clearImport,
		},
	}
	return m
}

// reopen reads the staged export again against the reloaded journal, so what is skipped follows the journal.
func (m Model) reopen(loaded ledgerLoadedMsg) tea.Cmd {
	staged, ok := m.imports.Staged()
	if !ok || m.importer == nil || loaded.err != nil || staged.Path == "" {
		return nil
	}
	return func() tea.Msg {
		fresh, err := m.importer.Open(context.Background(), staged.Path, loaded.ledger)
		return openedMsg{staged: fresh, err: err, earlier: &staged, load: loaded.load}
	}
}

// onReopened takes the staged export read again in: what was staged carries over. An export read again
// for an earlier load, or after the import was written or replaced, is dropped, and one that cannot be read
// leaves the import as it was, with the error flagged.
func (m Model) onReopened(opened openedMsg) Model {
	current, ok := m.imports.Staged()
	switch {
	case !ok || current.Path != opened.earlier.Path || opened.load != m.loads:
	case opened.err != nil:
		m.failure = opened.err
	default:
		m.imports = m.imports.WithRefreshed(opened.staged.Restage(*opened.earlier))
	}
	return m
}

// onOpened stages the export, or says why it cannot be: a file konto cannot use is a message in the Import
// view; hledger failing on it also flags the error, so its words are in the checks panel.
func (m Model) onOpened(opened openedMsg) Model {
	if opened.earlier != nil {
		return m.onReopened(opened)
	}
	var convertErr csvimport.ConvertError
	switch {
	case opened.err == nil:
		m.imports = m.imports.WithStaged(opened.staged)
	case errors.As(opened.err, &convertErr):
		m.failure = convertErr
		first, _, _ := strings.Cut(strings.TrimSpace(convertErr.Error()), "\n")
		m.imports = m.imports.WithMessage("unreadable row or file: " + first)
	case errors.Is(opened.err, domain.ErrAccountClosed):
		m.imports = m.imports.WithMessage(opened.err.Error() + " — reopen with o")
	default:
		m.imports = m.imports.WithMessage(opened.err.Error())
	}
	return m
}
