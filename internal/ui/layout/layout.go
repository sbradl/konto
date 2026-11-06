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

// PopupCapacity is how many lines of a body n lines long a popup shows under its title rule on a screen h rows
// tall: as many as fit in its height, at least one.
func PopupCapacity(n, h int) int {
	return max(1, PopupHeight(n+1, h)-1)
}

// Popup is a popup width cells wide: the title rule, then each line of body fitted to the width. The caller cuts
// body to what is shown.
func Popup(title string, body []string, width int) string {
	rows := make([]string, 0, 1+len(body))
	rows = append(rows, TitleRule(title, width))
	for _, line := range body {
		rows = append(rows, Fit(line, width))
	}

	return strings.Join(rows, "\n")
}

// Columns is how many columns of at least minWidth fit in width, between 1 and maxCols.
func Columns(width, minWidth, maxCols int) int {
	return max(1, min(maxCols, width/minWidth))
}

// PanelChrome is the border rows or columns around a panel body.
const PanelChrome = 2

// Panel draws body in a rounded border w×h cells in all, the title inset two cells.
// Body lines wider than the inside are cut off; missing lines are blank.
func Panel(title string, w, h int, body string) string {
	inner := max(0, w-PanelChrome)
	lines := make([]string, 0, h)
	lines = append(lines, panelTop(title, w))
	lines = append(lines, panelRows(body, inner, h-PanelChrome)...)
	lines = append(lines, "╰"+strings.Repeat("─", inner)+"╯")

	return strings.Join(lines, "\n")
}

// panelTop is a panel's top border w cells wide, the title inset two cells.
func panelTop(title string, w int) string {
	top := "╭─ " + theme.PanelTitle.Render(title) + " "

	return top + strings.Repeat("─", max(0, w-lipgloss.Width(top)-1)) + "╮"
}

// panelRows are count rows of body between the side borders, each fitted to inner cells; missing ones are blank.
func panelRows(body string, inner, count int) []string {
	lines := strings.Split(body, "\n")
	rows := make([]string, 0, max(0, count))
	for i := range max(0, count) {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		rows = append(rows, "│"+Fit(line, inner)+"│")
	}

	return rows
}
