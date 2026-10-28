// Package ledger is the use case that loads a Ledger snapshot by composing
// what hledger reports. It is read-only.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type gateway interface {
	Print(ctx context.Context) ([]domain.Transaction, error)
	Balances(ctx context.Context) ([]domain.Account, error)
	MonthlyActivity(ctx context.Context, first, last time.Time) ([][]domain.Account, error)
	Upcoming(ctx context.Context, begin, until time.Time) ([]domain.Transaction, error)
}

type journalFile interface {
	Exists() (bool, error)
	AccountDeclarations() ([]domain.Declaration, error)
	PeriodicRules() ([]domain.PeriodicRule, error)
	Commodities() ([]string, error)
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
	snapshot, accounts, err := l.fromHledger(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	declarations, err := l.file.AccountDeclarations()
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	if snapshot.Rules, err = l.file.PeriodicRules(); err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	if snapshot.Commodities, err = l.file.Commodities(); err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}
	snapshot.Accounts = declare(accounts, declarations)
	return snapshot, nil
}

// fromHledger is what hledger reports: the transactions, the balances (returned apart: the directives
// complete them), the month to date, the spend history and the projected occurrences. The queries are
// independent, so they run side by side.
func (l *Loader) fromHledger(ctx context.Context) (domain.Ledger, []domain.Account, error) {
	var (
		snapshot domain.Ledger
		accounts []domain.Account
	)
	today := l.today()
	err := domain.RunAll(
		func() error { return assign(&snapshot.Transactions)(l.gw.Print(ctx)) },
		func() error { return assign(&accounts)(l.gw.Balances(ctx)) },
		func() (err error) { snapshot.Month, snapshot.Spend, err = l.activity(ctx, today); return err },
		func() error {
			return assign(&snapshot.Upcoming)(l.gw.Upcoming(ctx, today.AddDate(0, 0, 1), today.AddDate(1, 0, 0)))
		},
	)
	if err != nil {
		return domain.Ledger{}, nil, err //nolint:wrapcheck // Load adds the context
	}
	return snapshot, accounts, nil
}

// assign stores a query's result in dst, passing its error on.
func assign[T any](dst *T) func(T, error) error {
	return func(value T, err error) error {
		*dst = value
		return err
	}
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

const spendMonths = 6

// activity sums the calendar month so far (income as a positive figure, and expenses) and totals the
// expenses of each of the six complete months before it, oldest first, from one monthly report.
func (l *Loader) activity(ctx context.Context, today time.Time) (domain.MonthSummary, []domain.MonthSpend, error) {
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	oldest := thisMonth.AddDate(0, -spendMonths, 0)
	months, err := l.gw.MonthlyActivity(ctx, oldest, today)
	if err != nil {
		return domain.MonthSummary{}, nil, err //nolint:wrapcheck // Load adds the context
	}
	if len(months) != spendMonths+1 {
		return domain.MonthSummary{}, nil, fmt.Errorf(
			"hledger reported %d months of activity, want %d", len(months), spendMonths+1)
	}
	history := make([]domain.MonthSpend, spendMonths)
	for i := range history {
		first := oldest.AddDate(0, i, 0)
		history[i] = domain.MonthSpend{Month: first, Total: summarise(months[i], first).Expenses}
	}
	return summarise(months[spendMonths], thisMonth), history, nil
}

// summarise sums income (as a positive figure) and expenses of a month's activity.
func summarise(activity []domain.Account, month time.Time) domain.MonthSummary {
	summary := domain.MonthSummary{Month: month.Month()}
	for _, account := range activity {
		switch {
		case domain.IsIncome(account.Name):
			summary.Income = summary.Income.Plus(account.Balance.Negated())
		case domain.IsExpense(account.Name):
			summary.Expenses = summary.Expenses.Plus(account.Balance)
		}
	}
	return summary
}
