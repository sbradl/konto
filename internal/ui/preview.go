package ui

import (
	"strings"

	"github.com/sbradl/konto/internal/ui/layout"
)

// withPreview draws what the open prompt would write over the screen, bottom-anchored like a popup; the
// arrow keys scroll it when it does not fit.
func (m Model) withPreview(base string) string {
	if m.prompt == nil || len(m.prompt.preview) == 0 {
		return base
	}
	lines, capacity := m.prompt.preview, m.previewCapacity()
	first := min(m.prompt.top, m.lastPreviewTop()) // a resize can leave less to scroll
	rows := []string{layout.TitleRule("write", m.width)}
	for _, line := range lines[first : first+capacity] {
		rows = append(rows, layout.Fit(" "+line, m.width))
	}
	return layout.Overlay(base, strings.Join(rows, "\n"), m.height)
}

// lastPreviewTop is the last line the preview can start at: from there on its end is on screen.
func (m Model) lastPreviewTop() int { return max(0, len(m.prompt.preview)-m.previewCapacity()) }

// previewCapacity is how many lines of the open prompt's preview fit under its title in half the screen.
func (m Model) previewCapacity() int {
	return max(1, layout.PopupHeight(len(m.prompt.preview)+1, m.height)-1)
}
