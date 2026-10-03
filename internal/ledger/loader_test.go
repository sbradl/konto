package ledger

import (
	"context"
	"errors"
	"reflect"
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
}

func (f fakeGateway) Print(context.Context) ([]domain.Transaction, error) {
	return f.transactions, f.err
}

func (f fakeGateway) Balances(context.Context) ([]domain.Account, error) {
	return f.accounts, f.balancesErr
}

type fakeFile struct {
	exists       bool
	err          error
	declarations []domain.Declaration
	declareErr   error
}

func (f fakeFile) Exists() (bool, error) { return f.exists, f.err }

func (f fakeFile) AccountDeclarations() ([]domain.Declaration, error) {
	return f.declarations, f.declareErr
}

var present = fakeFile{exists: true}

// Loading assembles a Ledger snapshot from what hledger reports.
func TestLoadReturnsSnapshotOfTheJournal(t *testing.T) {
	transactions := []domain.Transaction{{Description: "Rent"}, {Description: "Salary"}}

	got, err := NewLoader(fakeGateway{transactions: transactions}, present).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TransactionCount() != 2 {
		t.Errorf("TransactionCount = %d, want 2", got.TransactionCount())
	}
}

// A failing hledger call fails the load, with context.
func TestLoadReportsGatewayFailure(t *testing.T) {
	_, err := NewLoader(fakeGateway{err: errors.New("exit status 1")}, present).Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}

// A journal that is not on disk is reported as missing, without asking hledger.
func TestLoadReportsMissingJournal(t *testing.T) {
	gw := fakeGateway{err: errors.New("hledger must not be called")}

	_, err := NewLoader(gw, fakeFile{exists: false}).Load(context.Background())

	if !errors.Is(err, ErrNoJournal) {
		t.Errorf("error = %v, want ErrNoJournal", err)
	}
}

// A journal file that cannot be inspected fails the load.
func TestLoadReportsUnreadableJournal(t *testing.T) {
	_, err := NewLoader(fakeGateway{}, fakeFile{err: errors.New("permission denied")}).Load(context.Background())

	if err == nil || errors.Is(err, ErrNoJournal) || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}

// A journal hledger cannot read keeps its located issue for the parse-error screen.
func TestLoadKeepsTheJournalError(t *testing.T) {
	issue := domain.CheckIssue{Check: "parseable", Severity: domain.SeverityErr, File: "j", Line: 1, Col: 1}

	_, err := NewLoader(fakeGateway{err: domain.JournalError{Issue: issue}}, present).Load(context.Background())

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
		{Name: "assets:checking", Source: domain.SourceImport, IBAN: "DE12"},
		{Name: "assets:old", Source: domain.SourceImport, Closed: closed},
		{Name: "assets:new", Source: domain.SourceManual},
		{Name: "assets:new", Source: domain.SourceManual}, // a repeated directive adds nothing
	}}

	got, err := NewLoader(gw, file).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []domain.Account{
		{Name: "assets:checking", Source: domain.SourceImport, IBAN: "DE12"},
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
		"balances":   NewLoader(fakeGateway{balancesErr: errors.New("bal failed")}, present),
		"directives": NewLoader(fakeGateway{}, fakeFile{exists: true, declareErr: errors.New("read failed")}),
	} {
		if _, err := loader.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "failed") {
			t.Errorf("%s: expected an error carrying the cause, got: %v", name, err)
		}
	}
}
