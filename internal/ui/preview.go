package ui

import "github.com/sbradl/konto/internal/ui/layout"

// withPreview draws what the open prompt would write over the screen, bottom-anchored like a popup; the
// cursor keys scroll it when it does not fit.
func (m Model) withPreview(base string) string {
	if m.prompt == nil || len(m.prompt.preview) == 0 {
		return base
	}
	shown := layout.Cursor{Offset: min(m.prompt.scroll.Offset, m.lastPreviewTop())} // a resize can leave less to scroll
	lines := layout.Visible(shown, m.prompt.preview, m.previewCapacity())
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		body = append(body, " "+line)
	}

	return layout.Overlay(base, layout.Popup("write", body, m.width), m.height)
}

// lastPreviewTop is the last line the preview can start at: from there on its end is on screen.
func (m Model) lastPreviewTop() int { return max(0, len(m.prompt.preview)-m.previewCapacity()) }

// previewCapacity is how many lines of the open prompt's preview fit under its title in half the screen.
func (m Model) previewCapacity() int { return layout.PopupCapacity(len(m.prompt.preview), m.height) }
