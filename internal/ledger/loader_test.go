package ledger

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type fakeGateway struct {
	transactions []domain.Transaction
	accounts     []domain.Account
	err          error
	balancesErr  error
	months       [][]domain.Account // nil: a month without activity
	activityErr  error
	upcoming     []domain.Transaction
	upcomingErr  error
}

func (f fakeGateway) Upcoming(context.Context, time.Time, time.Time) ([]domain.Transaction, error) {
	return f.upcoming, f.upcomingErr
}

func (f fakeGateway) Print(context.Context) ([]domain.Transaction, error) {
	return f.transactions, f.err
}

func (f fakeGateway) MonthlyActivity(context.Context, time.Time, time.Time) ([][]domain.Account, error) {
	if f.months == nil && f.activityErr == nil {
		return make([][]domain.Account, 1), nil
	}

	return f.months, f.activityErr
}

// periodRecorder notes the period the loader asked the monthly report for.
type periodRecorder struct {
	fakeGateway
	first, last time.Time
}

func (p *periodRecorder) MonthlyActivity(ctx context.Context, first, last time.Time) ([][]domain.Account, error) {
	p.first, p.last = first, last

	return p.fakeGateway.MonthlyActivity(ctx, first, last)
}

var sept30 = func() time.Time { return time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC) }

func (f fakeGateway) Balances(context.Context) ([]domain.Account, error) {
	return f.accounts, f.balancesErr
}

type fakeFile struct {
	exists       bool
	err          error
	declarations []domain.Declaration
	declareErr   error
	rules        []domain.PeriodicRule
	rulesErr     error
	commodities  []string
	commodityErr error
}

func (f fakeFile) Commodities() ([]string, error) { return f.commodities, f.commodityErr }

func (f fakeFile) PeriodicRules() ([]domain.PeriodicRule, error) { return f.rules, f.rulesErr }

func (f fakeFile) Exists() (bool, error) { return f.exists, f.err }

func (f fakeFile) AccountDeclarations() ([]domain.Declaration, error) {
	return f.declarations, f.declareErr
}

var present = fakeFile{exists: true}

// Loading assembles a Ledger snapshot from what hledger reports.
func TestLoadReturnsSnapshotOfTheJournal(t *testing.T) {
	transactions := []domain.Transaction{{Description: "Rent"}, {Description: "Salary"}}

	got, err := NewLoader(fakeGateway{transactions: transactions}, present, sept30).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.TransactionCount() != 2 {
		t.Errorf("TransactionCount = %d, want 2", got.TransactionCount())
	}
}

// A failing hledger call fails the load, with context.
func TestLoadReportsGatewayFailure(t *testing.T) {
	_, err := NewLoader(fakeGateway{err: errors.New("exit status 1")}, present, sept30).Load(context.Background())

	assertCarriesCause(t, err, "exit status 1")
}

// A journal that is not on disk is reported as missing, without asking hledger.
func TestLoadReportsMissingJournal(t *testing.T) {
	gw := fakeGateway{err: errors.New("hledger must not be called")}

	_, err := NewLoader(gw, fakeFile{exists: false}, sept30).Load(context.Background())

	if !errors.Is(err, ErrNoJournal) {
		t.Errorf("error = %v, want ErrNoJournal", err)
	}
}

// A journal file that cannot be inspected fails the load.
func TestLoadReportsUnreadableJournal(t *testing.T) {
	_, err := NewLoader(fakeGateway{}, fakeFile{err: errors.New("permission denied")}, sept30).Load(context.Background())

	if err == nil || errors.Is(err, ErrNoJournal) || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}

// A journal hledger cannot read keeps its located issue for the parse-error screen.
func TestLoadKeepsTheJournalError(t *testing.T) {
	issue := domain.CheckIssue{Check: "parseable", Severity: domain.SeverityErr, File: "j", Line: 1, Col: 1}

	_, err := NewLoader(fakeGateway{err: domain.JournalError{Issue: issue}}, present, sept30).Load(context.Background())

	var got domain.JournalError
	if !errors.As(err, &got) || got.Issue != issue {
		t.Errorf("error = %v, want the JournalError back through errors.As", err)
	}
}

// Accounts come from hledger's balances, completed by the journal's account directives.
func TestLoadDeclaresAccountsFromTheirDirectives(t *testing.T) {
	closed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	gw := fakeGateway{accounts: []domain.Account{{Name: "assets:checking"}, {Name: "assets:old"}, {Name: "expenses:rent"}}}
	file := fakeFile{exists: true, declarations: []domain.Declaration{
		{Name: "assets:checking", Source: domain.SourceImport, IBAN: "DE12", Format: "postbank"},
		{Name: "assets:old", Source: domain.SourceImport, Closed: closed},
		{Name: "assets:new", Source: domain.SourceManual},
		{Name: "assets:new", Source: domain.SourceManual}, // a repeated directive adds nothing
	}}

	got, err := NewLoader(gw, file, sept30).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []domain.Account{
		{Name: "assets:checking", Source: domain.SourceImport, IBAN: "DE12", Format: "postbank"},
		{Name: "assets:old", Source: domain.SourceImport, Closed: closed},
		{Name: "expenses:rent"},
		{Name: "assets:new", Source: domain.SourceManual},
	}
	if !reflect.DeepEqual(got.Accounts, want) {
		t.Errorf("accounts = %+v, want %+v", got.Accounts, want)
	}
}

func TestLoadReportsBalanceAndDirectiveFailures(t *testing.T) {
	for name, loader := range map[string]*Loader{
		"balances":   NewLoader(fakeGateway{balancesErr: errors.New("bal failed")}, present, sept30),
		"directives": NewLoader(fakeGateway{}, fakeFile{exists: true, declareErr: errors.New("read failed")}, sept30),
	} {
		if _, err := loader.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "failed") {
			t.Errorf("%s: expected an error carrying the cause, got: %v", name, err)
		}
	}
}

func eur(mantissa int64) domain.MultiAmount {
	style := domain.DefaultAmount("EUR").Style

	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}}
}

// The month to date runs from the first of today's month through today; income is shown as a positive figure.
func TestLoadSummarisesTheMonthToDate(t *testing.T) {
	gw := &periodRecorder{}
	gw.months = [][]domain.Account{{
		{Name: "income:salary", Balance: eur(-300000)},
		{Name: "expenses:rent", Balance: eur(90000)},
		{Name: "expenses:groceries", Balance: eur(25000)},
		{Name: "assets:odd", Balance: eur(1)},
	}}

	got, err := NewLoader(gw, present, sept30).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC); !gw.first.Equal(want) || gw.last.Day() != 30 {
		t.Errorf("period = %v to %v, want 2026-09-01 through 2026-09-30", gw.first, gw.last)
	}

	if !got.Today.Equal(sept30()) {
		t.Errorf("Today = %v, want the loading day", got.Today)
	}

	month := got.Month
	if month.Month != time.September || month.Income[0].Mantissa != 300000 || month.Expenses[0].Mantissa != 115000 {
		t.Errorf("month = %+v, want September, 3.000,00 in and 1.150,00 out", month)
	}
}

func TestLoadReportsAFailureToSummariseTheMonth(t *testing.T) {
	_, err := NewLoader(fakeGateway{activityErr: errors.New("period failed")}, present, sept30).Load(context.Background())

	assertCarriesCause(t, err, "period failed")
}

// The occurrences run from the overdue window up to a year ahead: those up to today are past, the rest upcoming.
func TestLoadProjectsThePeriodicRulesFromTheOverdueWindowToNextYear(t *testing.T) {
	today := sept30()
	gw := &upcomingRecorder{fakeGateway{upcoming: []domain.Transaction{
		{Description: "Gym", Date: today}, {Description: "Rent", Date: today.AddDate(0, 0, 1)},
	}}, time.Time{}, time.Time{}}

	got, err := NewLoader(gw, present, sept30).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first, last := gw.first.Format(time.DateOnly), gw.last.Format(time.DateOnly); first != "2026-09-24" || last != "2027-09-30" {
		t.Errorf("projected %s..%s, want 2026-09-24..2027-09-30", first, last)
	}

	if len(got.Upcoming) != 1 || got.Upcoming[0].Description != "Rent" {
		t.Errorf("Upcoming = %+v, want the projected Rent", got.Upcoming)
	}

	if len(got.Past) != 1 || got.Past[0].Description != "Gym" {
		t.Errorf("Past = %+v, want today's Gym", got.Past)
	}
}

func TestLoadReportsAFailureToProjectTheRules(t *testing.T) {
	_, err := NewLoader(fakeGateway{upcomingErr: errors.New("occurrences failed")}, present, sept30).Load(context.Background())

	assertCarriesCause(t, err, "occurrences failed")
}

type upcomingRecorder struct {
	fakeGateway
	first, last time.Time
}

func (u *upcomingRecorder) Upcoming(_ context.Context, first, last time.Time) ([]domain.Transaction, error) {
	u.first, u.last = first, last

	return u.upcoming, nil
}

func TestLoadRejectsAReportWithTheWrongNumberOfMonths(t *testing.T) {
	gw := fakeGateway{months: make([][]domain.Account, 3)}

	_, err := NewLoader(gw, present, sept30).Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), "3 months") ||
		!strings.Contains(err.Error(), "want 1") {
		t.Errorf("expected an error naming the month count, got: %v", err)
	}
}

func TestLoadReadsThePeriodicRulesOfTheJournal(t *testing.T) {
	file := fakeFile{exists: true, rules: []domain.PeriodicRule{{Description: "Rent"}}}

	got, err := NewLoader(fakeGateway{}, file, sept30).Load(context.Background())
	if err != nil || len(got.Rules) != 1 || got.Rules[0].Description != "Rent" {
		t.Errorf("Rules = %+v, %v; want the journal's rule", got.Rules, err)
	}
}

func TestLoadReportsAFailureToReadTheRules(t *testing.T) {
	file := fakeFile{exists: true, rulesErr: errors.New("rules failed")}

	_, err := NewLoader(fakeGateway{}, file, sept30).Load(context.Background())

	assertCarriesCause(t, err, "rules failed")
}

func TestLoadReadsTheDeclaredCommodities(t *testing.T) {
	file := fakeFile{exists: true, commodities: []string{"EUR", "PLN"}}

	got, err := NewLoader(fakeGateway{}, file, sept30).Load(context.Background())

	if err != nil || !slices.Equal(got.Commodities, []string{"EUR", "PLN"}) {
		t.Errorf("Commodities = %v, %v", got.Commodities, err)
	}
}

func TestLoadReportsAFailureToReadTheCommodities(t *testing.T) {
	file := fakeFile{exists: true, commodityErr: errors.New("commodities failed")}

	_, err := NewLoader(fakeGateway{}, file, sept30).Load(context.Background())

	assertCarriesCause(t, err, "commodities failed")
}

// assertCarriesCause fails the test unless err is an error whose message names the cause.
func assertCarriesCause(t *testing.T, err error, cause string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), cause) {
		t.Errorf("expected an error carrying %q, got: %v", cause, err)
	}
}
