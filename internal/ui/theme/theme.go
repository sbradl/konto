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
	// Selected styles the active tab and the selected row: bold bright-white on the selection background.
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
)
