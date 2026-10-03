// Package theme is konto's fixed ANSI-16 palette as named styles (ADR 0005).
// View code uses these names, never raw colour indices.
package theme

import "charm.land/lipgloss/v2"

// The styles are immutable values, so package-level names are safe (spec §7.2).
//
//nolint:gochecknoglobals // immutable palette, spec §7.2 asks for named package values
var (
	// Dim styles labels, borders and secondary text.
	Dim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	// Bold styles figures that stand out.
	Bold = lipgloss.NewStyle().Bold(true)
	// Emphasis styles grand totals: bold bright white.
	Emphasis = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	// Negative styles negative amounts: ANSI red.
	Negative = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	// Ok styles a passing result: ANSI green.
	Ok = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	// Warn styles warnings and manual-entry marks: ANSI yellow, the amber of the handoff.
	Warn = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	// PanelTitle styles the title inset in a panel's top border: ANSI cyan.
	PanelTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	// Series styles a chart's data line: ANSI cyan.
	Series = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	// Selected styles the active tab and the selected row: bold bright-white on the selection background.
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	// StatusBar styles the bottom bar: bright white on the selection background.
	StatusBar = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	// StatusOk, StatusWarn and StatusErr are StatusBar with a green, amber or red foreground,
	// for the check indicator.
	StatusOk   = StatusBar.Foreground(lipgloss.Color("2"))
	StatusWarn = StatusBar.Foreground(lipgloss.Color("3"))
	StatusErr  = StatusBar.Foreground(lipgloss.Color("1"))
)
