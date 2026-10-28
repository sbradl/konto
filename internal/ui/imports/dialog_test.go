package imports

import (
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
)

func categorisable() csvimport.Staged {
	s := staged()
	s.Zero = eur(0)
	s.Rows[0].Payee, s.Rows[1].Payee, s.Rows[2].Payee = "REWE Markt", "Bäckerei Schmidt", "Kiosk"
	s.Rows[1].Purpose = "Brötchen"
	s.Rows[2].Purpose = "Zeitung"
	s.Rules = []csvimport.Rule{{
		ID: "2", Category: "expenses:groceries", Alternatives: [][]csvimport.Condition{{{Field: "payee", Pattern: "REWE"}}},
	}}
	s.Known = []domain.Account{
		{Name: "assets:checking", Source: domain.SourceImport},
		{Name: "assets:savings", Source: domain.SourceImport},
		{Name: "expenses:groceries"},
		{Name: "expenses:books"},
		{Name: "income:salary"},
	}
	return s
}

func categorising() Model { return New("/dl").Resize(180, 50).WithStaged(categorisable()) }

// keys applies keys one after the other.
func keys(m Model, names ...string) Model {
	for _, name := range names {
		m, _ = m.Key(name)
	}
	return m
}

// typed applies each character of text as a key.
func typed(m Model, text string) Model {
	for _, r := range text {
		m, _ = m.Key(string(r))
	}
	return m
}

func screen(m Model) string { return plain(m.View()) }

// onRow moves the cursor to the row n (0-based) and opens the categorise popup.
func onRow(m Model, n int) Model {
	for range n {
		m, _ = m.Key("down")
	}
	m, _ = m.Key("enter")
	return m
}

func TestEnterOffersNewRuleAddToRuleAndOnceOnAQuestionRowAndOnlyOnceOnAMatchedRow(t *testing.T) {
	question := screen(onRow(categorising(), 1))
	for _, want := range []string{"─ categorise ", "new rule", "add to rule", "once"} {
		if !strings.Contains(question, want) {
			t.Errorf("popup lacks %q:\n%s", want, question)
		}
	}
	matched := screen(onRow(categorising(), 0))
	if strings.Contains(matched, "new rule") || !strings.Contains(matched, "once") {
		t.Errorf("a matched row offers only once:\n%s", matched)
	}
	if !strings.Contains(matched, "╭─ ROWS ") {
		t.Error("the table is hidden by the popup")
	}
}

func TestEnterWithoutRowsOpensNothing(t *testing.T) {
	empty := csvimport.Staged{File: "x.csv"}
	if m, _ := New("/dl").Resize(180, 40).WithStaged(empty).Key("enter"); m.Capturing() {
		t.Error("a dialog opened over no rows")
	}
}

func TestEscClosesTheDialogAndChangesNothing(t *testing.T) {
	m := onRow(categorising(), 1)
	if !m.Capturing() {
		t.Fatal("the popup is not capturing keys")
	}
	m = keys(m, "esc")
	if m.Capturing() || strings.Contains(screen(m), "─ categorise ") {
		t.Error("Esc did not close the popup")
	}
	if got := m.staged.Rows[1].Target; got != "" {
		t.Errorf("target = %q", got)
	}
}

func TestChoicesAreMovedWithTheArrowKeysAndClamped(t *testing.T) {
	m := keys(onRow(categorising(), 1), "down", "down", "down", "up")
	if m.dlg.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.dlg.cursor)
	}
	if m = keys(m, "home"); m.dlg.cursor != 0 {
		t.Errorf("home: cursor = %d", m.dlg.cursor)
	}
	if m = keys(m, "end"); m.dlg.cursor != 2 {
		t.Errorf("end: cursor = %d", m.dlg.cursor)
	}
}

func TestOnceOpensThePickerAndPickingCategorisesOnlyThatRow(t *testing.T) {
	m := keys(onRow(categorising(), 2), "down", "down", "enter")
	if !strings.Contains(screen(m), "─ category ") {
		t.Fatalf("no picker:\n%s", screen(m))
	}
	m = typed(m, "expenses:books")
	m = keys(m, "enter")
	row := m.staged.Rows[2]
	if m.Capturing() || row.Target != "expenses:books" || row.RuleID != "" || !row.Once {
		t.Errorf("row = %+v, capturing %v", row, m.Capturing())
	}
	if !strings.Contains(screen(m), "1 need a category") {
		t.Errorf("summary:\n%s", screen(m))
	}
}

func TestThePickerFiltersAndOffersCreateWhenNothingMatches(t *testing.T) {
	m := keys(onRow(categorising(), 2), "down", "down", "enter")
	if view := screen(m); !strings.Contains(view, "expenses:groceries") || !strings.Contains(view, "+ create") ||
		!strings.Contains(view, "assets:savings  (transfer)") {
		t.Errorf("empty filter shows everything and + create:\n%s", view)
	}
	m = typed(m, "gro")
	if view := screen(m); !strings.Contains(view, "expenses:groceries") || strings.Contains(view, "expenses:books") ||
		strings.Contains(view, "+ create") {
		t.Errorf("filtered:\n%s", view)
	}
	m = keys(m, "backspace", "backspace", "backspace")
	m = typed(m, "coffee")
	if view := screen(m); !strings.Contains(view, "+ create expenses:coffee") {
		t.Errorf("no suggestion:\n%s", view)
	}
}

func TestCreatingACategoryStagesItAndPostsTheRowToIt(t *testing.T) {
	m := typed(keys(onRow(categorising(), 2), "down", "down", "enter"), "coffee")
	m = keys(m, "enter")
	if m.staged.Rows[2].Target != "expenses:coffee" || len(m.staged.NewAccounts) != 1 {
		t.Errorf("row %+v, new accounts %v", m.staged.Rows[2], m.staged.NewAccounts)
	}
	if !strings.Contains(screen(m), "account expenses:coffee") {
		t.Errorf("the staged line does not list the directive:\n%s", screen(m))
	}
}

func TestACategoryWhoseLeafExistsIsRefusedAndTheDialogStaysOpen(t *testing.T) {
	m := typed(keys(onRow(categorising(), 2), "down", "down", "enter"), "income:groceries")
	m = keys(m, "enter")
	if view := screen(m); !strings.Contains(view, "refused: ") || !strings.Contains(view, "groceries") {
		t.Errorf("no refusal:\n%s", view)
	}
	if m.staged.Rows[2].Target != "" {
		t.Error("the row was categorised")
	}
	if m = typed(m, "x"); strings.Contains(screen(m), "refused: ") {
		t.Error("typing did not clear the refusal")
	}
}

func TestTheCategoryListCursorMovesAndSelectsACandidate(t *testing.T) {
	m := keys(onRow(categorising(), 2), "down", "down", "enter", "down", "enter")
	if got := m.staged.Rows[2].Target; got != "expenses:groceries" {
		t.Errorf("target = %q, want the second candidate", got)
	}
}

func newRule(t *testing.T) Model {
	t.Helper()
	m := keys(onRow(categorising(), 2), "enter")
	if !strings.Contains(screen(m), "─ new rule ") {
		t.Fatalf("no form:\n%s", screen(m))
	}
	return m
}

func TestTheNewRuleFormIsPrefilledAndCountsTheRowsItMatches(t *testing.T) {
	view := screen(newRule(t))
	for _, want := range []string{"pattern", "Kiosk", "matches 1 row in this file", "if %payee Kiosk", "comment2 konto-rule:3"} {
		if !strings.Contains(view, want) {
			t.Errorf("form lacks %q:\n%s", want, view)
		}
	}
	m := keys(newRule(t), "ctrl+u")
	if !strings.Contains(screen(m), "enter a pattern") {
		t.Errorf("an empty pattern is not asked for:\n%s", screen(m))
	}
	m = typed(m, "e")
	if view := screen(m); !strings.Contains(view, "would take 1 row from rule 2") || !strings.Contains(view, "saving is blocked") {
		t.Errorf("no overlap warning:\n%s", view)
	}
}

func TestTheFormTakesAPurposeAndACategoryThenStagesTheRule(t *testing.T) {
	m := keys(newRule(t), "tab")
	m = typed(m, "Zeit")
	if !strings.Contains(screen(m), "& %purpose Zeit") {
		t.Errorf("purpose condition missing:\n%s", screen(m))
	}
	m = keys(m, "tab")
	if !strings.Contains(screen(m), "─ category ") {
		t.Fatalf("Tab onto the category did not open the picker:\n%s", screen(m))
	}
	m = keys(typed(m, "expenses:books"), "enter")
	if !strings.Contains(screen(m), "expenses:books") || !strings.Contains(screen(m), "─ new rule ") {
		t.Fatalf("the form did not come back with the category:\n%s", screen(m))
	}
	m = keys(m, "enter")
	row := m.staged.Rows[2]
	if m.Capturing() || row.RuleID != "3" || row.Target != "expenses:books" || !strings.Contains(screen(m), "rule 3") {
		t.Errorf("row %+v, capturing %v:\n%s", row, m.Capturing(), screen(m))
	}
}

func TestFocusMovesBothWaysAndWraps(t *testing.T) {
	m := keys(newRule(t), "shift+tab")
	if !strings.Contains(screen(m), "─ category ") {
		t.Errorf("Shift+Tab from the pattern should wrap to the category:\n%s", screen(m))
	}
	m = keys(m, "esc")
	if m.dlg.kind != dialogRuleForm || m.parent.kind != dialogNone {
		t.Errorf("Esc in the picker should go back to the form: %+v", m.dlg)
	}
	m = keys(m, "down", "up", "up")
	if !strings.Contains(screen(m), "─ category ") {
		t.Errorf("up from the pattern wraps to the category:\n%s", screen(m))
	}
}

func TestConfirmingWithoutAPatternOrCategoryIsRefused(t *testing.T) {
	m := keys(newRule(t), "enter")
	if !strings.Contains(screen(m), "refused: choose a category") || len(m.staged.StagedRules()) != 0 {
		t.Errorf("no refusal:\n%s", screen(m))
	}
	m = keys(m, "ctrl+u", "enter")
	if !strings.Contains(screen(m), "the pattern is required") {
		t.Errorf("no pattern refusal:\n%s", screen(m))
	}
	if m = keys(m, "backspace"); strings.Contains(screen(m), "refused:") {
		t.Error("editing did not clear the refusal")
	}
}

func TestAnOverlappingRuleIsRefusedNamingTheOtherRule(t *testing.T) {
	m := keys(newRule(t), "ctrl+u")
	m = typed(m, "REWE")
	m = keys(typed(keys(m, "tab", "tab"), "expenses:books"), "enter", "enter")
	if view := screen(m); !strings.Contains(view, "refused: rule 2") {
		t.Errorf("no refusal:\n%s", view)
	}
}

func TestAddToRuleListsTheRulesFiltersThemAndWidensTheOneChosen(t *testing.T) {
	m := keys(onRow(categorising(), 2), "down", "enter")
	view := screen(m)
	if !strings.Contains(view, "─ add to rule ") || !strings.Contains(view, "#2   expenses:groceries") {
		t.Fatalf("no rule list:\n%s", view)
	}
	if m2 := typed(m, "zzz"); len(m2.ruleChoices()) != 0 {
		t.Errorf("the filter kept %v", m2.ruleChoices())
	}
	if m2 := keys(typed(m, "zzz"), "enter"); m2.dlg.kind != dialogRuleList {
		t.Error("Enter on an empty list moved on")
	}
	m = keys(typed(m, "groc"), "enter")
	view = screen(m)
	if !strings.Contains(view, "─ widen rule 2 ") || !strings.Contains(view, "%payee Kiosk") {
		t.Fatalf("no widen form:\n%s", view)
	}
	m = keys(m, "enter")
	row := m.staged.Rows[2]
	if m.Capturing() || row.RuleID != "2" || row.Target != "expenses:groceries" {
		t.Errorf("row = %+v", row)
	}
}

func TestWideningIntoAnotherRulesRowsIsRefused(t *testing.T) {
	s := categorisable()
	s.Rules = append(s.Rules, csvimport.Rule{
		ID: "4", Category: "expenses:books", Alternatives: [][]csvimport.Condition{{{Field: "payee", Pattern: "Thalia"}}},
	})
	m := New("/dl").Resize(180, 50).WithStaged(s)
	m = keys(onRow(m, 2), "down", "enter", "down", "enter", "ctrl+u")
	m = keys(typed(m, "REWE"), "enter")
	if view := screen(m); !strings.Contains(view, "refused: rule 2") {
		t.Errorf("no refusal:\n%s", view)
	}
	if m = typed(m, "x"); strings.Contains(screen(m), "refused:") {
		t.Error("typing did not clear the refusal")
	}
}

func TestDialogsLeaveTheTableAloneWhenTheyListManyEntries(t *testing.T) {
	s := categorisable()
	for i := range 40 {
		s.Known = append(s.Known, domain.Account{Name: "expenses:category" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	m := New("/dl").Resize(120, 30).WithStaged(s)
	m = keys(onRow(m, 2), "down", "down", "enter", "end")
	view := screen(m)
	if !strings.Contains(view, "+ create") || !strings.Contains(view, "─ category ") {
		t.Errorf("the cursor row is not in view:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) != 30 {
		t.Errorf("view is %d lines, want 30", len(lines))
	}
}

func TestAShortTerminalKeepsTheRuleListCursorAndAFormInView(t *testing.T) {
	s := categorisable()
	for i := range 20 {
		s.Rules = append(s.Rules, csvimport.Rule{
			ID: string(rune('a' + i)), Category: "expenses:x", Alternatives: [][]csvimport.Condition{{{Field: "payee", Pattern: "x"}}},
		})
	}
	m := New("/dl").Resize(120, 16).WithStaged(s)
	m = keys(onRow(m, 2), "down", "enter", "end")
	if view := screen(m); !strings.Contains(view, "#t ") || len(strings.Split(view, "\n")) != 16 {
		t.Errorf("the last rule is not in view:\n%s", view)
	}
	form := keys(onRow(New("/dl").Resize(120, 12).WithStaged(categorisable()), 2), "enter")
	if view := screen(form); !strings.Contains(view, "─ new rule ") || len(strings.Split(view, "\n")) != 12 {
		t.Errorf("the form:\n%s", view)
	}
}

func TestATransferRowIsMarked(t *testing.T) {
	s := categorisable()
	s.Rows[1].Target = "assets:transfers"
	view := screen(New("/dl").Resize(180, 40).WithStaged(s))
	if !strings.Contains(view, "assets:transfers ⇄") {
		t.Errorf("no transfer mark:\n%s", view)
	}
}
