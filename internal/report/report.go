// Package report is the use case behind the Reports view: the balance sheet and the income
// statement for a period. It is read-only.
package report

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type gateway interface {
	BalancesUntil(ctx context.Context, last time.Time, prefixes ...string) ([]domain.Account, error)
	Activity(ctx context.Context, first, last time.Time, prefixes ...string) ([]domain.Account, error)
}

// Service builds reports.
type Service struct {
	gw    gateway
	today func() time.Time
}

// NewService returns a Service that reads through gw; today is the day every period ends.
func NewService(gw gateway, today func() time.Time) *Service { return &Service{gw: gw, today: today} }

func sheetPrefixes() []string { return []string{domain.AssetsPrefix, domain.LiabilitiesPrefix} }

func statementPrefixes() []string { return []string{domain.IncomePrefix, domain.ExpensesPrefix} }

// Report builds both tables for the period, ending today.
func (s *Service) Report(ctx context.Context, period domain.ReportPeriod) (domain.Report, error) {
	today := s.today()

	balances, err := s.gw.BalancesUntil(ctx, today, sheetPrefixes()...)
	if err != nil {
		return domain.Report{}, fmt.Errorf("reading balances: %w", err)
	}

	delta, err := s.periodChanges(ctx, period, today)
	if err != nil {
		return domain.Report{}, err
	}

	activity, err := s.statementActivity(ctx, period, today)
	if err != nil {
		return domain.Report{}, fmt.Errorf("reading income and expenses: %w", err)
	}

	return domain.Report{
		Period:          period,
		BalanceSheet:    balanceSheet(balances, delta),
		IncomeStatement: incomeStatement(activity),
	}, nil
}

// periodChanges is what each asset and liability account changed by within the period; nil under all time,
// which has no start and so no Δ column.
func (s *Service) periodChanges(
	ctx context.Context, period domain.ReportPeriod, today time.Time,
) (map[string]domain.MultiAmount, error) {
	start, bounded := period.Start(today)
	if !bounded {
		return nil, nil //nolint:nilnil // a nil delta is the absent Δ column, not a failure
	}

	return s.sheetChanges(ctx, start, today)
}

// sheetChanges is what each asset and liability account changed by from start to today.
func (s *Service) sheetChanges(ctx context.Context, start, today time.Time) (map[string]domain.MultiAmount, error) {
	changes, err := s.gw.Activity(ctx, start, today, sheetPrefixes()...)
	if err != nil {
		return nil, fmt.Errorf("reading changes: %w", err)
	}
	delta := make(map[string]domain.MultiAmount, len(changes))
	for _, change := range changes {
		delta[change.Name] = change.Balance
	}

	return delta, nil
}

// statementActivity is what income and expense accounts did within the period.
func (s *Service) statementActivity(
	ctx context.Context, period domain.ReportPeriod, today time.Time,
) ([]domain.Account, error) {
	if start, bounded := period.Start(today); bounded {
		return s.gw.Activity(ctx, start, today, statementPrefixes()...) //nolint:wrapcheck // Report adds the context
	}

	return s.gw.BalancesUntil(ctx, today, statementPrefixes()...) //nolint:wrapcheck // Report adds the context
}

// section is one titled group of accounts and the total row that closes it.
type section struct {
	title, total, prefix string
}

// total is a section's sum and its change within the period.
type total struct{ amount, change domain.MultiAmount }

func sheetSections() []section {
	return []section{
		{"Assets", "Total assets", domain.AssetsPrefix}, {"Liabilities", "Total liabilities", domain.LiabilitiesPrefix},
	}
}

// balanceSheet lists open assets, then liabilities. A nil delta means the period has no start, so no Δ column.
func balanceSheet(balances []domain.Account, delta map[string]domain.MultiAmount) []domain.ReportRow {
	var rows []domain.ReportRow
	var worth total
	for _, sec := range sheetSections() {
		leaves, sum := sheetLeaves(balances, delta, sec.prefix)
		worth = total{worth.amount.Plus(sum.amount), worth.change.Plus(sum.change)}

		if len(leaves) == 0 {
			continue
		}
		rows = append(rows, domain.ReportRow{Kind: domain.RowHeader, Label: sec.title})
		rows = append(rows, leaves...)
		rows = append(rows, totalRow(domain.RowSubtotal, sec.total, sum))
	}

	if len(rows) == 0 {
		return nil
	}

	rows = append(rows, totalRow(domain.RowGrandTotal, "Net worth", worth))
	if delta == nil {
		return withoutDelta(rows)
	}

	return rows
}

// withoutDelta is the rows with the Δ column dropped.
func withoutDelta(rows []domain.ReportRow) []domain.ReportRow {
	for i := range rows {
		rows[i].Delta = nil
	}

	return rows
}

// sheetLeaves are the rows of the non-zero accounts under prefix, and the sum. The change sums every account
// under prefix: one that ends the period at zero has no row, but what it gave or took still counts.
func sheetLeaves(
	balances []domain.Account, delta map[string]domain.MultiAmount, prefix string,
) ([]domain.ReportRow, total) {
	var leaves []domain.ReportRow
	var sum total
	for _, account := range balances {
		if !strings.HasPrefix(account.Name, prefix) || account.Balance.IsZero() {
			continue
		}

		sum = total{sum.amount.Plus(account.Balance), sum.change.Plus(account.Balance.Zeroed())}
		leaves = append(leaves, leafRow(account, delta))
	}
	sum.change = sum.change.Plus(changeUnder(delta, prefix))

	return leaves, sum
}

// leafRow is one account's row; its change is zero when the period did not touch it.
func leafRow(account domain.Account, delta map[string]domain.MultiAmount) domain.ReportRow {
	row := domain.ReportRow{Kind: domain.RowLeaf, Label: account.Name, Amount: account.Balance}
	if delta == nil {
		return row
	}

	change := delta[account.Name]
	if len(change) == 0 {
		change = account.Balance.Zeroed()
	}
	row.Delta = &change

	return row
}

// changeUnder sums the change of every account under prefix.
func changeUnder(delta map[string]domain.MultiAmount, prefix string) domain.MultiAmount {
	var sum domain.MultiAmount
	for _, name := range slices.Sorted(maps.Keys(delta)) {
		if strings.HasPrefix(name, prefix) {
			sum = sum.Plus(delta[name])
		}
	}

	return sum
}

// totalRow is a section's closing row.
func totalRow(kind domain.RowKind, label string, sum total) domain.ReportRow {
	return domain.ReportRow{Kind: kind, Label: label, Amount: sum.amount, Delta: &sum.change}
}

// incomeStatement lists income, then expenses, as positive figures, and the net.
func incomeStatement(activity []domain.Account) []domain.ReportRow {
	income, incomeRows := statementSection(activity, section{"Income", "Total income", domain.IncomePrefix})
	expenses, expenseRows := statementSection(activity, section{"Expenses", "Total expenses", domain.ExpensesPrefix})

	rows := slices.Concat(incomeRows, expenseRows)
	if len(rows) == 0 {
		return nil
	}
	net := income.Plus(expenses.Negated())

	return append(rows, domain.ReportRow{Kind: domain.RowGrandTotal, Label: "Net", Amount: net})
}

// statementSection is the rows of one income-statement section, empty when it has no accounts, and its total.
func statementSection(activity []domain.Account, sec section) (domain.MultiAmount, []domain.ReportRow) {
	var sum domain.MultiAmount
	var leaves []domain.ReportRow
	for _, account := range activity {
		if !strings.HasPrefix(account.Name, sec.prefix) {
			continue
		}
		amount := account.Reported()
		sum = sum.Plus(amount)
		leaves = append(leaves, domain.ReportRow{Kind: domain.RowLeaf, Label: account.Name, Amount: amount})
	}

	if len(leaves) == 0 {
		return nil, nil
	}
	rows := append([]domain.ReportRow{{Kind: domain.RowHeader, Label: sec.title}}, leaves...)

	return sum, append(rows, domain.ReportRow{Kind: domain.RowSubtotal, Label: sec.total, Amount: sum})
}
