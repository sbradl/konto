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

// StatusBar is bright white on the selection background, not bold (spec §7.2).
func TestStatusBarIsBrightWhiteOnSelectionBackground(t *testing.T) {
	if got := StatusBar.GetForeground(); got != lipgloss.Color("15") {
		t.Errorf("foreground = %v, want ANSI 15", got)
	}

	if got := StatusBar.GetBackground(); got != lipgloss.Color("8") {
		t.Errorf("background = %v, want ANSI 8", got)
	}

	if StatusBar.GetBold() {
		t.Error("StatusBar should not be bold")
	}
}

// Negative is ANSI red and Warn is ANSI yellow, foreground only (ADR 0005).
func TestNegativeAndWarnAreForegroundOnlyAnsiColours(t *testing.T) {
	for name, tc := range map[string]struct {
		style lipgloss.Style
		want  string
	}{"Negative": {Negative, "1"}, "Warn": {Warn, "3"}, "Series": {Series, "6"}, "PanelTitle": {PanelTitle, "6"}, "Emphasis": {Emphasis, "15"}, "Directive": {Directive, "5"}} {
		if got := tc.style.GetForeground(); got != lipgloss.Color(tc.want) {
			t.Errorf("%s foreground = %v, want ANSI %s", name, got, tc.want)
		}

		if bg := tc.style.GetBackground(); bg != (lipgloss.NoColor{}) {
			t.Errorf("%s background = %v, want none", name, bg)
		}
	}
}

func TestStatusIndicatorsAreStatusBarWithAColouredText(t *testing.T) {
	for name, tc := range map[string]struct {
		style lipgloss.Style
		want  string
	}{"StatusOk": {StatusOk, "2"}, "StatusWarn": {StatusWarn, "3"}, "StatusErr": {StatusErr, "1"}} {
		if got := tc.style.GetForeground(); got != lipgloss.Color(tc.want) {
			t.Errorf("%s foreground = %v, want ANSI %s", name, got, tc.want)
		}

		if got := tc.style.GetBackground(); got != lipgloss.Color("8") {
			t.Errorf("%s background = %v, want ANSI 8", name, got)
		}
	}
}

func TestOkIsAForegroundOnlyGreen(t *testing.T) {
	if got := Ok.GetForeground(); got != lipgloss.Color("2") {
		t.Errorf("foreground = %v, want ANSI 2", got)
	}

	if bg := Ok.GetBackground(); bg != (lipgloss.NoColor{}) {
		t.Errorf("background = %v, want none", bg)
	}
}

func TestLineColoursFollowGreenBlueMagentaCyanYellowRedAndWrap(t *testing.T) {
	want := []string{"2", "4", "5", "6", "3", "1", "2", "4"}
	for rank, colour := range want {
		if got := Line(rank).GetForeground(); got != lipgloss.Color(colour) {
			t.Errorf("rank %d is coloured %v, want ANSI %s", rank, got, colour)
		}
	}
}
