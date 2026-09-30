// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
)

const tabs = " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "

type ledgerLoader interface {
	Load(ctx context.Context) (domain.Ledger, error)
}

// ledgerLoadedMsg carries the result of loading the journal.
type ledgerLoadedMsg struct {
	ledger domain.Ledger
	err    error
}

// Model is konto's single tea.Model.
type Model struct {
	journalPath string
	loader      ledgerLoader
	ledger      *domain.Ledger
	loadErr     error
	width       int
}

// New returns the root model for the journal at journalPath, loaded through loader.
func New(journalPath string, loader ledgerLoader) Model {
	return Model{journalPath: journalPath, loader: loader}
}

// Init starts loading the ledger.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		ledger, err := m.loader.Load(context.Background())
		return ledgerLoadedMsg{ledger: ledger, err: err}
	}
}

// Update handles app-level input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
	}
	if loaded, ok := msg.(ledgerLoadedMsg); ok {
		m.ledger, m.loadErr = &loaded.ledger, loaded.err
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && isQuit(key) {
		return m, tea.Quit
	}
	return m, nil
}

func isQuit(key tea.KeyPressMsg) bool {
	return key.String() == "q" || key.String() == "ctrl+c"
}

// View renders the tab bar.
func (m Model) View() tea.View {
	view := tea.NewView(m.content())
	view.AltScreen = true
	return view
}

func (m Model) content() string {
	if m.loadErr != nil {
		return m.tabBar() + "\n" + m.loadErr.Error()
	}
	return m.tabBar()
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
