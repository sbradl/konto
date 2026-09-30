// Package ledger is the use case that loads a Ledger snapshot by composing
// what hledger reports. It is read-only.
package ledger

import (
	"context"
	"fmt"

	"github.com/sbradl/konto/internal/domain"
)

type gateway interface {
	Print(ctx context.Context) ([]domain.Transaction, error)
}

// Loader builds Ledger snapshots.
type Loader struct {
	gw gateway
}

// NewLoader returns a Loader that reads through gw.
func NewLoader(gw gateway) *Loader { return &Loader{gw: gw} }

// Load reads the journal into a fresh snapshot.
func (l *Loader) Load(ctx context.Context) (domain.Ledger, error) {
	transactions, err := l.gw.Print(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	return domain.Ledger{Transactions: transactions}, nil
}
