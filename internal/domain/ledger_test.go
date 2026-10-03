package domain

import "testing"

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
