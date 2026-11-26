// Package ledger is the use case that loads a Ledger snapshot by composing
// what hledger reports. It is read-only.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/parallel"
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

	snapshot, err := l.read(ctx)
	if err != nil {
		return domain.Ledger{}, fmt.Errorf("loading ledger: %w", err)
	}

	return snapshot, nil
}

// read is the snapshot from hledger's reports, completed by the journal's directives and periodic rules.
func (l *Loader) read(ctx context.Context) (domain.Ledger, error) {
	snapshot, accounts, err := l.fromHledger(ctx)
	if err != nil {
		return domain.Ledger{}, err
	}

	declarations, err := l.file.AccountDeclarations()
	if err != nil {
		return domain.Ledger{}, err //nolint:wrapcheck // Load wraps it
	}

	if snapshot.Rules, err = l.file.PeriodicRules(); err != nil {
		return domain.Ledger{}, err //nolint:wrapcheck // Load wraps it
	}

	if snapshot.Commodities, err = l.file.Commodities(); err != nil {
		return domain.Ledger{}, err //nolint:wrapcheck // Load wraps it
	}
	snapshot.Accounts = declare(accounts, declarations)

	return snapshot, nil
}

// fromHledger is what hledger reports: the transactions, the balances (returned apart: the directives
// complete them), the month to date, the spend history and the projected occurrences. The queries are
// independent, so they run side by side. The occurrences up to today are past, the overdue candidates; upcoming
// ones start tomorrow, unlike in a forecast, whose balance still has to count today's (forecast.start).
func (l *Loader) fromHledger(ctx context.Context) (domain.Ledger, []domain.Account, error) {
	var (
		snapshot domain.Ledger
		accounts []domain.Account
	)
	today := l.today()
	snapshot.Today = today

	err := parallel.RunAll(
		func() error { return assign(&snapshot.Transactions)(l.gw.Print(ctx)) },
		func() error { return assign(&accounts)(l.gw.Balances(ctx)) },
		func() (err error) {
			snapshot.Month, err = l.activity(ctx, today)

			return err
		},
		func() (err error) {
			snapshot.Past, snapshot.Upcoming, err = l.occurrences(ctx, today)

			return err
		},
	)
	if err != nil {
		return domain.Ledger{}, nil, err //nolint:wrapcheck // Load adds the context
	}

	return snapshot, accounts, nil
}

// occurrences are the periodic rules' occurrences of the overdue window up to today, and of the year after it.
func (l *Loader) occurrences(ctx context.Context, today time.Time) (past, upcoming []domain.Transaction, err error) {
	all, err := l.gw.Upcoming(ctx, today.AddDate(0, 0, 1-domain.OverdueDays), today.AddDate(1, 0, 0))
	past, upcoming = splitAfter(today, all)

	return past, upcoming, err //nolint:wrapcheck // Load adds the context
}

// splitAfter parts the occurrences into those up to today and those after it.
func splitAfter(today time.Time, occurrences []domain.Transaction) (past, upcoming []domain.Transaction) {
	tomorrow := time.Date(today.Year(), today.Month(), today.Day()+1, 0, 0, 0, 0, today.Location())
	for _, occurrence := range occurrences {
		if occurrence.Date.Before(tomorrow) {
			past = append(past, occurrence)
		} else {
			upcoming = append(upcoming, occurrence)
		}
	}

	return past, upcoming
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
	byName := declarationsByName(declarations)
	for i, account := range accounts {
		accounts[i] = withDeclaration(account, byName[account.Name])
	}

	return append(accounts, declaredOnly(declarations, accounts)...)
}

// declarationsByName is each declaration under its account's name.
func declarationsByName(declarations []domain.Declaration) map[string]domain.Declaration {
	byName := make(map[string]domain.Declaration, len(declarations))
	for _, declaration := range declarations {
		byName[declaration.Name] = declaration
	}

	return byName
}

// withDeclaration is the account completed by what its directive says.
func withDeclaration(account domain.Account, declaration domain.Declaration) domain.Account {
	account.Source = declaration.Source
	account.IBAN = declaration.IBAN
	account.Format = declaration.Format
	account.Closed = declaration.Closed

	return account
}

// declaredOnly are the declared accounts hledger did not list, empty, once each, in the journal's order.
func declaredOnly(declarations []domain.Declaration, listed []domain.Account) []domain.Account {
	seen := make(map[string]bool, len(listed))
	for _, account := range listed {
		seen[account.Name] = true
	}
	var accounts []domain.Account
	for _, declaration := range declarations {
		if !seen[declaration.Name] {
			seen[declaration.Name] = true
			accounts = append(accounts, withDeclaration(domain.Account{Name: declaration.Name}, declaration))
		}
	}

	return accounts
}

// activity sums the calendar month so far: income as a positive figure, and expenses.
func (l *Loader) activity(ctx context.Context, today time.Time) (domain.MonthSummary, error) {
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())

	months, err := l.gw.MonthlyActivity(ctx, thisMonth, today)
	if err != nil {
		return domain.MonthSummary{}, err //nolint:wrapcheck // Load adds the context
	}

	if len(months) != 1 {
		return domain.MonthSummary{}, fmt.Errorf("hledger reported %d months of activity, want 1", len(months))
	}

	return summarise(months[0], thisMonth), nil
}

// summarise sums income (as a positive figure) and expenses of a month's activity.
func summarise(activity []domain.Account, month time.Time) domain.MonthSummary {
	summary := domain.MonthSummary{Month: month.Month()}
	for _, account := range activity {
		switch {
		case domain.IsIncome(account.Name):
			summary.Income = summary.Income.Plus(account.Reported())
		case domain.IsExpense(account.Name):
			summary.Expenses = summary.Expenses.Plus(account.Reported())
		}
	}

	return summary
}
