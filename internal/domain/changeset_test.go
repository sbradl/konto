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
	style := AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}
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

// A rule's block runs from the blank line before its id line to the blank line after it.
func TestBlockIsTheParagraphAroundTheRulesIDLine(t *testing.T) {
	lines := []string{
		"if %payee A", "    comment2 konto-rule:1", "",
		"if %payee B", "& %purpose C", "    account2 expenses:b", "    comment2 konto-rule:2", "    x", "",
		"if %payee D", "    comment2 konto-rule:3",
	}
	for _, tt := range []struct {
		id         string
		first, end int
		found      bool
	}{{"1", 0, 2, true}, {"2", 3, 8, true}, {"3", 9, 11, true}, {"4", 0, 0, false}} {
		first, end, found := SetCategoryRule{ID: tt.id}.Block(lines)
		if first != tt.first || end != tt.end || found != tt.found {
			t.Errorf("rule %s: Block = %d, %d, %v; want %d, %d, %v", tt.id, first, end, found, tt.first, tt.end, tt.found)
		}
	}
	idFirst := []string{"comment2 konto-rule:5", "    account2 x", "", "comment2 konto-rule:6"}
	for id, want := range map[string][2]int{"5": {0, 2}, "6": {3, 4}} {
		if first, end, found := (SetCategoryRule{ID: id}).Block(idFirst); !found || [2]int{first, end} != want {
			t.Errorf("rule %s opening its block: Block = %d, %d, %v; want %v", id, first, end, found, want)
		}
	}
}

// A line of nothing but spaces ends a block too, as it does when the rules are read.
func TestBlockEndsAtALineOfSpaces(t *testing.T) {
	lines := []string{"if %payee A", "    comment2 konto-rule:1", "  ", "if %payee B", "    comment2 konto-rule:2", "\t"}

	if first, end, found := (SetCategoryRule{ID: "2"}).Block(lines); !found || first != 3 || end != 5 {
		t.Errorf("Block = %d, %d, %v; want 3, 5", first, end, found)
	}
	if !IsBlankLine(" \t") || IsBlankLine(" x") {
		t.Error("only a line of spaces is blank")
	}
}
