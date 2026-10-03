// Package ledger is the use case that loads a Ledger snapshot by composing
// what hledger reports. It is read-only.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type gateway interface {
	Print(ctx context.Context) ([]domain.Transaction, error)
	Balances(ctx context.Context) ([]domain.Account, error)
	PeriodActivity(ctx context.Context, first, last time.Time) ([]domain.Account, error)
	Upcoming(ctx context.Context, first, last time.Time) ([]domain.Transaction, error)
}

type journalFile interface {
	Exists() (bool, error)
	AccountDeclarations() ([]domain.Declaration, error)
}

// ErrNoJournal means there is no journal file to load yet (first run).
var ErrNoJournal = errors.New("no journal file")

// Loader builds Ledger snapshots.
type Loader struct {
	gw    gateway
	file  journalFile
	today func() time.Time
}

// NewLoader returns a Loader that reads through gw once file exists; today anchors the month to date.
func NewLoader(gw gateway, file journalFile, today func() time.Time) *Loader {
	return &Loader{gw: gw, file: file, today: today}
}

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
	month, err := l.monthToDate(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	today := l.today()
	upcoming, err := l.gw.Upcoming(ctx, today, today.AddDate(1, 0, 0))
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	return domain.Ledger{
		Accounts:     declare(accounts, declarations),
		Transactions: transactions,
		Upcoming:     upcoming,
		Month:        month,
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

// monthToDate sums the calendar month so far: income (as a positive figure) and expenses.
func (l *Loader) monthToDate(ctx context.Context) (domain.MonthSummary, error) {
	today := l.today()
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	activity, err := l.gw.PeriodActivity(ctx, first, today)
	if err != nil {
		return domain.MonthSummary{}, err //nolint:wrapcheck // Load adds the context
	}
	summary := domain.MonthSummary{Month: today.Month()}
	for _, account := range activity {
		switch {
		case strings.HasPrefix(account.Name, "income:"):
			summary.Income = summary.Income.Plus(account.Balance.Negated())
		case strings.HasPrefix(account.Name, "expenses:"):
			summary.Expenses = summary.Expenses.Plus(account.Balance)
		}
	}
	return summary, nil
}
