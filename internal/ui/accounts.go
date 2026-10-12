package ui

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/dashboard"
	"github.com/sbradl/konto/internal/ui/imports"
	"github.com/sbradl/konto/internal/ui/layout"
)

// prompt is a y/n question about a planned write; y applies the changeset. An import's prompt also shows
// what will be written, and is checked before it is applied.
type prompt struct {
	question  string
	changeset domain.Changeset
	preview   []string
	top       int    // the first line of the preview on screen
	imported  string // the confirmation shown once an import is written
	quit      bool   // y leaves konto: the question is about discarding the staged import
}

// pendingWrite is a changeset on its way to the journal and what to say once it is there.
type pendingWrite struct {
	changeset domain.Changeset
	imported  string // the confirmation of an import: it also empties the Import view
	rule      string // the confirmation of a saved rule
}

// onDashboardKey hands a key to the Dashboard and carries out what it asks for: closing or reopening the
// selected account.
func (m Model) onDashboardKey(key string) Model {
	var action dashboard.Action
	m.dash, action = m.dash.Key(key)
	switch action {
	case dashboard.CloseSelected:
		return m.planAccountChange(m.accounts.Close, "close")
	case dashboard.ReopenSelected:
		return m.planAccountChange(m.accounts.Reopen, "reopen")
	case dashboard.NoAction:
	}
	return m
}

// planAccountChange plans closing or reopening the Dashboard's selected account: a refusal becomes a notice,
// a plan becomes a y/n prompt. Refusals that only mean "nothing to do" (closing a closed account, reopening
// an open one) stay quiet.
func (m Model) planAccountChange(plan func(domain.Account) (domain.Changeset, error), verb string) Model {
	account, ok := m.dash.Selected()
	if !ok {
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
	m.prompt = &prompt{question: verb + " " + account.Name + "?", changeset: changeset}
	return m
}

// onPromptKey answers the open y/n prompt; every other key leaves it open.
func (m Model) onPromptKey(key string) (Model, tea.Cmd) {
	switch key {
	case "y":
		pending := *m.prompt
		m.prompt = nil
		if pending.quit {
			return m, tea.Quit
		}
		return m.write(pendingWrite{changeset: pending.changeset, imported: pending.imported})
	case "n", "esc":
		m.prompt = nil
	case "up":
		m.prompt = m.prompt.scrolled(-1, m.lastPreviewTop())
	case "down":
		m.prompt = m.prompt.scrolled(1, m.lastPreviewTop())
	}
	return m, nil
}

// scrolled is the prompt with its preview moved by lines, between the first line and last, the last line
// the preview can start at; a new prompt, since the one open is shared with earlier models.
func (p *prompt) scrolled(lines, last int) *prompt {
	next := *p
	next.top = max(0, min(p.top+lines, last))
	return &next
}

// write writes the changeset; an import's and a rule's is checked first, so a journal that would fail its
// checks is never written. The spinner shows from now until the reload after it lands.
func (m Model) write(pending pendingWrite) (Model, tea.Cmd) {
	m.refreshing = true
	return m, func() tea.Msg {
		if (pending.imported != "" || pending.rule != "") && m.verifier != nil {
			if err := m.verifier.Verify(context.Background(), pending.changeset); err != nil {
				return writtenMsg{err: err}
			}
		}
		return writtenMsg{err: m.writer.Apply(pending.changeset), imported: pending.imported, rule: pending.rule}
	}
}

// onWritten refreshes after a successful write and reports a failed one. A written import is confirmed and
// leaves the Import view empty.
func (m Model) onWritten(written writtenMsg) (Model, tea.Cmd) {
	if written.err != nil {
		m.notice, m.refreshing = "write failed: "+written.err.Error(), false
		return m, nil
	}
	if written.imported != "" {
		m.notice, m.imports = written.imported, m.imports.Cleared()
	}
	if written.rule != "" {
		m.notice = written.rule
	}
	return m, m.load
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
	case action.Open != "" && m.refreshing:
		m.imports = m.imports.WithMessage("still reloading the journal — open the file again in a moment")
		return m, nil
	case action.Open != "":
		return m, func() tea.Msg {
			staged, err := m.importer.Open(context.Background(), action.Open, *m.ledger)
			return openedMsg{staged: staged, err: err}
		}
	case action.Write:
		return m.planImport(), nil
	}
	return m, nil
}

// planImport asks y/n before writing the staged import, showing what will be written; what keeps it from
// being written (rows without a category, a balance the bank does not share) is a message in the view.
func (m Model) planImport() Model {
	staged, _ := m.imports.Staged() // the view asks to write only with an import staged
	plan, err := staged.Plan()
	if err != nil {
		m.imports = m.imports.WithMessage(err.Error())
		return m
	}
	transactions := layout.Plural(plan.Count, "transaction")
	m.prompt = &prompt{
		question: "write " + transactions + " into " + plan.Account + "?", changeset: plan.Changeset,
		preview: plan.Preview, imported: "imported " + transactions + " into " + plan.Account,
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
		return openedMsg{staged: fresh, err: err, earlier: &staged}
	}
}

// onReopened takes the staged export read again in: what was staged carries over. An export read again
// after the import was written or replaced is dropped, and one that cannot be read leaves the import as it
// was, with the error flagged.
func (m Model) onReopened(opened openedMsg) Model {
	current, ok := m.imports.Staged()
	switch {
	case !ok || current.Path != opened.earlier.Path:
	case opened.err != nil:
		m.refreshErr = opened.err
	default:
		m.imports = m.imports.WithRefreshed(opened.staged.Carry(*opened.earlier))
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
		m.refreshErr = convertErr
		first, _, _ := strings.Cut(strings.TrimSpace(convertErr.Error()), "\n")
		m.imports = m.imports.WithMessage("unreadable row or file: " + first)
	default:
		m.imports = m.imports.WithMessage(opened.err.Error())
	}
	return m
}
