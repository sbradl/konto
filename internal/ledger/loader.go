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
	PeriodicRules() ([]domain.PeriodicRule, error)
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
	snapshot.Accounts = declare(accounts, declarations)
	return snapshot, nil
}

// fromHledger is what hledger reports: the transactions, the balances (returned apart: the directives
// complete them), the month to date, the spend history and the projected occurrences.
func (l *Loader) fromHledger(ctx context.Context) (domain.Ledger, []domain.Account, error) {
	var snapshot domain.Ledger
	transactions, err := l.gw.Print(ctx)
	if err != nil {
		return snapshot, nil, err //nolint:wrapcheck // Load adds the context
	}
	accounts, err := l.gw.Balances(ctx)
	if err != nil {
		return snapshot, nil, err //nolint:wrapcheck // Load adds the context
	}
	month, err := l.monthToDate(ctx)
	if err != nil {
		return snapshot, nil, err
	}
	spend, err := l.spendHistory(ctx)
	if err != nil {
		return snapshot, nil, err
	}
	today := l.today()
	upcoming, err := l.gw.Upcoming(ctx, today.AddDate(0, 0, 1), today.AddDate(1, 0, 0))
	if err != nil {
		return snapshot, nil, err //nolint:wrapcheck // Load adds the context
	}
	return domain.Ledger{Transactions: transactions, Month: month, Spend: spend, Upcoming: upcoming}, accounts, nil
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

// monthToDate sums the calendar month so far: income (as a positive figure) and expenses.
func (l *Loader) monthToDate(ctx context.Context) (domain.MonthSummary, error) {
	today := l.today()
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	return l.summarise(ctx, first, today)
}

// spendHistory totals the expenses of each of the six complete months before today's, oldest first.
func (l *Loader) spendHistory(ctx context.Context) ([]domain.MonthSpend, error) {
	today := l.today()
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	history := make([]domain.MonthSpend, 0, spendMonths)
	for back := spendMonths; back >= 1; back-- {
		first := thisMonth.AddDate(0, -back, 0)
		summary, err := l.summarise(ctx, first, thisMonth.AddDate(0, -back+1, -1))
		if err != nil {
			return nil, err
		}
		history = append(history, domain.MonthSpend{Month: first, Total: summary.Expenses})
	}
	return history, nil
}

// summarise sums income (as a positive figure) and expenses from first through last.
func (l *Loader) summarise(ctx context.Context, first, last time.Time) (domain.MonthSummary, error) {
	activity, err := l.gw.PeriodActivity(ctx, first, last)
	if err != nil {
		return domain.MonthSummary{}, err //nolint:wrapcheck // Load adds the context
	}
	summary := domain.MonthSummary{Month: first.Month()}
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
