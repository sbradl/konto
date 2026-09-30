// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ledger"
)

const tabs = " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "

type ledgerLoader interface {
	Load(ctx context.Context) (domain.Ledger, error)
}

type journalCreator interface {
	Create() error
}

// journalCreateFailedMsg reports that the first-run journal could not be written.
type journalCreateFailedMsg struct{ err error }

// ledgerLoadedMsg carries the result of loading the journal.
type ledgerLoadedMsg struct {
	ledger domain.Ledger
	err    error
}

// Model is konto's single tea.Model.
type Model struct {
	journalPath string
	loader      ledgerLoader
	creator     journalCreator
	editor      func(file string, line int) *exec.Cmd
	ledger      *domain.Ledger
	loadErr     error
	firstRun    bool
	parseIssue  *domain.CheckIssue
	createErr   error
	width       int
	height      int
}

// editorClosedMsg reports that the user's editor has exited.
type editorClosedMsg struct{}

// New returns the root model for the journal at journalPath, loaded through
// loader; creator writes the journal on first run; editor builds the command
// that opens a file at a line.
func New(
	journalPath string, loader ledgerLoader, creator journalCreator, editor func(file string, line int) *exec.Cmd,
) Model {
	return Model{journalPath: journalPath, loader: loader, creator: creator, editor: editor}
}

// Init starts loading the ledger.
func (m Model) Init() tea.Cmd { return m.load }

func (m Model) load() tea.Msg {
	snapshot, err := m.loader.Load(context.Background())
	return ledgerLoadedMsg{ledger: snapshot, err: err}
}

// create writes the first-run journal, then loads it.
func (m Model) create() tea.Msg {
	if err := m.creator.Create(); err != nil {
		return journalCreateFailedMsg{err: err}
	}
	return m.load()
}

// Update handles app-level input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if loaded, ok := msg.(ledgerLoadedMsg); ok {
		m.ledger, m.loadErr = &loaded.ledger, loaded.err
		m.firstRun = errors.Is(loaded.err, ledger.ErrNoJournal)
		m.parseIssue = issueOf(loaded.err)
	}
	if failed, ok := msg.(journalCreateFailedMsg); ok {
		m.createErr = failed.err
	}
	if _, ok := msg.(editorClosedMsg); ok {
		return m, m.load
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		return m, m.onKey(key)
	}
	return m, nil
}

func (m Model) onKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "y":
		if m.firstRun {
			return m.create
		}
	case "n":
		if m.firstRun {
			return tea.Quit
		}
	case "enter":
		if m.parseIssue != nil {
			return m.openEditor(*m.parseIssue)
		}
	}
	return nil
}

// openEditor suspends konto while the user's editor shows the issue.
func (m Model) openEditor(issue domain.CheckIssue) tea.Cmd {
	return tea.ExecProcess(m.editor(issue.File, issue.Line), editorClosed)
}

// editorClosed turns the editor's exit, however it went, into a reload request.
func editorClosed(error) tea.Msg { return editorClosedMsg{} }

// View renders the tab bar.
func (m Model) View() tea.View {
	view := tea.NewView(m.content())
	view.AltScreen = true
	return view
}

func (m Model) content() string {
	if m.firstRun {
		return m.centred(m.firstRunText())
	}
	if m.parseIssue != nil {
		return m.centred(parseErrorText(*m.parseIssue))
	}
	if m.ledger == nil {
		return m.centred("loading journal…")
	}
	if m.loadErr != nil {
		return m.tabBar() + "\n" + m.loadErr.Error()
	}
	return m.tabBar()
}

func (m Model) firstRunText() string {
	text := "No journal found at " + m.journalPath + "\n\ny to create it · n to quit"
	if m.createErr != nil {
		text += "\n\n" + m.createErr.Error()
	}
	return text
}

func (m Model) centred(text string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, text)
}

// tabBar puts the tabs on the left and the journal path, with the
// transaction count once loaded, at the right edge.
func (m Model) tabBar() string {
	info := m.journalPath
	if m.ledger != nil {
		info += fmt.Sprintf(" · %d txns", m.ledger.TransactionCount())
	}
	info += " "
	gap := max(0, m.width-lipgloss.Width(tabs)-lipgloss.Width(info))
	return tabs + strings.Repeat(" ", gap) + info
}

// issueOf is the located journal issue behind err, if there is one.
func issueOf(err error) *domain.CheckIssue {
	var journalErr domain.JournalError
	if !errors.As(err, &journalErr) {
		return nil
	}
	return &journalErr.Issue
}

func parseErrorText(issue domain.CheckIssue) string {
	return issue.Severity.String() + " " + issue.Check + "  " + issue.Location() + "\n\n" +
		issue.Message + "\n\nEnter to jump to the editor · q to quit"
}
