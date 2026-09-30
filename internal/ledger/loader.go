// Package ledger is the use case that loads a Ledger snapshot by composing
// what hledger reports. It is read-only.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/sbradl/konto/internal/domain"
)

type gateway interface {
	Print(ctx context.Context) ([]domain.Transaction, error)
}

type journalFile interface {
	Exists() (bool, error)
}

// ErrNoJournal means there is no journal file to load yet (first run).
var ErrNoJournal = errors.New("no journal file")

// Loader builds Ledger snapshots.
type Loader struct {
	gw   gateway
	file journalFile
}

// NewLoader returns a Loader that reads through gw once file exists.
func NewLoader(gw gateway, file journalFile) *Loader { return &Loader{gw: gw, file: file} }

// Load reads the journal into a fresh snapshot.
func (l *Loader) Load(ctx context.Context) (domain.Ledger, error) {
	exists, err := l.file.Exists()
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	if !exists {
		return domain.Ledger{}, ErrNoJournal
	}
	transactions, err := l.gw.Print(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	return domain.Ledger{Transactions: transactions}, nil
}
