package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/ui/dashboard"
	"github.com/sbradl/konto/internal/ui/imports"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/scheduled"
)

// tabCount is the number of v1 views.
const tabCount = 3

// tab is one view as the root drives it; adding a view is adding a tab. A view without hidden panels or a
// dialog of its own leaves the field nil.
type tab struct {
	name     string
	keys     []layout.Binding                     // what the status bar and the keys popup list
	resize   func(Model, int, int) Model          // gives the view its content area
	view     func(Model) string                   // draws the view
	key      func(Model, string) (Model, tea.Cmd) // hands the view a key and runs what it asks for
	position func(Model) (row, total int)         // the cursor row and the row count
	hidden   func(Model) int                      // how many panels the terminal is too small to show
	capture  *capture                             // the view's dialog that can take the keyboard
}

// capture is a dialog of a view that, while active, takes every key as text.
type capture struct {
	active func(Model) bool                     // whether the dialog has the keyboard
	key    func(Model, string) (Model, tea.Cmd) // hands the dialog a key
}

// tabs are the three views, in tab order; number key i+1 selects tab i.
func tabs() [tabCount]tab {
	return [tabCount]tab{dashboardTab(), importingTab(), schedulingTab()}
}

// dashboardTab is the Dashboard: Tab moves its panel focus, the focused panel scrolls.
func dashboardTab() tab {
	return tab{
		name: "dash", keys: dashboard.Bindings(),
		resize: func(m Model, w, h int) Model {
			m.dash = m.dash.Resize(w, h)

			return m
		},
		view:     func(m Model) string { return m.dash.View() },
		key:      Model.onDashboardKey,
		position: func(m Model) (row, total int) { return m.dash.Position() },
		hidden:   func(m Model) int { return m.dash.HiddenPanels() },
	}
}

// importingTab is the CSV import, whose dialogs take the keyboard.
func importingTab() tab {
	return tab{
		name: "import", keys: imports.Bindings(),
		resize: func(m Model, w, h int) Model {
			m.imports = m.imports.Resize(w, h)

			return m
		},
		view:     func(m Model) string { return m.imports.View() },
		key:      Model.onImportKey,
		position: func(m Model) (row, total int) { return m.imports.Position() },
		capture:  &capture{active: Model.importDialogOpen, key: Model.onImportKey},
	}
}

// schedulingTab is the Scheduled view, whose rule form takes the keyboard.
func schedulingTab() tab {
	return tab{
		name: "sched", keys: scheduled.Bindings(),
		resize: func(m Model, w, h int) Model {
			m.scheduled = m.scheduled.Resize(w, h)

			return m
		},
		view:     func(m Model) string { return m.scheduled.View() },
		key:      Model.onScheduledKey,
		position: func(m Model) (row, total int) { return m.scheduled.Position() },
		capture:  &capture{active: func(m Model) bool { return m.scheduled.Capturing() }, key: Model.onFormKey},
	}
}

// activeTab is the view on screen.
func (m Model) activeTab() tab { return tabs()[m.active] }

// onActiveViewKey hands a key to the view on screen.
func (m Model) onActiveViewKey(key string) (Model, tea.Cmd) { return m.activeTab().key(m, key) }

// onCapturedKey hands a key to the dialog that has the keyboard: the Import dialog or the Scheduled form.
func (m Model) onCapturedKey(key string) (Model, tea.Cmd) { return m.activeTab().capture.key(m, key) }

// position is the active view's cursor row and row count.
func (m Model) position() (row, total int) { return m.activeTab().position(m) }

// hiddenPanels is how many panels of the active view the terminal is too small to show.
func (m Model) hiddenPanels() int {
	if hidden := m.activeTab().hidden; hidden != nil {
		return hidden(m)
	}

	return 0
}
