package forecast

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

var euroStyle = domain.DefaultAmount("EUR").Style

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

func amount(commodity string, mantissa int64) domain.Amount {
	return domain.Amount{Commodity: commodity, Mantissa: mantissa, Places: 2, Style: euroStyle}
}

type fakeGateway struct {
	mu          sync.Mutex
	queries     []domain.ProjectionQuery
	projections map[string]domain.Projection
	err         error
}

func (f *fakeGateway) Projection(_ context.Context, query domain.ProjectionQuery) (domain.Projection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, query)
	return f.projections[query.Account], f.err
}

// asked are the queries by account name: they run side by side, so their order is no one's.
func (f *fakeGateway) asked() []domain.ProjectionQuery {
	queries := slices.Clone(f.queries)
	slices.SortFunc(queries, func(a, b domain.ProjectionQuery) int { return strings.Compare(a.Account, b.Account) })
	return queries
}

func today() time.Time { return day("2026-09-30") }

// months is a projection with a balance at the end of each of the given months.
func months(ends []string, balances ...domain.MultiAmount) domain.Projection {
	projection := domain.Projection{Balances: balances}
	for _, end := range ends {
		projection.Ends = append(projection.Ends, day(end))
	}
	return projection
}

func ledger() domain.Ledger {
	return domain.Ledger{
		Accounts: []domain.Account{
			{Name: "liabilities:creditcard", Source: domain.SourceImport},
			{Name: "assets:savings", Source: domain.SourceImport},
			{Name: "assets:checking", Source: domain.SourceImport},
			{Name: "assets:transfers", Source: domain.SourceClearing},
			{Name: "assets:old", Source: domain.SourceImport, Closed: day("2026-03-01")},
			{Name: "assets:cash", Source: domain.SourceManual},
			{Name: "expenses:rent"},
		},
		Transactions: []domain.Transaction{
			{Date: day("2026-09-26"), Postings: []domain.Posting{{Account: "assets:checking"}}},
			{Date: day("2026-09-14"), Postings: []domain.Posting{{Account: "liabilities:creditcard"}}},
		},
	}
}

func TestForecastProjectsEachOpenAccountByName(t *testing.T) {
	gw := &fakeGateway{}

	if _, err := NewService(gw, today).Forecast(context.Background(), ledger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make([]string, 0, len(gw.queries))
	for _, query := range gw.asked() {
		names = append(names, query.Account)
	}
	want := []string{"assets:cash", "assets:checking", "assets:savings", "liabilities:creditcard"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("projected %v, want %v: no closed, clearing or category account", names, want)
	}
}

func TestForecastStartsAnImportFedAccountAfterItsWatermarkAndOthersToday(t *testing.T) {
	gw := &fakeGateway{}

	if _, err := NewService(gw, today).Forecast(context.Background(), ledger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	from := map[string]time.Time{}
	for _, query := range gw.asked() {
		from[query.Account] = query.ForecastFrom
	}
	for account, want := range map[string]string{
		"assets:checking": "2026-09-27", "liabilities:creditcard": "2026-09-15",
		"assets:savings": "2026-09-30", // import-fed but no transaction yet
		"assets:cash":    "2026-09-30", // manual
	} {
		if !from[account].Equal(day(want)) {
			t.Errorf("%s counts rules from %v, want %s", account, from[account], want)
		}
	}
	first := gw.asked()[0]
	if !first.Begin.Equal(day("2026-03-30")) || !first.ForecastTo.Equal(day("2027-09-30")) {
		t.Errorf("query = %+v, want six months back and twelve ahead", first)
	}
}

// multiCurrency is the forecast of a checking account in euros and a wallet in three currencies.
func multiCurrency(t *testing.T) Forecast {
	t.Helper()
	ends := []string{"2026-09-30", "2026-10-30"}
	gw := &fakeGateway{projections: map[string]domain.Projection{
		"assets:checking": months(ends, domain.MultiAmount{amount("EUR", 100)}, domain.MultiAmount{amount("EUR", 200)}),
		"assets:cash": months(ends, nil,
			domain.MultiAmount{amount("CZK", 5), amount("EUR", 7), amount("PLN", 9)}),
	}}

	got, err := NewService(gw, today).Forecast(context.Background(), ledger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return got
}

func TestForecastMakesOneChartPerCurrencyWithALinePerAccountHoldingIt(t *testing.T) {
	var shape []string
	for _, chart := range multiCurrency(t).Charts {
		for _, line := range chart.Lines {
			shape = append(shape, chart.Commodity+" "+line.Account)
		}
	}

	want := []string{"CZK assets:cash", "EUR assets:cash", "EUR assets:checking", "PLN assets:cash"}
	if !reflect.DeepEqual(shape, want) {
		t.Errorf("lines = %v, want %v (charts in the order the currencies first show up)", shape, want)
	}
}

func TestLinesRankByAccountNameAndAreZeroBeforeTheAccountHasABalance(t *testing.T) {
	euros := multiCurrency(t).Charts[1]
	cash := euros.Lines[0]

	if cash.Rank != 0 || euros.Lines[1].Rank != 1 {
		t.Errorf("ranks = %d, %d; want each account's place by name", cash.Rank, euros.Lines[1].Rank)
	}
	if first := cash.Points[0].Balance; first.Mantissa != 0 || first.Commodity != "EUR" || first.Style != euroStyle {
		t.Errorf("before the account has a balance the line is a zero in the same style, got %+v", first)
	}
	if cash.Points[1].Balance.Mantissa != 7 {
		t.Errorf("points = %+v", cash.Points)
	}
}

func TestForecastTakesTheMilestonesFromTheMonthsThatEndOnThoseDays(t *testing.T) {
	ends := []string{"2026-08-30", "2026-09-30", "2026-10-30", "2026-11-30", "2026-12-30", "2027-03-30", "2027-09-30"}
	balances := make([]domain.MultiAmount, 0, len(ends))
	for i := range ends {
		balances = append(balances, domain.MultiAmount{amount("EUR", int64(100*(i+1)))})
	}
	gw := &fakeGateway{projections: map[string]domain.Projection{"assets:checking": months(ends, balances...)}}

	got, err := NewService(gw, today).Forecast(context.Background(), domain.Ledger{Accounts: []domain.Account{
		{Name: "assets:checking", Source: domain.SourceManual},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mantissas := make([]int64, 0, len(Milestones()))
	for _, milestone := range got.Charts[0].Lines[0].Milestones {
		mantissas = append(mantissas, milestone.Mantissa)
	}
	if want := []int64{200, 500, 600, 700}; !reflect.DeepEqual(mantissas, want) {
		t.Errorf("milestones = %v, want %v (today, +3, +6, +12 months)", mantissas, want)
	}
}

func TestAMilestoneBeforeTheFirstMonthIsZero(t *testing.T) {
	gw := &fakeGateway{projections: map[string]domain.Projection{
		"assets:checking": months([]string{"2027-09-30"}, domain.MultiAmount{amount("EUR", 700)}),
	}}

	got, _ := NewService(gw, today).Forecast(context.Background(), domain.Ledger{Accounts: []domain.Account{
		{Name: "assets:checking", Source: domain.SourceManual},
	}})

	milestones := got.Charts[0].Lines[0].Milestones
	if milestones[0].Mantissa != 0 || milestones[3].Mantissa != 700 {
		t.Errorf("milestones = %+v", milestones)
	}
}

func TestForecastReportsTheCoveredSpan(t *testing.T) {
	got, err := NewService(&fakeGateway{}, today).Forecast(context.Background(), ledger())

	if err != nil || !got.Today.Equal(day("2026-09-30")) || !got.Begin.Equal(day("2026-03-30")) || !got.End.Equal(day("2027-09-30")) {
		t.Errorf("forecast = %+v, %v", got, err)
	}
}

func TestForecastWithNoAccountsHasNoCharts(t *testing.T) {
	got, err := NewService(&fakeGateway{}, today).Forecast(context.Background(), domain.Ledger{})

	if err != nil || len(got.Charts) != 0 {
		t.Errorf("charts = %+v, %v", got.Charts, err)
	}
}

func TestForecastReportsAGatewayFailure(t *testing.T) {
	gw := &fakeGateway{err: errors.New("hledger failed")}

	if _, err := NewService(gw, today).Forecast(context.Background(), ledger()); err == nil {
		t.Error("expected the gateway's error")
	}
}

func TestMonthsAreClampedToTheLengthOfTheMonth(t *testing.T) {
	for _, tc := range []struct {
		from   string
		months int
		want   string
	}{
		{"2026-08-31", -6, "2026-02-28"},
		{"2026-08-31", 3, "2026-11-30"},
		{"2026-01-31", 1, "2026-02-28"},
		{"2027-12-15", 3, "2028-03-15"},
		{"2028-01-31", 1, "2028-02-29"},
	} {
		if got := addMonths(day(tc.from), tc.months); !got.Equal(day(tc.want)) {
			t.Errorf("%s %+d months = %v, want %s", tc.from, tc.months, got, tc.want)
		}
	}
}

func TestAnAccountThatNeverHeldAnythingIsAFlatZeroLineInTheLedgersCurrency(t *testing.T) {
	gw := &fakeGateway{projections: map[string]domain.Projection{
		"assets:checking": months([]string{"2026-09-30"}, domain.MultiAmount{amount("EUR", 100)}),
		"assets:new":      months([]string{"2026-09-30"}, nil),
	}}
	staged := domain.Ledger{Accounts: []domain.Account{
		{Name: "assets:checking", Source: domain.SourceManual, Balance: domain.MultiAmount{amount("EUR", 100)}},
		{Name: "assets:new", Source: domain.SourceManual},
	}}

	got, err := NewService(gw, today).Forecast(context.Background(), staged)

	if err != nil || len(got.Charts) != 1 || len(got.Charts[0].Lines) != 2 {
		t.Fatalf("charts = %+v, %v", got.Charts, err)
	}
	if line := got.Charts[0].Lines[1]; line.Account != "assets:new" || line.Points[0].Balance.Mantissa != 0 ||
		line.Points[0].Balance.Style != euroStyle {
		t.Errorf("line = %+v", line)
	}
}

func TestAnEmptyLedgerGivesAnAccountWithoutBalancesNoLine(t *testing.T) {
	gw := &fakeGateway{projections: map[string]domain.Projection{"assets:new": months([]string{"2026-09-30"}, nil)}}

	got, _ := NewService(gw, today).Forecast(context.Background(), domain.Ledger{Accounts: []domain.Account{
		{Name: "assets:new", Source: domain.SourceManual},
	}})

	if len(got.Charts) != 0 {
		t.Errorf("charts = %+v", got.Charts)
	}
}
