// Package layout holds the responsive layout helpers and the overlay compositor (spec §6.5).
package layout

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/ui/theme"
)

// popupZ lifts a popup above the base layer.
const popupZ = 10

// Overlay draws popup over base, full width and bottom-anchored just above the
// last row (the status bar). A popup taller than the screen is cut off at the bottom.
func Overlay(base, popup string, height int) string {
	return overlayAbove(base, popup, height, 1)
}

// OverlayFlush draws popup over base like Overlay, but down to the very last row of base: for a view that
// draws inside the screen and so has no status bar of its own below it.
func OverlayFlush(base, popup string, height int) string {
	return overlayAbove(base, popup, height, 0)
}

// overlayAbove puts popup at the bottom of base, leaving gap rows below it.
func overlayAbove(base, popup string, height, gap int) string {
	popupHeight := min(lipgloss.Height(popup), height)
	top := max(0, height-popupHeight-gap)
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

// Panel draws body in a rounded border w×h cells in all, the title inset two cells.
// Body lines wider than the inside are cut off; missing lines are blank.
func Panel(title string, w, h int, body string) string {
	inner := max(0, w-2)
	rows := strings.Split(body, "\n")
	lines := make([]string, 0, h)
	top := "╭─ " + theme.PanelTitle.Render(title) + " "
	lines = append(lines, top+strings.Repeat("─", max(0, w-lipgloss.Width(top)-1))+"╮")
	for i := range max(0, h-2) {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, "│"+Fit(row, inner)+"│")
	}
	lines = append(lines, "╰"+strings.Repeat("─", inner)+"╯")
	return strings.Join(lines, "\n")
}
