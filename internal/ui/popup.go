package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

// popupKind is which overlay, if any, is open. Only one is ever visible (spec §6.5).
type popupKind int

const (
	popupNone popupKind = iota
	popupKeys
	popupChecks
)

// Popup geometry: the keys popup reflows to at most maxKeyColumns columns of at least minKeyColumnWidth.
const (
	maxKeyColumns     = 4
	minKeyColumnWidth = 25
	keyWidth          = 7 // widest key label ("Enter") plus a gap
)

// keyBinding is one key and what it does, as the status bar and keys popup show it.
type keyBinding struct{ key, action string }

func globalKeys() []keyBinding {
	return []keyBinding{
		{"1–5", "switch view"}, {"?", "keys"}, {"!", "checks"}, {"r", "refresh"}, {"q", "quit"}, {"Esc", "close popup"},
	}
}

// viewKeys are the keys each view answers to, by tab index.
func viewKeys() [tabCount][]keyBinding {
	return [tabCount][]keyBinding{
		{{"c", "close"}, {"o", "reopen"}},
		{{"n", "new file"}, {"Enter", "categorise"}, {"w", "write"}},
		{{"n", "new"}, {"e", "edit"}, {"d", "delete"}},
		nil,
		{{"p", "period"}},
	}
}

// onPopupKey handles a key while a popup is open: the opening keys toggle, Esc
// closes, the checks panel has its own cursor keys, and everything else is swallowed
// so view keys never leak through.
func (m Model) onPopupKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.popup = popupNone
	case "?":
		m = m.toggle(popupKeys)
	case "!":
		m = m.toggle(popupChecks)
	default:
		if m.popup == popupChecks {
			return m.onChecksKey(key)
		}
	}
	return m, nil
}

// toggle opens kind, or closes it when it is already the open popup.
func (m Model) toggle(kind popupKind) Model {
	if m.popup == kind {
		m.popup = popupNone
	} else {
		m.popup = kind
	}
	return m
}

// withPopup draws the open popup over the composed screen.
func (m Model) withPopup(base string) string {
	if m.popup == popupNone {
		return base
	}
	return layout.Overlay(base, m.popupBox(), m.height)
}

// popupBox is the open popup sized to the screen: full width, content tall, at most half the screen.
func (m Model) popupBox() string {
	title, body := "keys", m.keysBody()
	if m.popup == popupChecks {
		title, body = "checks", m.checksBody()
	}
	rows := append([]string{m.titleRule(title)}, body...)
	rows = rows[:layout.PopupHeight(len(rows), m.height)]
	for i, row := range rows[1:] {
		rows[i+1] = fit(row, m.width)
	}
	return strings.Join(rows, "\n")
}

// titleRule is the popup's top edge with the title inset two cells.
func (m Model) titleRule(title string) string {
	rule := "─ " + title + " "
	return theme.Dim.Render(rule + strings.Repeat("─", max(0, m.width-lipgloss.Width(rule))))
}

// keysColumns is how many columns the keys popup uses at the given width.
func keysColumns(width int) int { return layout.Columns(width, minKeyColumnWidth, maxKeyColumns) }

// keysBody lays the global keys, then the active view's, out column by column.
func (m Model) keysBody() []string {
	bindings := append(globalKeys(), viewKeys()[m.active]...)
	columns := keysColumns(m.width)
	rows := (len(bindings) + columns - 1) / columns
	keyCell := lipgloss.NewStyle().Width(keyWidth)
	columnCell := lipgloss.NewStyle().Width(m.width / columns)
	body := make([]string, rows)
	for i, binding := range bindings {
		cell := " " + keyCell.Render(binding.key) + theme.Dim.Render(binding.action)
		body[i%rows] += columnCell.Render(cell)
	}
	return body
}

// fit cuts row off at width or pads it with blanks to width.
func fit(row string, width int) string {
	row = lipgloss.NewStyle().MaxWidth(width).Render(row)
	return row + strings.Repeat(" ", width-lipgloss.Width(row))
}
