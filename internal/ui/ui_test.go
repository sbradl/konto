package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
)

const journalPath = "/data/household.journal"

type fakeLoader struct {
	ledger domain.Ledger
	err    error
}

func (f fakeLoader) Load(context.Context) (domain.Ledger, error) { return f.ledger, f.err }

func newModel() Model { return New(journalPath, fakeLoader{}) }

// loaded runs the model's start-up command and feeds the result back in.
func loaded(t *testing.T, m Model) tea.Model {
	t.Helper()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should start loading the ledger")
	}
	updated, _ := m.Update(cmd())
	return updated
}

func sized(width int) tea.Model {
	m, _ := newModel().Update(tea.WindowSizeMsg{Width: width, Height: 40})
	return m
}

func render(t *testing.T, m tea.Model) tea.View {
	t.Helper()
	return m.View()
}

// The user sees the five-tab bar on the first line of the screen.
func TestViewShowsTabBarOnFirstLine(t *testing.T) {
	first, _, _ := strings.Cut(render(t, newModel()).Content, "\n")

	want := " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "
	if !strings.Contains(first, want) {
		t.Errorf("first line = %q, want it to contain %q", first, want)
	}
}

// konto takes over the whole terminal (alternate screen).
func TestViewUsesAlternateScreen(t *testing.T) {
	if !render(t, newModel()).AltScreen {
		t.Error("view should request the alternate screen")
	}
}

// Pressing q or Ctrl+C quits.
func TestQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		_, cmd := newModel().Update(key)

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
		if _, cmd := newModel().Update(msg); cmd != nil {
			t.Errorf("%T should not produce a command", msg)
		}
	}
}

// Once the ledger has loaded, the tab bar shows how many transactions it holds.
func TestTabBarShowsTransactionCountAfterLoad(t *testing.T) {
	ledger := domain.Ledger{Transactions: make([]domain.Transaction, 54)}
	model, _ := New(journalPath, fakeLoader{ledger: ledger}).Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	first, _, _ := strings.Cut(render(t, loaded(t, model.(Model))).Content, "\n")

	if want := journalPath + " · 54 txns "; !strings.HasSuffix(first, want) {
		t.Errorf("line should end with %q, got %q", want, first)
	}
}

// A failed load is shown below the tab bar instead of being swallowed.
func TestFailedLoadIsShown(t *testing.T) {
	model := New(journalPath, fakeLoader{err: errors.New("hledger exploded")})

	_, rest, _ := strings.Cut(render(t, loaded(t, model)).Content, "\n")

	if !strings.Contains(rest, "hledger exploded") {
		t.Errorf("screen below the tab bar should show the error, got %q", rest)
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
