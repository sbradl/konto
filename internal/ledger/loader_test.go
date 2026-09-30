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

// Loading assembles a Ledger snapshot from what hledger reports.
func TestLoadReturnsSnapshotOfTheJournal(t *testing.T) {
	transactions := []domain.Transaction{{Description: "Rent"}, {Description: "Salary"}}

	got, err := NewLoader(fakeGateway{transactions: transactions}).Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TransactionCount() != 2 {
		t.Errorf("TransactionCount = %d, want 2", got.TransactionCount())
	}
}

// A failing hledger call fails the load, with context.
func TestLoadReportsGatewayFailure(t *testing.T) {
	_, err := NewLoader(fakeGateway{err: errors.New("exit status 1")}).Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}
