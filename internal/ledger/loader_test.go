package ledger

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

type fakeGateway struct {
	transactions []domain.Transaction
	err          error
}

func (f fakeGateway) Print(context.Context) ([]domain.Transaction, error) {
	return f.transactions, f.err
}

type fakeFile struct {
	exists bool
	err    error
}

func (f fakeFile) Exists() (bool, error) { return f.exists, f.err }

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
