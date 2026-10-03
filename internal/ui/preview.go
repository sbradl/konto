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
	lines := m.prompt.preview
	capacity := max(1, layout.PopupHeight(len(lines)+1, m.height)-1)
	first := max(0, min(m.previewTop, len(lines)-capacity))
	rows := []string{m.titleRule("write")}
	for _, line := range lines[first : first+capacity] {
		rows = append(rows, fit(" "+line, m.width))
	}
	return layout.Overlay(base, strings.Join(rows, "\n"), m.height)
}
