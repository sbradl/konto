package report

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

func eur(mantissa int64) domain.MultiAmount {
	style := domain.DefaultAmount("EUR").Style
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}}
}

func account(name string, mantissa int64) domain.Account {
	return domain.Account{Name: name, Balance: eur(mantissa)}
}

// fakeGateway answers by the first query it is asked about; it records every call.
type fakeGateway struct {
	until    map[string][]domain.Account // by first query, balances up to today
	activity map[string][]domain.Account // by first query, change within the period
	calls    []string
	err      error
}

func (f *fakeGateway) BalancesUntil(_ context.Context, last time.Time, prefixes ...string) ([]domain.Account, error) {
	f.calls = append(f.calls, "until "+last.Format(time.DateOnly)+" "+strings.Join(prefixes, ","))
	return f.until[prefixes[0]], f.err
}

func (f *fakeGateway) Activity(_ context.Context, first, last time.Time, prefixes ...string) ([]domain.Account, error) {
	f.calls = append(f.calls, "activity "+first.Format(time.DateOnly)+".."+last.Format(time.DateOnly)+" "+strings.Join(prefixes, ","))
	return f.activity[prefixes[0]], f.err
}

var sept30 = func() time.Time { return time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC) }

func household() *fakeGateway {
	return &fakeGateway{
		until: map[string][]domain.Account{
			"assets:": {account("assets:checking", 1324000), account("assets:savings", 1350000)},
			"income:": {account("income:salary", -900000), account("expenses:rent", 270000)},
		},
		activity: map[string][]domain.Account{
			"assets:": {account("assets:checking", 135000), account("liabilities:creditcard", -3500)},
			"income:": {account("income:salary", -300000), account("expenses:rent", 90000), account("expenses:books", 3500)},
		},
	}
}

func rows(table []domain.ReportRow) []string {
	lines := make([]string, len(table))
	for i, row := range table {
		lines[i] = row.Label + "|"
		if row.Kind != domain.RowHeader {
			lines[i] += row.Amount.Format()
		}
		if row.Delta != nil {
			lines[i] += "|" + row.Delta.Format()
		}
	}
	return lines
}

func TestBalanceSheetShowsBalancesAtTodayAndTheChangeWithinThePeriod(t *testing.T) {
	gw := household()
	gw.until["assets:"] = append(gw.until["assets:"], account("liabilities:creditcard", -14000))

	got, err := NewService(gw, sept30).Report(context.Background(), domain.MonthToDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"Assets|",
		"assets:checking|13.240,00 EUR|1.350,00 EUR",
		"assets:savings|13.500,00 EUR|0,00 EUR",
		"Total assets|26.740,00 EUR|1.350,00 EUR",
		"Liabilities|",
		"liabilities:creditcard|-140,00 EUR|-35,00 EUR",
		"Total liabilities|-140,00 EUR|-35,00 EUR",
		"Net worth|26.600,00 EUR|1.315,00 EUR",
	}
	if !reflect.DeepEqual(rows(got.BalanceSheet), want) {
		t.Errorf("balance sheet = %q, want %q", rows(got.BalanceSheet), want)
	}
	if got.BalanceSheet[0].Kind != domain.RowHeader || got.BalanceSheet[1].Kind != domain.RowLeaf ||
		got.BalanceSheet[3].Kind != domain.RowSubtotal || got.BalanceSheet[7].Kind != domain.RowGrandTotal {
		t.Errorf("row kinds = %v", got.BalanceSheet)
	}
}

func TestIncomeStatementShowsPositiveFiguresAndTheNet(t *testing.T) {
	got, err := NewService(household(), sept30).Report(context.Background(), domain.MonthToDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"Income|",
		"income:salary|3.000,00 EUR",
		"Total income|3.000,00 EUR",
		"Expenses|",
		"expenses:rent|900,00 EUR",
		"expenses:books|35,00 EUR",
		"Total expenses|935,00 EUR",
		"Net|2.065,00 EUR",
	}
	if !reflect.DeepEqual(rows(got.IncomeStatement), want) {
		t.Errorf("income statement = %q, want %q", rows(got.IncomeStatement), want)
	}
}

func TestEveryPeriodEndsToday(t *testing.T) {
	gw := household()

	if _, err := NewService(gw, sept30).Report(context.Background(), domain.QuarterToDate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"until 2026-09-30 assets:,liabilities:",
		"activity 2026-07-01..2026-09-30 assets:,liabilities:",
		"activity 2026-07-01..2026-09-30 income:,expenses:",
	}
	if !reflect.DeepEqual(gw.calls, want) {
		t.Errorf("calls = %q, want %q", gw.calls, want)
	}
}

func TestAllTimeHasNoDeltaAndReadsTheStatementFromBalances(t *testing.T) {
	gw := household()

	got, err := NewService(gw, sept30).Report(context.Background(), domain.AllTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, row := range got.BalanceSheet {
		if row.Delta != nil {
			t.Errorf("row %q has a Δ under all time", row.Label)
		}
	}
	if want := "until 2026-09-30 income:,expenses:"; gw.calls[1] != want || len(gw.calls) != 2 {
		t.Errorf("calls = %q, want the second to be %q", gw.calls, want)
	}
	if got.IncomeStatement[len(got.IncomeStatement)-1].Amount.Format() != "6.300,00 EUR" {
		t.Errorf("all-time net = %v", got.IncomeStatement)
	}
}

func TestAnEmptyPeriodGivesEmptyStatementAndNoZeroRows(t *testing.T) {
	gw := &fakeGateway{until: map[string][]domain.Account{"assets:": {account("assets:old", 0)}}}

	got, err := NewService(gw, sept30).Report(context.Background(), domain.MonthToDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.IncomeStatement) != 0 || len(got.BalanceSheet) != 0 {
		t.Errorf("want empty tables, got %q / %q", rows(got.BalanceSheet), rows(got.IncomeStatement))
	}
}

func TestAnAccountWithoutChangeShowsAZeroDelta(t *testing.T) {
	gw := &fakeGateway{until: map[string][]domain.Account{"assets:": {account("assets:savings", 1350000)}}}

	got, _ := NewService(gw, sept30).Report(context.Background(), domain.MonthToDate)

	if rows(got.BalanceSheet)[1] != "assets:savings|13.500,00 EUR|0,00 EUR" {
		t.Errorf("row = %q", rows(got.BalanceSheet)[1])
	}
}

func TestOnlyExpensesStillGiveANet(t *testing.T) {
	gw := &fakeGateway{activity: map[string][]domain.Account{"income:": {account("expenses:rent", 90000)}}}

	got, _ := NewService(gw, sept30).Report(context.Background(), domain.MonthToDate)

	want := []string{"Expenses|", "expenses:rent|900,00 EUR", "Total expenses|900,00 EUR", "Net|-900,00 EUR"}
	if !reflect.DeepEqual(rows(got.IncomeStatement), want) {
		t.Errorf("income statement = %q, want %q", rows(got.IncomeStatement), want)
	}
}

func TestAnAccountThatEndsAtZeroStillCountsItsChange(t *testing.T) {
	gw := &fakeGateway{
		until: map[string][]domain.Account{"assets:": {account("assets:checking", 1324000)}},
		activity: map[string][]domain.Account{"assets:": {
			account("assets:checking", 500000), account("assets:savings", -500000), account("liabilities:card", 14000),
		}},
	}

	got, _ := NewService(gw, sept30).Report(context.Background(), domain.MonthToDate)

	want := []string{
		"Assets|",
		"assets:checking|13.240,00 EUR|5.000,00 EUR",
		"Total assets|13.240,00 EUR|0,00 EUR",
		"Net worth|13.240,00 EUR|140,00 EUR",
	}
	if !reflect.DeepEqual(rows(got.BalanceSheet), want) {
		t.Errorf("balance sheet = %q, want %q", rows(got.BalanceSheet), want)
	}
}

func TestReportPassesOnGatewayFailures(t *testing.T) {
	for _, period := range []domain.Period{domain.MonthToDate, domain.AllTime} {
		_, err := NewService(&fakeGateway{err: errors.New("boom")}, sept30).Report(context.Background(), period)

		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("%v: expected an error carrying the cause, got %v", period, err)
		}
	}
}

type failingLater struct {
	*fakeGateway
	failOn int
}

func (f failingLater) Activity(ctx context.Context, first, last time.Time, prefixes ...string) ([]domain.Account, error) {
	if len(f.calls)+1 == f.failOn {
		return nil, errors.New("later boom")
	}
	return f.fakeGateway.Activity(ctx, first, last, prefixes...)
}

func TestReportPassesOnALaterGatewayFailure(t *testing.T) {
	for _, failOn := range []int{2, 3} {
		_, err := NewService(failingLater{household(), failOn}, sept30).Report(context.Background(), domain.MonthToDate)

		if err == nil || !strings.Contains(err.Error(), "later boom") {
			t.Errorf("call %d failing: got %v", failOn, err)
		}
	}
}
