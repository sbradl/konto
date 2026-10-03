// Package layout holds the responsive layout helpers and the overlay compositor (spec §6.5).
package layout

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// popupZ lifts a popup above the base layer.
const popupZ = 10

// Overlay draws popup over base, full width and bottom-anchored just above the
// last row (the status bar). A popup taller than the screen is cut off at the bottom.
func Overlay(base, popup string, height int) string {
	popupHeight := min(lipgloss.Height(popup), height)
	top := max(0, height-popupHeight-1)
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(base).X(0).Y(0).Z(0),
		lipgloss.NewLayer(popup).X(0).Y(top).Z(popupZ),
	)
	rendered := compositor.Render()
	return strings.Join(strings.Split(rendered, "\n")[:height], "\n")
}

// PopupHeight is the height a popup takes: its content, capped at half the screen, at least one row.
func PopupHeight(content, screen int) int {
	return max(1, min(content, screen/2))
}

// Columns is how many columns of at least minWidth fit in width, between 1 and maxCols.
func Columns(width, minWidth, maxCols int) int {
	return max(1, min(maxCols, width/minWidth))
}
