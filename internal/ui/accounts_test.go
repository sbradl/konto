package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
)

func accountsLedger() domain.Ledger {
	return domain.Ledger{Accounts: []domain.Account{
		{Name: "assets:checking", Source: domain.SourceImport},
		{Name: "assets:savings", Source: domain.SourceImport},
		{Name: "assets:spare"},
	}}
}

func pressKey(m Model, code rune) Model {
	updated, _ := m.Update(tea.KeyPressMsg{Code: code})
	return updated.(Model)
}

func TestDashboardShowsTheAccountsPanel(t *testing.T) {
	content := plain(started(t, fakeLoader{ledger: accountsLedger()}).View().Content)

	for _, want := range []string{"ACCOUNTS", "assets:checking", "assets:savings", "assets:spare"} {
		if !strings.Contains(content, want) {
			t.Errorf("content lacks %q:\n%s", want, content)
		}
	}
}

func TestOtherViewsDoNotShowTheAccountsPanel(t *testing.T) {
	model, _ := press(started(t, fakeLoader{ledger: accountsLedger()}), "2")

	if content := plain(model.View().Content); strings.Contains(content, "ACCOUNTS") {
		t.Errorf("the import view shows the accounts panel:\n%s", content)
	}
}

func TestStatusBarShowsTheCursorPositionOnTheDashboardOnly(t *testing.T) {
	model := started(t, fakeLoader{ledger: accountsLedger()})

	if got := lastLine(model); !strings.Contains(got, "1/3") {
		t.Errorf("status bar = %q, want 1/3", got)
	}
	model = pressKey(model, tea.KeyDown)
	if got := lastLine(model); !strings.Contains(got, "2/3") {
		t.Errorf("status bar = %q, want 2/3 after ↓", got)
	}
	other, _ := press(model, "3")
	if got := lastLine(other); strings.Contains(got, "/3") {
		t.Errorf("status bar = %q, want no position outside the dashboard", got)
	}
}

func TestStatusBarShowsNoPositionWithoutAccounts(t *testing.T) {
	if got := lastLine(started(t, fakeLoader{})); strings.Contains(got, "/") {
		t.Errorf("status bar = %q, want no position", got)
	}
}

func TestEndAndHomeMoveTheCursor(t *testing.T) {
	model := pressKey(started(t, fakeLoader{ledger: accountsLedger()}), tea.KeyEnd)
	if got := lastLine(model); !strings.Contains(got, "3/3") {
		t.Errorf("status bar = %q, want 3/3 after End", got)
	}
	if got := lastLine(pressKey(model, tea.KeyHome)); !strings.Contains(got, "1/3") {
		t.Errorf("status bar = %q, want 1/3 after Home", got)
	}
}

func TestCursorKeysAreIgnoredWhilePopupIsOpen(t *testing.T) {
	model, _ := press(started(t, fakeLoader{ledger: accountsLedger()}), "?")

	if got := lastLine(pressKey(model, tea.KeyDown)); !strings.Contains(got, "1/3") {
		t.Errorf("status bar = %q, want the cursor unmoved", got)
	}
}

func TestStatusBarCountsUnclassifiedAccountsAsWarnings(t *testing.T) {
	if got := lastLine(started(t, fakeLoader{ledger: accountsLedger()})); !strings.Contains(got, "● 1 warn") {
		t.Errorf("status bar = %q, want ● 1 warn", got)
	}
}

func TestStatusBarShowsNoWarningForAClassifiedLedger(t *testing.T) {
	if got := lastLine(started(t, fakeLoader{})); strings.Contains(got, "●") {
		t.Errorf("status bar = %q, want no indicator", got)
	}
}

func TestWarningCountIsAmber(t *testing.T) {
	raw := started(t, fakeLoader{ledger: accountsLedger()}).View().Content
	lines := strings.Split(raw, "\n")

	if last := lines[len(lines)-1]; !strings.Contains(last, "\x1b[33;100m● 1 warn") {
		t.Errorf("warning is not amber on the status bar background: %q", last)
	}
}

func TestChecksPanelListsEveryUnclassifiedAccount(t *testing.T) {
	model, _ := press(started(t, fakeLoader{ledger: accountsLedger()}), "!")

	content := plain(model.View().Content)
	if !strings.Contains(content, "WARN  unclassified  assets:spare has no konto:source tag") {
		t.Errorf("checks panel lacks the unclassified row:\n%s", content)
	}
	if strings.Contains(content, "no checks have run yet") {
		t.Errorf("the placeholder must give way to the rows:\n%s", content)
	}
}

func TestStatusBarSpansTheWidthWithAWarning(t *testing.T) {
	model := started(t, fakeLoader{ledger: accountsLedger()})

	lines := strings.Split(model.View().Content, "\n")
	if got := lipgloss.Width(lines[len(lines)-1]); got != 120 {
		t.Errorf("status bar is %d wide, want 120", got)
	}
}

func TestAccountsPanelFillsTheContentArea(t *testing.T) {
	model := started(t, fakeLoader{ledger: accountsLedger()})

	lines := screenLines(model)
	if !strings.HasPrefix(lines[2], "╭─ ACCOUNTS ") || !strings.HasPrefix(lines[len(lines)-2], "╰") {
		t.Errorf("the panel should run from the rule to the status bar:\n%s", strings.Join(lines, "\n"))
	}
}
