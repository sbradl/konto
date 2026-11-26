package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/ui/uitest"
)

func pressEsc(m Model) Model {
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	return updated.(Model)
}

func screenLines(m Model) []string { return strings.Split(uitest.Plain(m.View().Content), "\n") }

func TestQuestionMarkOpensTheKeysPopupListingTheGlobalKeys(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")

	content := uitest.Plain(model.View().Content)
	for _, want := range []string{"keys", "1–3", "?", "!", "r", "q"} {
		if !strings.Contains(content, want) {
			t.Errorf("content lacks %q:\n%s", want, content)
		}
	}
}

func TestEscClosesThePopup(t *testing.T) {
	for _, open := range []string{"?", "!"} {
		model, _ := press(started(t, fakeLoader{}), open)

		if got := pressEsc(model); got.popup != popupNone {
			t.Errorf("after %s and Esc, popup = %v, want none", open, got.popup)
		}
	}
}

func TestOpeningKeyTogglesThePopup(t *testing.T) {
	for _, open := range []string{"?", "!"} {
		model, _ := press(started(t, fakeLoader{}), open)
		model, _ = press(model, open)

		if model.popup != popupNone {
			t.Errorf("%s twice: popup = %v, want none", open, model.popup)
		}
	}
}

func TestOnlyOnePopupIsOpenAtATime(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")
	model, _ = press(model, "!")

	content := uitest.Plain(model.View().Content)
	if !strings.Contains(content, "checks") || strings.Contains(content, "1–3") {
		t.Errorf("want the checks panel only:\n%s", content)
	}
}

func TestKeysPopupShowsTheActiveViewsKeys(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "3")
	model, _ = press(model, "?")

	content := uitest.Plain(model.View().Content)
	for _, want := range []string{"new", "edit", "delete"} {
		if !strings.Contains(content, want) {
			t.Errorf("content lacks %q:\n%s", want, content)
		}
	}
}

func TestViewKeysAreIgnoredWhileAPopupIsOpen(t *testing.T) {
	for _, open := range []string{"?", "!"} {
		model, _ := press(started(t, fakeLoader{}), open)
		model, cmd := press(model, "3")

		if model.active != 0 || cmd != nil {
			t.Errorf("%s open: active = %d, cmd = %v, want the dash view and no command", open, model.active, cmd)
		}
	}
}

func TestQuitStillWorksWhileAPopupIsOpen(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")

	if _, cmd := press(model, "q"); cmd == nil {
		t.Error("q should quit with a popup open")
	}
}

func TestPopupOverlaysTheViewAboveTheStatusBar(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")

	lines := screenLines(model)
	if len(lines) != 40 {
		t.Fatalf("screen has %d rows, want 40", len(lines))
	}

	if !strings.Contains(lines[0], "1 dash") {
		t.Errorf("tab bar is gone: %q", lines[0])
	}

	if !strings.Contains(lines[len(lines)-1], "? keys") {
		t.Errorf("status bar is covered: %q", lines[len(lines)-1])
	}
}

func TestPopupEdgeSpansTheFullWidthAndPopupIsAtMostHalfTheScreen(t *testing.T) {
	for _, open := range []string{"?", "!"} {
		model, _ := press(resized(started(t, fakeLoader{}), 100, 30), open)

		lines := screenLines(model)
		title := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "─ ") {
				title = i
			}
		}

		if title < 0 {
			t.Fatalf("%s: no popup title row:\n%s", open, strings.Join(lines, "\n"))
		}

		if rows := len(lines) - 1 - title; rows > 15 {
			t.Errorf("%s: popup is %d rows, want at most 15", open, rows)
		}

		if got := lipgloss.Width(lines[title]); got != 100 {
			t.Errorf("%s: popup edge is %d cells wide, want 100", open, got)
		}
	}
}

func TestKeysPopupReflowsToOneColumnOnAVeryNarrowScreen(t *testing.T) {
	if got := keysColumns(30); got != 1 {
		t.Errorf("keysColumns(30) = %d, want 1", got)
	}

	if got := keysColumns(200); got != 4 {
		t.Errorf("keysColumns(200) = %d, want 4", got)
	}
}

func TestPopupKeysDoNothingOutsideTheNormalViews(t *testing.T) {
	model, _ := press(firstRunModel(t, &fakeCreator{}), "?")

	if model.popup != popupNone {
		t.Errorf("popup = %v, want none on the first-run screen", model.popup)
	}
}

// Content taller than half the screen is cut off at the cap.
func TestPopupBoxIsCappedAtHalfTheScreen(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")
	model.height = 4

	if got := strings.Count(model.popupBox(), "\n") + 1; got != 2 {
		t.Errorf("popup is %d rows, want 2 (half of 4)", got)
	}
}

func trimRows(rows []string) []string {
	trimmed := make([]string, len(rows))
	for i, row := range rows {
		trimmed[i] = strings.TrimRight(row, " ")
	}

	return trimmed
}

func plainRows(box string) []string { return strings.Split(uitest.Plain(box), "\n") }

func TestKeysBodyFillsColumnsTopToBottom(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "3")
	model, _ = press(model, "?")
	model = resized(model, 100, 30)

	want := []string{
		"─ keys " + strings.Repeat("─", 93),
		" 1–3    switch view       r      refresh           n      new               c      copy transaction",
		" ?      keys              q      quit              e      edit" + "",
		" !      checks            Esc    close popup       d      delete" + "",
	}
	if got := trimRows(plainRows(model.popupBox())); !reflect.DeepEqual(got, want) {
		t.Errorf("popup =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestKeysBodyIsOneColumnOnANarrowScreen(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "?")
	model = resized(model, 30, 40)

	rows := plainRows(model.popupBox())

	if len(rows) != 11 || rows[1] != " 1–3    switch view"+strings.Repeat(" ", 11) || rows[10] != " t      today"+strings.Repeat(" ", 17) {
		t.Errorf("popup =\n%s", strings.Join(rows, "\n"))
	}
}

func TestChecksPanelFillsTheWidth(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "!")

	want := []string{
		"─ checks " + strings.Repeat("─", 111),
		" ✓ all 12 checks pass" + strings.Repeat(" ", 99),
	}
	if got := plainRows(model.popupBox()); !reflect.DeepEqual(got, want) {
		t.Errorf("panel =\n%s", strings.Join(got, "\n"))
	}
}

// r is global: it reloads with a popup open, so a fix can be picked up from the checks panel.
func TestRRefreshesWhileAPopupIsOpen(t *testing.T) {
	for _, open := range []string{"?", "!"} {
		model, _ := press(started(t, fakeLoader{}), open)

		model, cmd := press(model, "r")

		if !model.busy() || cmd == nil || model.popup == popupNone {
			t.Errorf("%s open: refreshing = %v, cmd = %v, popup = %v; want a reload with the popup still open",
				open, model.busy(), cmd, model.popup)
		}
	}
}
