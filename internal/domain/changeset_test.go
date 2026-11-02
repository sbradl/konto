package domain

import (
	"slices"
	"testing"
	"time"
)

// Every edit kind is an Edit; the closed set is what lets the journal adapter switch over it.
func TestEditKindsAreEdits(*testing.T) {
	for _, edit := range []Edit{
		SetAccountTag{}, RemoveAccountTag{}, DeleteRule{}, Declare{}, AddTransactions{}, SetCategoryRule{}, AddRule{}, ReplaceRule{}, SetCarry{},
	} {
		edit.isEdit()
	}
}

func euros(mantissa int64) MultiAmount {
	style := DefaultAmount("EUR").Style
	return MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}}
}

func TestTransactionLinesCarryCommentsAmountsAndAssertions(t *testing.T) {
	transaction := NewTransaction{
		Date: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), Description: "Kaffeehaus", Comment: "Kaffee",
		Postings: []NewPosting{
			{Account: "expenses:cafe", Amount: euros(320), Comment: "konto-rule:7"},
			{Account: "assets:checking", Amount: euros(-320), Assertion: euros(1318203)},
			{Account: "assets:other", Assertion: euros(500)},
			{Account: "assets:rest"},
		},
	}

	want := []string{
		"2026-09-28 Kaffeehaus  ; Kaffee",
		"    expenses:cafe  3,20 EUR  ; konto-rule:7",
		"    assets:checking  -3,20 EUR = 13.182,03 EUR",
		"    assets:other  = 5,00 EUR",
		"    assets:rest",
	}
	if got := transaction.Lines(); !slices.Equal(got, want) {
		t.Errorf("lines =\n%q\nwant\n%q", got, want)
	}
}

func TestTransactionWithoutCommentHasNoSemicolon(t *testing.T) {
	transaction := NewTransaction{
		Date: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), Description: "Kiosk",
		Postings: []NewPosting{{Account: "expenses:books", Amount: euros(200)}},
	}

	if got := transaction.Lines()[0]; got != "2026-09-28 Kiosk" {
		t.Errorf("header = %q", got)
	}
}
