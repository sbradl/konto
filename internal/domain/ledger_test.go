package domain

import (
	"testing"
	"time"
)

func TestUnclassifiedIssuesListsEveryUnclassifiedAccount(t *testing.T) {
	ledger := Ledger{Accounts: []Account{
		{Name: "assets:checking", Source: SourceImport},
		{Name: "assets:spare"},
		{Name: "expenses:rent"},
		{Name: "liabilities:card"},
	}}

	issues := ledger.UnclassifiedIssues()

	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2: %+v", len(issues), issues)
	}
	want := CheckIssue{Check: "unclassified", Severity: SeverityWarn, Message: "assets:spare has no konto:source tag"}
	if issues[0] != want {
		t.Errorf("first issue = %+v, want %+v", issues[0], want)
	}
}

func TestUnclassifiedIssuesIsEmptyForAClassifiedLedger(t *testing.T) {
	if issues := (Ledger{Accounts: []Account{{Name: "assets:a", Source: SourceManual}}}).UnclassifiedIssues(); len(issues) != 0 {
		t.Errorf("got %+v, want none", issues)
	}
}

func openAccount(name string, mantissa int64) Account {
	return Account{Name: name, Source: SourceImport, Balance: MultiAmount{eur(mantissa)}}
}

func TestNetWorthSumsOpenAssetsAndLiabilitiesOnly(t *testing.T) {
	closed := openAccount("assets:old", 5000)
	closed.Closed = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	ledger := Ledger{Accounts: []Account{
		openAccount("assets:checking", 1324000), openAccount("assets:savings", 1350000), openAccount("liabilities:card", -14000),
		closed, openAccount("expenses:rent", 810000), openAccount("equity:opening", -1000000),
	}}

	got := ledger.NetWorth()

	if len(got) != 1 || got[0].Mantissa != 2660000 {
		t.Errorf("NetWorth = %+v, want 26.600,00", got)
	}
}

func TestMonthFiguresFallBackToAZeroInTheLedgersCommodity(t *testing.T) {
	ledger := Ledger{Accounts: []Account{{Name: "equity:x"}, openAccount("assets:a", 100)}}

	for name, got := range map[string]MultiAmount{
		"income": ledger.MonthIncome(), "expenses": ledger.MonthExpenses(), "net": ledger.MonthNet(),
	} {
		if len(got) != 1 || got[0].Mantissa != 0 || got[0].Format() != "0,00 EUR" {
			t.Errorf("%s = %+v, want 0,00 EUR", name, got)
		}
	}
}

func TestMonthNetIsIncomeLessExpenses(t *testing.T) {
	ledger := Ledger{Month: MonthSummary{Month: time.September, Income: MultiAmount{eur(300000)}, Expenses: MultiAmount{eur(118500)}}}

	if got := ledger.MonthNet(); got[0].Mantissa != 181500 {
		t.Errorf("MonthNet = %+v, want 1.815,00", got)
	}
	if ledger.MonthIncome()[0].Mantissa != 300000 || ledger.MonthExpenses()[0].Mantissa != 118500 {
		t.Error("income and expenses should pass through")
	}
}

func TestZeroIsNilWithoutBalances(t *testing.T) {
	if got := (Ledger{Accounts: []Account{{Name: "assets:a"}}}).Zero(); got != nil {
		t.Errorf("Zero = %+v, want nil", got)
	}
}

func TestWatermarkIsTheLatestTransactionDateOfTheAccount(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	ledger := Ledger{Transactions: []Transaction{
		{Date: day(10), Postings: []Posting{{Account: "assets:checking"}}},
		{Date: day(26), Postings: []Posting{{Account: "assets:savings"}, {Account: "assets:checking"}}},
		{Date: day(29), Postings: []Posting{{Account: "assets:savings"}}},
		{Date: day(5), Postings: []Posting{{Account: "assets:checking"}}},
	}}

	got, ok := ledger.Watermark("assets:checking")

	if !ok || !got.Equal(day(26)) {
		t.Errorf("watermark = %v, %v; want 2026-09-26", got, ok)
	}
	if _, ok := ledger.Watermark("assets:none"); ok {
		t.Error("an account without transactions has no watermark")
	}
}

func TestWatermarkOfAnAccountWhoseOnlyTransactionIsDatedTheZeroTime(t *testing.T) {
	ledger := Ledger{Transactions: []Transaction{{Postings: []Posting{{Account: "assets:x"}}}}}

	if _, ok := ledger.Watermark("assets:x"); !ok {
		t.Error("a transaction counts whatever its date")
	}
}

func TestWatermarkLeavesTheCarryOut(t *testing.T) {
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	ledger := Ledger{Transactions: []Transaction{
		{Date: day(2025, 12, 28), Postings: []Posting{{Account: "assets:b"}}},
		{Date: day(2025, 12, 31), Comment: CarryTag, Postings: []Posting{{Account: "assets:b"}}},
		{Date: day(2026, 1, 1), Comment: CarryTag, Postings: []Posting{{Account: "assets:b"}}},
	}}

	got, ok := ledger.Watermark("assets:b")

	if !ok || !got.Equal(day(2025, 12, 28)) {
		t.Errorf("watermark = %v, %v; want 2025-12-28, the carry moves no watermark", got, ok)
	}
}

func TestAmountLikeTakesTheStyleOfAnAccountHoldingTheCommodity(t *testing.T) {
	style := AmountStyle{SymbolLeft: true, DecimalMark: ".", GroupMark: ",", GroupSize: 3}
	ledger := Ledger{Accounts: []Account{
		{Name: "assets:a", Balance: MultiAmount{{Commodity: "EUR", Mantissa: 100, Places: 2}}},
		{Name: "assets:b", Balance: MultiAmount{{Commodity: "USD", Mantissa: 5, Places: 3, Style: style}}},
	}}

	got := ledger.AmountLike("USD")

	if got.Commodity != "USD" || got.Places != 3 || got.Style != style || got.Mantissa != 0 {
		t.Errorf("like = %+v", got)
	}
}

func TestAmountLikeFallsBackToTheCommaStyle(t *testing.T) {
	got := Ledger{}.AmountLike("EUR")

	if got.Format() != "0,00 EUR" || got.Style.GroupMark != "." {
		t.Errorf("like = %+v (%s)", got, got.Format())
	}
}
