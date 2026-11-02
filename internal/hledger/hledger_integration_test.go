//go:build integration

package hledger

// These tests run every Client method against the real hledger and the fixture journals in testdata, so a
// change in hledger's JSON or its error output (the shapes issue.go and the readers here rely on) fails here
// rather than in front of the user (spec §6, testing).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// ledger is a Client on the fixture ledger: the main journal and two year files, which reach hledger as one
// journal on standard input (the 2026 balance assertion only holds across the include).
func ledger(t *testing.T) *Client {
	t.Helper()
	files := []string{fixture(t, "ledger/main.journal"), fixture(t, "ledger/2025.journal"), fixture(t, "ledger/2026.journal")}
	return New(Binary, func() []string { return files })
}

// single is a Client on one fixture file, named with -f.
func single(t *testing.T, name string) *Client {
	t.Helper()
	path := fixture(t, name)
	return New(Binary, func() []string { return []string{path} })
}

func on(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
}

// balancesOf is each account's balance as konto shows it.
func balancesOf(accounts []domain.Account) map[string]string {
	shown := map[string]string{}
	for _, account := range accounts {
		shown[account.Name] = account.Balance.Format()
	}
	return shown
}

// printed is the fixture ledger's transactions as Print reads them.
func printed(t *testing.T) []domain.Transaction {
	t.Helper()
	got, err := ledger(t).Print(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("transactions = %+v, want 3", got)
	}
	return got
}

func TestRealPrintReadsDatesCommentsAndSources(t *testing.T) {
	salary := printed(t)[1]

	if salary.Date != on(9, 1) || salary.Description != "Acme GmbH" || salary.Comment != "September" {
		t.Errorf("salary = %+v", salary)
	}
	if want := (domain.SourcePos{File: fixture(t, "ledger/2026.journal"), Line: 1}); salary.Source != want {
		t.Errorf("source = %+v, want %+v", salary.Source, want)
	}
}

func TestRealPrintReadsAmountsAndTags(t *testing.T) {
	rent := printed(t)[2]

	if posting := rent.Postings[0]; posting.Account != "expenses:rent" || posting.Amounts.Format() != "900,00 EUR" ||
		posting.Tags["konto-rule"] != "1" {
		t.Errorf("rent posting = %+v, want 900,00 EUR to expenses:rent tagged konto-rule:1", posting)
	}
	if amount := rent.Postings[1].Amounts; amount.Format() != "-900,00 EUR" || amount[0].Style != euroStyle {
		t.Errorf("inferred amount = %+v, want -900,00 EUR in the declared style", amount)
	}
}

func TestRealUpcomingProjectsThePeriodicRule(t *testing.T) {
	got, err := ledger(t).Upcoming(context.Background(), on(10, 1), on(12, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 || got[0].Date != on(10, 1) || got[1].Date != on(11, 1) || got[0].Description != "Rent" {
		t.Fatalf("occurrences = %+v, want Rent on 2026-10-01 and 2026-11-01", got)
	}
	if want := (domain.SourcePos{File: fixture(t, "ledger/main.journal"), Line: 11}); got[0].Source != want {
		t.Errorf("source = %+v, want the `~` line %+v", got[0].Source, want)
	}
}

func TestRealBalancesAreTheCurrentBalances(t *testing.T) {
	got, err := ledger(t).Balances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	shown := balancesOf(got)
	for account, want := range map[string]string{
		"assets:checking": "3.100,00 EUR", "expenses:rent": "900,00 EUR", "income:salary": "-4.000,00 EUR",
	} {
		if shown[account] != want {
			t.Errorf("%s = %q, want %q (all: %v)", account, shown[account], want, shown)
		}
	}
}

func TestRealMonthlyActivityIsOneEntryPerMonth(t *testing.T) {
	got, err := ledger(t).MonthlyActivity(context.Background(), on(8, 1), on(9, 30))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 || len(got[0]) != 0 {
		t.Fatalf("months = %+v, want August empty and September", got)
	}
	if shown := balancesOf(got[1]); len(shown) != 2 || shown["expenses:rent"] != "900,00 EUR" ||
		shown["income:salary"] != "-2.000,00 EUR" {
		t.Errorf("September = %v", shown)
	}
}

func TestRealActivityAndBalancesUntilKeepToTheirDaysAndPrefixes(t *testing.T) {
	activity, err := ledger(t).Activity(context.Background(), on(9, 1), on(9, 30), "expenses")
	if shown := balancesOf(activity); err != nil || len(shown) != 1 || shown["expenses:rent"] != "900,00 EUR" {
		t.Errorf("activity = %v, %v; want only the September rent", shown, err)
	}
	until, err := ledger(t).BalancesUntil(context.Background(), on(9, 1), "assets")
	if shown := balancesOf(until); err != nil || len(shown) != 1 || shown["assets:checking"] != "4.000,00 EUR" {
		t.Errorf("balances until 2026-09-01 = %v, %v; want the salary day counted", shown, err)
	}
}

func TestRealTextBalancesReadStandardInput(t *testing.T) {
	text := "decimal-mark ,\n2026-09-01 x\n    assets:cash    5,00 EUR\n    equity:opening-balances\n" +
		"2026-09-30 y\n    assets:cash    1,00 EUR\n    expenses:x\n"

	got, err := ledger(t).TextBalances(context.Background(), text, on(9, 30))

	if shown := balancesOf(got); err != nil || len(shown) != 1 || shown["assets:cash"] != "5,00 EUR" {
		t.Errorf("balances = %v, %v; want assets:cash before the day", shown, err)
	}
}

func TestRealProjectionGivesMonthEndsWithTheForecast(t *testing.T) {
	got, err := ledger(t).Projection(context.Background(), domain.ProjectionQuery{
		Account: "assets:checking", Begin: on(9, 1), ForecastFrom: on(10, 1), ForecastTo: on(12, 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balances := make([]string, len(got.Balances))
	for i, balance := range got.Balances {
		balances[i] = balance.Format()
	}
	wantEnds := []time.Time{on(10, 1), on(11, 1), on(12, 1)}
	if !slices.Equal(got.Ends, wantEnds) || !slices.Equal(balances, []string{"3.100,00 EUR", "2.200,00 EUR", "1.300,00 EUR"}) {
		t.Errorf("ends = %v, balances = %v", got.Ends, balances)
	}
}

func TestRealOccurrencesProjectRulesGivenOnStandardInput(t *testing.T) {
	text := "~ monthly from 2026-01-15  Gym\n    expenses:gym    1 EUR\n    assets:checking\n"

	got, err := ledger(t).Occurrences(context.Background(), text, on(10, 1), on(12, 1))

	if err != nil || !slices.Equal(got, []time.Time{on(10, 15), on(11, 15)}) {
		t.Errorf("occurrences = %v, %v", got, err)
	}
}

func TestRealConvertCSVAppliesTheRules(t *testing.T) {
	rules := "skip 1\nseparator ;\ndecimal-mark ,\nfields date, description, amount\n" +
		"currency EUR\naccount1 assets:checking\naccount2 expenses:groceries\ncomment2 konto-rule:7\n"

	got, err := ledger(t).ConvertCSV(context.Background(), fixture(t, "export.csv"), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].Date != on(9, 30) || got[0].Description != "REWE" {
		t.Fatalf("transactions = %+v", got)
	}
	if postings := got[0].Postings; postings[0].Amounts[0].Mantissa != -4217 || postings[0].Amounts[0].Commodity != "EUR" ||
		postings[1].Account != "expenses:groceries" || postings[1].Tags["konto-rule"] != "7" {
		t.Errorf("postings = %+v", postings)
	}
}

func TestRealCheckPassesAndLocatesFailures(t *testing.T) {
	if issue, err := ledger(t).Check(context.Background(), "assertions"); issue != nil || err != nil {
		t.Errorf("assertions across the ledger's files = %+v, %v; want them to hold", issue, err)
	}
	issue, err := single(t, "broken/checks.journal").Check(context.Background(), "accounts")
	if err != nil || issue == nil {
		t.Fatalf("issue = %+v, %v; want the undeclared account", issue, err)
	}
	want := domain.CheckIssue{Check: "accounts", Severity: domain.SeverityWarn, File: fixture(t, "broken/checks.journal"), Line: 8}
	message := issue.Message
	issue.Message = ""
	if *issue != want || !strings.Contains(message, `account "expenses:undeclared" has not been declared.`) {
		t.Errorf("issue = %+v with message %q, want %+v", *issue, message, want)
	}
}

func TestRealCheckReadsAFailedAssertionWithItsColumn(t *testing.T) {
	issue, err := single(t, "broken/assertion.journal").Check(context.Background(), "assertions")
	if err != nil || issue == nil {
		t.Fatalf("issue = %+v, %v; want the failed assertion", issue, err)
	}
	if issue.Line != 5 || issue.Col == 0 || !strings.HasPrefix(issue.Message, "Balance assertion failed in assets:checking") {
		t.Errorf("issue = %+v", *issue)
	}
}

func TestRealCheckTextSaysWhyInHledgersWords(t *testing.T) {
	text, err := os.ReadFile(fixture(t, "broken/assertion.journal"))
	if err != nil {
		t.Fatal(err)
	}

	err = ledger(t).CheckText(context.Background(), string(text), []string{"assertions"})

	if err == nil || !strings.HasPrefix(err.Error(), "Balance assertion failed in assets:checking") {
		t.Errorf("error = %v", err)
	}
	if err := ledger(t).CheckText(context.Background(), "decimal-mark ,\n", []string{"assertions"}); err != nil {
		t.Errorf("a clean text fails: %v", err)
	}
}

func TestRealParseErrorsAreLocated(t *testing.T) {
	for name, want := range map[string]domain.CheckIssue{
		"broken/parse.journal": {Message: "unexpected 'E'", Line: 4, Col: 28},
		// a line range and no column
		"broken/unbalanced.journal": {Message: "This transaction is unbalanced.", Line: 3},
	} {
		_, err := single(t, name).Print(context.Background())

		var got domain.JournalError
		if !errors.As(err, &got) {
			t.Fatalf("%s: error = %v, want a JournalError", name, err)
		}
		want.Check, want.Severity, want.File = domain.ParseableCheck, domain.SeverityErr, fixture(t, name)
		if got.Issue != want {
			t.Errorf("%s: issue = %+v, want %+v", name, got.Issue, want)
		}
	}
}

func TestRealPreflightAcceptsTheInstalledHledger(t *testing.T) {
	if version, err := PreflightOnPath(); err != nil || !version.AtLeast(minVersion()) {
		t.Errorf("version = %+v, %v", version, err)
	}
}
