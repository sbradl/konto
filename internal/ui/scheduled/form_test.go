package scheduled

import (
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
)

func withAccounts() domain.Ledger {
	ledger := household()
	ledger.Commodities = []string{"EUR", "PLN"}
	ledger.Accounts = []domain.Account{
		{Name: "assets:checking", Balance: eur(1324000)},
		{Name: "assets:wallet", Balance: domain.MultiAmount{{Commodity: "PLN", Mantissa: 5000, Places: 2, Style: euro}}},
		{Name: "assets:nothing"},
		{Name: "assets:old-account", Closed: date("2026-03-01")},
		{Name: "expenses:rent"},
		{Name: "expenses:books"},
		{Name: "income:salary"},
	}
	return ledger
}

func opened() Model { return New().Resize(180, 40).WithLedger(withAccounts()).OpenNew() }

// pressed applies keys one after the other; the actions asked for are the last one's.
func pressed(m Model, keys ...string) (Model, Action) {
	var action Action
	for _, key := range keys {
		m, action = m.FormKey(key)
	}
	return m, action
}

func typed(m Model, text string) Model {
	for _, r := range text {
		m, _ = m.FormKey(string(r))
	}
	return m
}

func screen(m Model) string { return plain(m.View()) }

func TestNOpensTheFormWithItsFieldsAndFrequencyMonthly(t *testing.T) {
	m := opened()

	view := screen(m)
	for _, want := range []string{
		"─ new periodic rule ", "next due date", "frequency", "remaining payments", "description", "amount", "currency",
		"from account", "to account", "[monthly]", "[EUR]", "PLN",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("form lacks %q:\n%s", want, view)
		}
	}
	if !m.Capturing() {
		t.Error("the form does not capture the keyboard")
	}
}

func TestTheFrequencyDropdownOffersTheSevenChoices(t *testing.T) {
	view := screen(opened())

	if !strings.Contains(view, "once  weekly  biweekly  [monthly]  quarterly  half-yearly  yearly") {
		t.Errorf("choices:\n%s", view)
	}
}

func TestOnceHidesTheRemainingPaymentsField(t *testing.T) {
	m, _ := pressed(opened(), "tab", "left", "left", "left")

	if view := screen(m); !strings.Contains(view, "[once]") || strings.Contains(view, "remaining payments") {
		t.Errorf("once keeps the remaining payments:\n%s", view)
	}
	m, _ = pressed(m, "right")
	if view := screen(m); !strings.Contains(view, "[weekly]") || !strings.Contains(view, "remaining payments") {
		t.Errorf("weekly hides the remaining payments:\n%s", view)
	}
}

func TestTabAndShiftTabMoveBetweenTheFieldsAndSkipTheHiddenOne(t *testing.T) {
	m := opened()
	focusOn := func(label string) bool {
		for line := range strings.SplitSeq(screen(m), "\n") {
			if strings.Contains(line, "▸") {
				return strings.Contains(line, label)
			}
		}
		return false
	}
	if !focusOn("next due date") {
		t.Fatalf("the focus starts elsewhere:\n%s", screen(m))
	}

	m, _ = pressed(m, "tab")
	if !focusOn("frequency") {
		t.Errorf("Tab moved the focus elsewhere:\n%s", screen(m))
	}
	m, _ = pressed(m, "shift+tab")
	if !focusOn("next due date") {
		t.Errorf("Shift+Tab did not go back:\n%s", screen(m))
	}
	m, _ = pressed(m, "shift+tab")
	if !focusOn("to account") {
		t.Errorf("Shift+Tab does not wrap to the last field:\n%s", screen(m))
	}
	m, _ = pressed(m, "tab", "tab", "left", "left", "left", "tab")
	if !focusOn("description") {
		t.Errorf("a once rule's Tab does not skip the remaining payments:\n%s", screen(m))
	}
}

func TestTypingIntoATextFieldIsTextAndBackspaceAndCtrlUEditIt(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab")
	m = typed(m, "q2r!")

	if view := screen(m); !strings.Contains(view, "description         q2r!") {
		t.Errorf("the description is not q2r!:\n%s", view)
	}
	m, _ = pressed(m, "backspace")
	m = typed(m, "é")
	m, _ = pressed(m, "backspace", "backspace")
	if view := screen(m); !strings.Contains(view, "description         q2") {
		t.Errorf("backspace did not remove a character:\n%s", view)
	}
	m, _ = pressed(m, "ctrl+u")
	if view := screen(m); strings.Contains(view, "q2") {
		t.Errorf("ctrl+u did not clear:\n%s", view)
	}
}

func TestOtherKeysDoNothingInATextField(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "left", "up", "f1")

	if view := screen(m); !strings.Contains(view, "description         ▏") {
		t.Errorf("the description changed:\n%s", view)
	}
}

func TestEscClosesTheForm(t *testing.T) {
	m, action := pressed(opened(), "esc")

	if m.Capturing() || action.Save != nil || strings.Contains(screen(m), "new periodic rule") {
		t.Error("Esc did not close the form without saving")
	}
}

func TestTheCurrencyDropdownCyclesTheDeclaredCurrenciesAndStopsAtTheEnds(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab")

	m, _ = pressed(m, "right", "right")
	if view := screen(m); !strings.Contains(view, "EUR  [PLN]") {
		t.Errorf("right did not stop at the last currency:\n%s", view)
	}
	m, _ = pressed(m, "left", "left", "left")
	if view := screen(m); !strings.Contains(view, "[EUR]  PLN") {
		t.Errorf("left did not stop at the first currency:\n%s", view)
	}
}

func TestTheAccountPickerFiltersByTypingAndOffersOpenAccountsOnly(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	if view := screen(m); strings.Contains(view, "assets:old-account") || !strings.Contains(view, "assets:checking") {
		t.Errorf("the empty filter should list every open account:\n%s", view)
	}

	m = typed(m, "check")

	if view := screen(m); !strings.Contains(view, "assets:checking") || strings.Contains(view, "expenses:books") {
		t.Errorf("the filter did not narrow the list:\n%s", view)
	}
}

func TestEnterPicksTheMatchAndMovesOnAndTheCurrencyFollowsTheFromAccount(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "wallet")

	m, _ = pressed(m, "enter")

	view := screen(m)
	if !strings.Contains(view, "from account        assets:wallet") || !strings.Contains(view, "EUR  [PLN]") ||
		!strings.Contains(view, "▸to account") {
		t.Errorf("view:\n%s", view)
	}
}

func TestAFromAccountWithoutAHeldCurrencyFallsBackToTheFirstDeclared(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "right", "tab")
	m = typed(m, "nothing")

	m, _ = pressed(m, "enter")

	if view := screen(m); !strings.Contains(view, "[EUR]  PLN") {
		t.Errorf("view:\n%s", view)
	}
}

func TestEnterWithNoMatchSaysSoAndPicksNothing(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "expenses:nonexistent")

	m, _ = pressed(m, "enter")

	view := screen(m)
	if !strings.Contains(view, "no account matches expenses:nonexistent") || !strings.Contains(view, "to account          expenses:nonexistent") {
		t.Errorf("view:\n%s", view)
	}
}

func TestTheArrowKeysMoveThePickersCursorAndEnterTakesThatMatch(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m, _ = pressed(m, "down", "down", "up", "down")

	m, _ = pressed(m, "enter")

	if view := screen(m); !strings.Contains(view, "from account        assets:wallet") {
		t.Errorf("the third match is not chosen:\n%s", view)
	}
}

func TestTheCursorStopsAtTheEndsOfTheMatches(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "assets:")

	m, _ = pressed(m, "up", "down", "down", "down", "down", "down", "enter")

	if view := screen(m); !strings.Contains(view, "from account        assets:wallet") {
		t.Errorf("view:\n%s", view)
	}
}

func TestALongListOfMatchesScrollsWithTheCursor(t *testing.T) {
	ledger := withAccounts()
	for _, name := range []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"} {
		ledger.Accounts = append(ledger.Accounts, domain.Account{Name: "zz:" + name})
	}
	m := New().Resize(180, 60).WithLedger(ledger).OpenNew()
	m, _ = pressed(m, "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "zz:")

	m, _ = pressed(m, "down", "down", "down", "down", "down", "down")

	if view := screen(m); !strings.Contains(view, "zz:a7") || strings.Contains(view, "zz:a1\n") {
		t.Errorf("view:\n%s", view)
	}
}

func TestEnterWithAnAccountAlreadyChosenSavesInsteadOfPickingAgain(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "checking")
	m, _ = pressed(m, "enter", "shift+tab")

	_, action := pressed(m, "enter")

	if action.Save == nil || action.Save.Input.From != "assets:checking" {
		t.Errorf("action = %+v, want a save with the account kept", action)
	}
}

func TestEnterAndCtrlSAskTheRootToSaveWhatTheFormHolds(t *testing.T) {
	m := typed(opened(), "2026-10-15")
	m, _ = pressed(m, "tab", "tab", "tab")
	m = typed(m, "Book club")

	for _, key := range []string{"enter", "ctrl+s"} {
		_, action := pressed(m, key)

		if action.Save == nil || action.Save.Editing != nil || action.Save.Input.Date != "2026-10-15" ||
			action.Save.Input.Description != "Book club" || action.Save.Input.Frequency != periodic.Monthly {
			t.Errorf("%s: action = %+v", key, action)
		}
	}
}

func TestARefusalIsShownAndClearedByTheNextKey(t *testing.T) {
	m := opened().WithFormError("description is required")
	if view := screen(m); !strings.Contains(view, "description is required") {
		t.Fatalf("no refusal:\n%s", view)
	}

	m, _ = pressed(m, "x")

	if view := screen(m); strings.Contains(view, "description is required") {
		t.Errorf("the refusal stays:\n%s", view)
	}
	if view := screen(New().Resize(180, 40).WithLedger(withAccounts()).WithFormError("x")); strings.Contains(view, "x") && Capturing(New()) {
		t.Error("an error without a form opened one")
	}
}

func Capturing(m Model) bool { return m.Capturing() }

func TestOpenEditFillsTheFormAndTitlesItEdit(t *testing.T) {
	rule := household().Rules[1]
	input := periodic.Input{
		Date: "2026-10-01", Frequency: periodic.Quarterly, Remaining: "3", Description: "Vermieter Mueller", Amount: "900,00",
		Currency: "EUR", From: "assets:checking", To: "expenses:rent",
	}

	m := New().Resize(180, 40).WithLedger(withAccounts()).OpenEdit(input, rule)

	view := screen(m)
	for _, want := range []string{
		"─ edit periodic rule ", "2026-10-01", "[quarterly]", "remaining payments  3", "Vermieter Mueller", "900,00", "[EUR]",
		"assets:checking", "expenses:rent",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("form lacks %q:\n%s", want, view)
		}
	}
	_, action := pressed(m, "ctrl+s")
	if action.Save == nil || action.Save.Editing == nil || action.Save.Editing.Pos != rule.Pos {
		t.Errorf("action = %+v, want a save of the rule being edited", action)
	}
}

func TestWithNoDeclaredCurrenciesTheFormStillOpens(t *testing.T) {
	ledger := withAccounts()
	ledger.Commodities = nil
	m := New().Resize(180, 40).WithLedger(ledger).OpenNew()
	m, _ = pressed(m, "tab", "tab", "tab", "tab", "tab", "right", "left")

	if !m.Capturing() {
		t.Error("the form closed")
	}
	m, _ = pressed(m, "tab")
	m = typed(m, "assets:c")
	m, _ = pressed(m, "enter")
	if view := screen(m); !strings.Contains(view, "from account        assets:checking") || !strings.Contains(view, "▸to account") {
		t.Errorf("view:\n%s", view)
	}
}

func TestTheViewBehindTheFormStaysWhenThereIsNoForm(t *testing.T) {
	if view := screen(loaded()); strings.Contains(view, "periodic rule") {
		t.Errorf("a form is drawn without being opened:\n%s", view)
	}
	if m := loaded().CloseForm(); m.Capturing() {
		t.Error("closing nothing opened a form")
	}
}

func TestTypingFillsEveryTextFieldAndAToAccountCanBePicked(t *testing.T) {
	m := typed(opened(), "2026-10-15")
	m, _ = pressed(m, "tab", "tab")
	m = typed(m, "3")
	m, _ = pressed(m, "tab")
	m = typed(m, "Book club")
	m, _ = pressed(m, "tab")
	m = typed(m, "12,00")
	m, _ = pressed(m, "tab", "tab", "tab")
	m = typed(m, "books")
	m, _ = pressed(m, "enter")

	_, action := pressed(m, "ctrl+s")

	want := periodic.Input{
		Date: "2026-10-15", Frequency: periodic.Monthly, Remaining: "3", Description: "Book club", Amount: "12,00",
		Currency: "EUR", To: "expenses:books",
	}
	if action.Save == nil || action.Save.Input != want {
		t.Errorf("saved %+v, want %+v", action.Save, want)
	}
}

func TestOtherKeysLeaveADropdownAlone(t *testing.T) {
	m, _ := pressed(opened(), "tab", "x", "enter")

	if view := screen(m); !strings.Contains(view, "[monthly]") {
		t.Errorf("view:\n%s", view)
	}
}
