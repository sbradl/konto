// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const tabs = " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "

// Model is konto's single tea.Model.
type Model struct {
	journalPath string
	width       int
}

// New returns the root model for the journal at journalPath.
func New(journalPath string) Model { return Model{journalPath: journalPath} }

// Init has nothing to load yet.
func (Model) Init() tea.Cmd { return nil }

// Update handles app-level input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
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
	view := tea.NewView(m.tabBar())
	view.AltScreen = true
	return view
}

// tabBar puts the tabs on the left and the journal path at the right edge.
func (m Model) tabBar() string {
	path := m.journalPath + " "
	gap := max(0, m.width-lipgloss.Width(tabs)-lipgloss.Width(path))
	return tabs + strings.Repeat(" ", gap) + path
}
