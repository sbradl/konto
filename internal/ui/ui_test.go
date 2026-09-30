package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

const journalPath = "/data/household.journal"

func sized(width int) tea.Model {
	m, _ := New(journalPath).Update(tea.WindowSizeMsg{Width: width, Height: 40})
	return m
}

func render(t *testing.T, m tea.Model) tea.View {
	t.Helper()
	return m.View()
}

// The user sees the five-tab bar on the first line of the screen.
func TestViewShowsTabBarOnFirstLine(t *testing.T) {
	first, _, _ := strings.Cut(render(t, New(journalPath)).Content, "\n")

	want := " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "
	if !strings.Contains(first, want) {
		t.Errorf("first line = %q, want it to contain %q", first, want)
	}
}

// konto takes over the whole terminal (alternate screen).
func TestViewUsesAlternateScreen(t *testing.T) {
	if !render(t, New(journalPath)).AltScreen {
		t.Error("view should request the alternate screen")
	}
}

// Pressing q or Ctrl+C quits.
func TestQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		_, cmd := New(journalPath).Update(key)

		if cmd == nil {
			t.Fatalf("%v: expected a command, got nil", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v: command should quit", key)
		}
	}
}

// Other keys and messages leave the program running.
func TestOtherInputDoesNotQuit(t *testing.T) {
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'x', Text: "x"}, tea.WindowSizeMsg{Width: 120, Height: 40}} {
		if _, cmd := New(journalPath).Update(msg); cmd != nil {
			t.Errorf("%T should not produce a command", msg)
		}
	}
}

// konto has nothing to do on start-up yet.
func TestInitReturnsNoCommand(t *testing.T) {
	if cmd := New(journalPath).Init(); cmd != nil {
		t.Error("Init should not produce a command")
	}
}

// The journal path sits at the right edge of the tab bar, opposite the tabs.
func TestTabBarShowsJournalPathAtRightEdge(t *testing.T) {
	first, _, _ := strings.Cut(render(t, sized(120)).Content, "\n")

	if !strings.HasPrefix(first, " 1 dash") {
		t.Errorf("tabs should start the line, got %q", first)
	}
	if want := journalPath + " "; !strings.HasSuffix(first, want) {
		t.Errorf("line should end with %q, got %q", want, first)
	}
	if width := lipgloss.Width(first); width != 120 {
		t.Errorf("tab bar width = %d, want 120", width)
	}
}

// On a terminal too narrow for both, the tabs and the path are still separated.
func TestTabBarKeepsTabsAndPathApartWhenNarrow(t *testing.T) {
	first, _, _ := strings.Cut(render(t, sized(40)).Content, "\n")

	if !strings.Contains(first, "5 reports "+journalPath) {
		t.Errorf("tabs and path should be separated by a space, got %q", first)
	}
}
