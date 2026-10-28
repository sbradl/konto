package ui

import tea "charm.land/bubbletea/v2"

// tabCount is the number of v1 views.
const tabCount = 5

// tab is one view as the root drives it; adding a view is adding a tab. A view without a cursor, hidden
// panels or a dialog of its own leaves the field nil; captures and captured are set together.
type tab struct {
	name     string
	keys     []keyBinding                         // what the status bar and the keys popup list
	resize   func(Model, int, int) Model          // gives the view its content area
	view     func(Model) string                   // draws the view
	key      func(Model, string) (Model, tea.Cmd) // hands the view a key and runs what it asks for
	position func(Model) (row, total int)         // the cursor row and the row count
	hidden   func(Model) int                      // how many panels the terminal is too small to show
	captures func(Model) bool                     // whether a dialog of the view has the keyboard
	captured func(Model, string) (Model, tea.Cmd) // hands that dialog a key
}

// tabs are the five v1 views, in tab order; number key i+1 selects tab i.
func tabs() [tabCount]tab {
	return [tabCount]tab{
		{
			name: "dash", keys: []keyBinding{{"c", "close"}, {"o", "reopen"}},
			resize:   func(m Model, w, h int) Model { m.dash = m.dash.Resize(w, h); return m },
			view:     func(m Model) string { return m.dash.View() },
			key:      Model.onDashboardKey,
			position: func(m Model) (row, total int) { return m.dash.Position() },
			hidden:   func(m Model) int { return m.dash.HiddenPanels() },
		},
		{
			name: "import", keys: []keyBinding{{"n", "new file"}, {"Enter", "categorise"}, {"w", "write"}},
			resize:   func(m Model, w, h int) Model { m.imports = m.imports.Resize(w, h); return m },
			view:     func(m Model) string { return m.imports.View() },
			key:      Model.onImportKey,
			position: func(m Model) (row, total int) { return m.imports.Position() },
			captures: func(m Model) bool { return m.importer != nil && m.imports.Capturing() },
			captured: Model.onImportKey,
		},
		{
			name: "sched", keys: []keyBinding{{"n", "new"}, {"e", "edit"}, {"d", "delete"}},
			resize:   func(m Model, w, h int) Model { m.scheduled = m.scheduled.Resize(w, h); return m },
			view:     func(m Model) string { return m.scheduled.View() },
			key:      Model.onScheduledKey,
			position: func(m Model) (row, total int) { return m.scheduled.Position() },
			captures: func(m Model) bool { return m.scheduled.Capturing() },
			captured: Model.onFormKey,
		},
		{
			name:   "forecast",
			resize: func(m Model, w, h int) Model { m.forecasts = m.forecasts.Resize(w, h); return m },
			view:   func(m Model) string { return m.forecasts.View() },
			hidden: func(m Model) int { return m.forecasts.HiddenPanels() },
		},
		{
			name: "reports", keys: []keyBinding{{"p", "period"}},
			resize: func(m Model, w, h int) Model { m.reports = m.reports.Resize(w, h); return m },
			view:   func(m Model) string { return m.reports.View() },
			key:    Model.onReportsKey,
		},
	}
}

// activeTab is the view on screen.
func (m Model) activeTab() tab { return tabs()[m.active] }

// onActiveViewKey hands a key to the view on screen.
func (m Model) onActiveViewKey(key string) (Model, tea.Cmd) {
	if handle := m.activeTab().key; handle != nil {
		return handle(m, key)
	}
	return m, nil
}

// onCapturedKey hands a key to the dialog that has the keyboard: the Import dialog or the Scheduled form.
func (m Model) onCapturedKey(key string) (Model, tea.Cmd) { return m.activeTab().captured(m, key) }

// position is the active view's cursor row and row count; views without a cursor have none.
func (m Model) position() (row, total int) {
	if position := m.activeTab().position; position != nil {
		return position(m)
	}
	return 0, 0
}

// hiddenPanels is how many panels of the active view the terminal is too small to show.
func (m Model) hiddenPanels() int {
	if hidden := m.activeTab().hidden; hidden != nil {
		return hidden(m)
	}
	return 0
}
