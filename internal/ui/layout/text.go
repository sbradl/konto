package layout

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/ui/theme"
)

// Fit cuts text off at width or pads it with blanks to width. With no room at all the text is left whole,
// for the edge of the panel it is drawn in to cut: a label cut there still says what its figure is.
func Fit(text string, width int) string {
	if width < 1 { // lipgloss reads a max width of zero as no limit anyway; a negative one must not pad
		return text
	}
	cut := lipgloss.NewStyle().MaxWidth(width).Render(text)
	return cut + strings.Repeat(" ", width-lipgloss.Width(cut))
}

// AlignRight pads text on the left to width; text wider than that is left whole.
func AlignRight(text string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(text))) + text
}

// TitleRule is a dim top edge width cells wide with the title inset two cells, for a popup, dialog or form.
func TitleRule(title string, width int) string {
	rule := "─ " + title + " "
	return theme.Dim.Render(rule + strings.Repeat("─", max(0, width-lipgloss.Width(rule))))
}

// Plural is the count with the noun, an s added unless the count is one.
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
