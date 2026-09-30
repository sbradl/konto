package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// Selected is bold bright-white on the dim "sel" background (ADR 0005, spec §7.2).
func TestSelectedIsBoldBrightWhiteOnSelectionBackground(t *testing.T) {
	if got := Selected.GetForeground(); got != lipgloss.Color("15") {
		t.Errorf("foreground = %v, want ANSI 15", got)
	}
	if got := Selected.GetBackground(); got != lipgloss.Color("8") {
		t.Errorf("background = %v, want ANSI 8", got)
	}
	if !Selected.GetBold() {
		t.Error("Selected should be bold")
	}
}

// Dim is the dim foreground with no background: labels, borders, secondary text.
func TestDimIsTheDimForegroundOnly(t *testing.T) {
	if got := Dim.GetForeground(); got != lipgloss.Color("8") {
		t.Errorf("foreground = %v, want ANSI 8", got)
	}
	if bg := Dim.GetBackground(); bg != (lipgloss.NoColor{}) {
		t.Errorf("background = %v, want none", bg)
	}
}
