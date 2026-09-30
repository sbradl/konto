// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	tea "charm.land/bubbletea/v2"
)

const tabBar = " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "

// Model is konto's single tea.Model.
type Model struct{}

// New returns the root model.
func New() Model { return Model{} }

// Init has nothing to load yet.
func (Model) Init() tea.Cmd { return nil }

// Update handles app-level input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && isQuit(key) {
		return m, tea.Quit
	}
	return m, nil
}

func isQuit(key tea.KeyPressMsg) bool {
	return key.String() == "q" || key.String() == "ctrl+c"
}

// View renders the tab bar.
func (Model) View() tea.View {
	view := tea.NewView(tabBar)
	view.AltScreen = true
	return view
}
