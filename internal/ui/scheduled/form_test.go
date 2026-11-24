package scheduled

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/uitest"
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

// saved is the save the action asks for; nil when it asks for none.
func saved(action Action) *Save {
	save, _ := action.(*Save)

	return save
}

func typed(m Model, text string) Model {
	for _, r := range text {
		m, _ = m.FormKey(string(r))
	}

	return m
}

func screen(m Model) string { return uitest.Plain(m.View()) }

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
	m, _ := pressed(opened(), "tab", "tab", "tab", "left", "right", "f1")

	if view := screen(m); !strings.Contains(view, "description         ▏") {
		t.Errorf("the description changed:\n%s", view)
	}
}

func TestEscClosesTheForm(t *testing.T) {
	m, action := pressed(opened(), "esc")

	if m.Capturing() || saved(action) != nil || strings.Contains(screen(m), "new periodic rule") {
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

func TestTheUpAndDownArrowsMoveBetweenTheFieldsAndLeaveTheDropdownsAlone(t *testing.T) {
	focus := func(m Model) string {
		for line := range strings.SplitSeq(screen(m), "\n") {
			if strings.Contains(line, "▸") {
				return line
			}
		}

		return ""
	}

	m, _ := pressed(opened(), "down")
	if line := focus(m); !strings.Contains(line, "frequency") || !strings.Contains(line, "[monthly]") {
		t.Errorf("down did not move to the frequency: %q", line)
	}

	m, _ = pressed(m, "down", "down", "up")
	if line := focus(m); !strings.Contains(line, "remaining payments") {
		t.Errorf("down twice and up once did not end on the remaining payments: %q", line)
	}

	m, _ = pressed(m, "up", "up")
	if line := focus(m); !strings.Contains(line, "next due date") {
		t.Errorf("up did not go back to the first field: %q", line)
	}

	if view := screen(m); !strings.Contains(view, "[monthly]") {
		t.Errorf("the arrows changed the frequency:\n%s", view)
	}
}

func TestPageKeysMoveThePickersCursorToItsEnds(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "assets:")

	if end, _ := pressed(m, "end", "enter"); !strings.Contains(screen(end), "from account        assets:wallet") {
		t.Errorf("end did not take the last match:\n%s", screen(end))
	}

	if home, _ := pressed(m, "pgdown", "home", "enter"); !strings.Contains(screen(home), "from account        assets:checking") {
		t.Errorf("home did not take the first match:\n%s", screen(home))
	}
}

func TestATypedNameNoAccountHasIsOfferedAsANewCategoryAndEnterTakesIt(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "gym")

	if view := screen(m); !strings.Contains(view, "+ create expenses:gym") {
		t.Errorf("no create entry:\n%s", view)
	}
	m, _ = pressed(m, "enter")

	if view := screen(m); !strings.Contains(view, "to account          expenses:gym") {
		t.Errorf("view:\n%s", view)
	}
}

func TestANewCategoryMoneyComesFromGoesUnderIncome(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "bonus")

	m, _ = pressed(m, "enter")

	if view := screen(m); !strings.Contains(view, "from account        income:bonus") {
		t.Errorf("view:\n%s", view)
	}
}

func TestTheNewCategoryFollowsTheMatchesAndTheCursorReachesIt(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "book")

	m, _ = pressed(m, "down", "down", "enter")

	if view := screen(m); !strings.Contains(view, "to account          expenses:book") ||
		strings.Contains(view, "expenses:books") {
		t.Errorf("view:\n%s", view)
	}
}

func TestEnterWithNothingToPickSaysSoAndPicksNothing(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "zz:")

	m, _ = pressed(m, "enter")

	view := screen(m)
	if !strings.Contains(view, "no account matches zz:") || !strings.Contains(view, "to account          zz:") {
		t.Errorf("view:\n%s", view)
	}
}

func TestNoNewCategoryIsOfferedForAnAccountNameOrAnUnfinishedOne(t *testing.T) {
	for _, text := range []string{"expenses:books", "expenses:"} {
		m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab", "tab")
		m = typed(m, text)

		if view := screen(m); strings.Contains(view, "+ create") {
			t.Errorf("%q offers a create entry:\n%s", text, view)
		}
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

	if saved(action) == nil || saved(action).Input.From != "assets:checking" {
		t.Errorf("action = %+v, want a save with the account kept", action)
	}
}

func TestEnterAndCtrlSAskTheRootToSaveWhatTheFormHolds(t *testing.T) {
	m := typed(opened(), "2026-10-15")
	m, _ = pressed(m, "tab", "tab", "tab")
	m = typed(m, "Book club")

	for _, key := range []string{"enter", "ctrl+s"} {
		_, action := pressed(m, key)

		if saved(action) == nil || saved(action).Editing != nil || saved(action).Input.Date != "2026-10-15" ||
			saved(action).Input.Description != "Book club" || saved(action).Input.Frequency != periodic.Monthly {
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

func TestATextFieldEditsAtItsCaretAndTheNextStartsAtItsEnd(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab")
	m = typed(m, "ac")
	m, _ = pressed(m, "left")

	m = typed(m, "b")
	if view := screen(m); !strings.Contains(view, "ab"+layout.Caret+"c") {
		t.Errorf("typed at the caret:\n%s", view)
	}
	m, _ = pressed(m, "tab")

	m = typed(m, "1")
	if view := screen(m); strings.Contains(view, layout.Caret+"1") || !strings.Contains(view, "1"+layout.Caret) {
		t.Errorf("the next field starts at its end:\n%s", view)
	}
}

func TestAPickerFilterEditsAtItsCaretAndMovingItKeepsTheMatchesCursor(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "as")

	m, _ = pressed(m, "down", "left")
	if m.form.cursor.Row != 1 || !m.form.browsing {
		t.Errorf("left moved the matches cursor: %d, %v", m.form.cursor.Row, m.form.browsing)
	}

	m = typed(m, "x")
	if view := screen(m); !strings.Contains(view, "ax"+layout.Caret+"s") || m.form.cursor.Row != 0 || m.form.browsing {
		t.Errorf("typed at the caret, the cursor starts over: %d, %v\n%s", m.form.cursor.Row, m.form.browsing, view)
	}
}

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
	if saved(action) == nil || saved(action).Editing == nil || saved(action).Editing.Pos != rule.Pos {
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
	if saved(action) == nil || saved(action).Input != want {
		t.Errorf("saved %+v, want %+v", saved(action), want)
	}
}

func TestOtherKeysLeaveADropdownAlone(t *testing.T) {
	m, _ := pressed(opened(), "tab", "x", "enter")

	if view := screen(m); !strings.Contains(view, "[monthly]") {
		t.Errorf("view:\n%s", view)
	}
}

func TestTheCursorCannotRunPastTheLastMatch(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "assets:")

	m, _ = pressed(m, "down", "down", "down", "down", "up", "enter")

	if view := screen(m); !strings.Contains(view, "from account        assets:nothing") {
		t.Errorf("one up from the end is the middle of three matches:\n%s", view)
	}
}

func TestTheHighlightedPickerRowIsTheOneUnderTheCursor(t *testing.T) {
	m, _ := pressed(opened(), "tab", "tab", "tab", "tab", "tab", "tab")
	m = typed(m, "assets:")

	m, _ = pressed(m, "down")

	highlighted := strings.Join(selectedLines(m.View()), "\n")
	if !strings.Contains(highlighted, "assets:nothing") || strings.Contains(highlighted, "assets:checking") ||
		strings.Contains(highlighted, "assets:wallet") {
		t.Errorf("highlighted rows:\n%s", highlighted)
	}
}

func copyLedger() domain.Ledger {
	ledger := withAccounts()
	style := domain.DefaultAmount("EUR").Style
	booking := func(day int, description, to string, cents int64) domain.Transaction {
		return domain.Transaction{Date: date(fmt.Sprintf("2026-09-%02d", day)), Description: description, Postings: []domain.Posting{
			{Account: to, Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: cents, Places: 2, Style: style}}},
			{Account: "assets:checking", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: -cents, Places: 2, Style: style}}},
		}}
	}
	ledger.Transactions = []domain.Transaction{
		booking(2, "Vermieter Mueller", "expenses:rent", 90000),
		booking(10, "Thalia", "expenses:books", 2000),
		booking(20, "REWE", "expenses:groceries", 4217),
	}

	return ledger
}

func copyList(t *testing.T) Model {
	t.Helper()

	m := New().Resize(180, 40).WithLedger(copyLedger()).OpenCopy()
	if !m.Capturing() || !strings.Contains(screen(m), "─ copy a transaction ") {
		t.Fatalf("no copy picker:\n%s", screen(m))
	}

	return m
}

func TestCopyListsTheTransactionsNewestFirstAndFiltersThem(t *testing.T) {
	m := copyList(t)
	view := screen(m)

	rewe, rent := strings.Index(view, "2026-09-20  REWE"), strings.Index(view, "2026-09-02  Vermieter Mueller")
	if rewe < 0 || rent < 0 || rewe > rent || !strings.Contains(view, "42,17 EUR") || !strings.Contains(view, "assets:checking → expenses:groceries") {
		t.Errorf("the list:\n%s", view)
	}
}

func TestTheCopyListFiltersByDescription(t *testing.T) {
	m := typed(copyList(t), "thal")
	if view := screen(m); !strings.Contains(view, "Thalia") || strings.Contains(view, "REWE") {
		t.Errorf("filtered:\n%s", view)
	}

	if m = typed(m, "x"); !strings.Contains(screen(m), "no transaction matches") {
		t.Errorf("nothing left:\n%s", screen(m))
	}

	if m, _ = pressed(m, "enter"); m.form != nil || m.copying == nil {
		t.Error("Enter with nothing listed opened a form")
	}
}

func TestMovingTheCaretKeepsTheTransactionUnderTheCursor(t *testing.T) {
	m, _ := pressed(typed(copyList(t), "e"), "down", "left")
	if m.copying.cursor.Row != 1 || m.copying.back != 1 {
		t.Errorf("cursor %d, caret %d", m.copying.cursor.Row, m.copying.back)
	}
}

func TestALongCopyListScrollsAndHighlightsTheTransactionUnderTheCursor(t *testing.T) {
	ledger := copyLedger()
	for day := 1; day <= 9; day++ {
		ledger.Transactions = append(ledger.Transactions, domain.Transaction{
			Date:        date(fmt.Sprintf("2026-08-%02d", day)),
			Description: fmt.Sprintf("Old %d", day),
		})
	}
	m, _ := pressed(New().Resize(120, 12).WithLedger(ledger).OpenCopy(), "end")
	lines := strings.Split(m.View(), "\n")
	title := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, copyTitle) })
	var highlighted []string
	for _, line := range lines[title+1:] {
		if strings.HasPrefix(line, "\x1b[") { // an unselected row starts with its plain margin
			highlighted = append(highlighted, uitest.Plain(line))
		}
	}

	if shown := len(lines) - title - 1; shown != 5 { // half the screen: the title, the filter and four rows
		t.Errorf("the list takes %d lines:\n%s", shown, screen(m))
	}

	if len(highlighted) != 1 || !strings.Contains(highlighted[0], "Old 1") {
		t.Errorf("highlighted %q in:\n%s", highlighted, screen(m))
	}
}

func TestEnterOpensTheNewRuleFormFilledFromTheTransaction(t *testing.T) {
	m, _ := pressed(copyList(t), "down", "enter")
	if m.copying != nil || m.form == nil || m.form.editing != nil {
		t.Fatalf("no new-rule form: %+v", m.form)
	}

	want := periodic.Input{
		Date: "2026-10-10", Frequency: periodic.Monthly, Description: "Thalia", Amount: "20,00",
		Currency: "EUR", From: "assets:checking", To: "expenses:books",
	}
	if m.form.input != want {
		t.Errorf("input = %+v, want %+v", m.form.input, want)
	}
}

func TestTheCopyListMovesItsCursorAndEscClosesIt(t *testing.T) {
	m, _ := pressed(copyList(t), "end")
	if m.copying.cursor.Row != 2 {
		t.Errorf("end: %d", m.copying.cursor.Row)
	}

	if m, _ = pressed(typed(m, "e"), "left"); m.copying.back != 1 || m.copying.cursor.Row != 0 {
		t.Errorf("typing starts the list over and left moves the caret: %+v", *m.copying)
	}

	if m, _ = pressed(m, "esc"); m.Capturing() || strings.Contains(screen(m), "copy a transaction") {
		t.Errorf("esc did not close:\n%s", screen(m))
	}
}

func TestCAsksForTheCopyList(t *testing.T) {
	if _, action := New().Resize(180, 40).WithLedger(copyLedger()).Key("c"); action != (CopyTransaction{}) {
		t.Errorf("action = %#v", action)
	}
}
