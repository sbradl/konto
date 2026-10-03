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
	Balances(ctx context.Context) ([]domain.Account, error)
}

type journalFile interface {
	Exists() (bool, error)
	AccountDeclarations() ([]domain.Declaration, error)
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
	accounts, err := l.gw.Balances(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	declarations, err := l.file.AccountDeclarations()
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	return domain.Ledger{
		Accounts:     declare(accounts, declarations),
		Transactions: transactions,
	}, nil
}

// declare completes each account with what its `account` directive says. An account declared
// but without postings is absent from hledger's balances, so it joins with an empty balance.
func declare(accounts []domain.Account, declarations []domain.Declaration) []domain.Account {
	byName := make(map[string]domain.Declaration, len(declarations))
	for _, declaration := range declarations {
		byName[declaration.Name] = declaration
	}
	listed := make(map[string]bool, len(accounts))
	for i, account := range accounts {
		listed[account.Name] = true
		declaration := byName[account.Name]
		accounts[i].Source = declaration.Source
		accounts[i].IBAN = declaration.IBAN
		accounts[i].Closed = declaration.Closed
	}
	for _, declaration := range declarations {
		if !listed[declaration.Name] {
			listed[declaration.Name] = true
			accounts = append(accounts, domain.Account{
				Name: declaration.Name, Source: declaration.Source, IBAN: declaration.IBAN, Closed: declaration.Closed,
			})
		}
	}
	return accounts
}
