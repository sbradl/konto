// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"context"
	"errors"
	"fmt"
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
	ledger      *domain.Ledger
	loadErr     error
	firstRun    bool
	createErr   error
	width       int
	height      int
}

// New returns the root model for the journal at journalPath, loaded through
// loader; creator writes the journal on first run.
func New(journalPath string, loader ledgerLoader, creator journalCreator) Model {
	return Model{journalPath: journalPath, loader: loader, creator: creator}
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
	}
	if failed, ok := msg.(journalCreateFailedMsg); ok {
		m.createErr = failed.err
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
	}
	return nil
}

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
