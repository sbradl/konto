package domain

import (
	"reflect"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }

func posting(account string, mantissa int64) Posting {
	return Posting{Account: account, Amounts: MultiAmount{eur(mantissa)}}
}

func recentLedger() Ledger {
	return Ledger{
		Accounts: []Account{
			{Name: "assets:checking", Source: SourceImport},
			{Name: "assets:savings", Source: SourceImport},
			{Name: "assets:transfers", Source: SourceClearing},
		},
		Transactions: []Transaction{
			{Date: day(10), Description: "REWE", Postings: []Posting{posting("expenses:groceries", 25000), posting("assets:checking", -25000)}},
			{Date: day(25), Description: "Acme", Postings: []Posting{posting("assets:checking", 300000), posting("income:salary", -300000)}},
			{Date: day(26), Description: "Sparen", Postings: []Posting{posting("assets:transfers", 50000), posting("assets:checking", -50000)}},
			{Date: day(26), Description: "Sparen", Postings: []Posting{posting("assets:savings", 50000), posting("assets:transfers", -50000)}},
		},
	}
}

func TestRecentListsNewestFirstAndLaterTransactionsFirstWithinADay(t *testing.T) {
	got := recentLedger().Recent()

	order := make([]string, len(got))
	for i, entry := range got {
		order[i] = entry.Date.Format("01-02") + " " + entry.Description + " " + entry.Category
	}
	want := []string{
		"09-26 Sparen assets:transfers", "09-26 Sparen assets:transfers",
		"09-25 Acme income:salary", "09-10 REWE expenses:groceries",
	}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("recent = %v, want %v", order, want)
	}
}

func TestRecentSeesATransactionFromItsOwnAccount(t *testing.T) {
	got := recentLedger().Recent()

	// the later Sparen is the savings leg: +500,00; the earlier is the checking leg: -500,00
	if got[0].Amount[0].Mantissa != 50000 || got[1].Amount[0].Mantissa != -50000 {
		t.Errorf("transfer legs = %+v, %+v; want +500,00 then -500,00", got[0].Amount, got[1].Amount)
	}
	if got[2].Amount[0].Mantissa != 300000 || got[3].Amount[0].Mantissa != -25000 {
		t.Errorf("income and expense = %+v, %+v", got[2].Amount, got[3].Amount)
	}
}

func TestRecentFallsBackToTheFirstPostingWithoutAnOwnAccount(t *testing.T) {
	ledger := Ledger{Transactions: []Transaction{{
		Date: day(1), Description: "Odd",
		Postings: []Posting{posting("expenses:a", 100), posting("equity:b", -100)},
	}}}

	got := ledger.Recent()[0]

	if got.Category != "equity:b" || got.Amount[0].Mantissa != 100 {
		t.Errorf("entry = %+v, want the first posting as subject and equity:b as category", got)
	}
}

func TestRecentHandlesATransactionWithoutPostings(t *testing.T) {
	got := (Ledger{Transactions: []Transaction{{Date: day(1), Description: "Empty"}}}).Recent()

	if len(got) != 1 || got[0].Description != "Empty" || got[0].Category != "" || got[0].Amount != nil {
		t.Errorf("recent = %+v", got)
	}
}

func TestRecentOfASinglePostingHasNoCategory(t *testing.T) {
	ledger := Ledger{Transactions: []Transaction{{Date: day(1), Postings: []Posting{posting("assets:a", 100)}}}}

	if got := ledger.Recent()[0]; got.Category != "" {
		t.Errorf("category = %q, want none", got.Category)
	}
}
