// Package forecast is the use case behind the Forecast view: each open asset and liability account's balance
// over the six months behind and the twelve ahead, the future as the periodic rules make it (spec §4.10,
// ADR 0007). It estimates nothing; hledger does all the projecting.
package forecast

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/parallel"
)

// Months of history and of forecast the chart covers.
const (
	monthsBack  = 6
	monthsAhead = 12
)

// Milestones are the months from today whose balances the table shows.
func Milestones() []int { return []int{0, 3, 6, 12} }

type gateway interface {
	Projection(ctx context.Context, query domain.ProjectionQuery) (domain.Projection, error)
}

// Point is an account's balance in one currency at the end of a month.
type Point struct {
	End     time.Time
	Balance domain.Amount
}

// Line is one account's course in one currency.
type Line struct {
	Account    string
	Rank       int // the account's place among the forecast accounts, by name: its colour in every chart
	Points     []Point
	Milestones []domain.Amount // the balance at each of Milestones()
}

// Chart is every account's line in one currency; the currencies are never converted.
type Chart struct {
	Commodity string
	Lines     []Line
}

// Forecast is what the view shows.
type Forecast struct {
	Today  time.Time
	Begin  time.Time // the first day of the chart
	End    time.Time // the last day of the chart
	Charts []Chart
}

// Service builds forecasts.
type Service struct {
	gw    gateway
	today func() time.Time
}

// NewService returns a Service that projects through gw, with today as read from the clock today.
func NewService(gw gateway, today func() time.Time) *Service { return &Service{gw: gw, today: today} }

// Forecast projects every open asset and liability account of the ledger, the clearing account left out. The
// projections are independent reads, so hledger is asked about all the accounts at once.
func (s *Service) Forecast(ctx context.Context, ledger domain.Ledger) (Forecast, error) {
	today := s.today()
	forecast := Forecast{Today: today, Begin: addMonths(today, -monthsBack), End: addMonths(today, monthsAhead)}
	accounts := forecastable(ledger)

	projections, err := s.project(ctx, ledger, accounts, forecast)
	if err != nil {
		return Forecast{}, err
	}

	for rank := range accounts {
		lines := linesOf(accountProjection{accounts[rank].Name, rank, projections[rank], ledger.Zero()}, today)
		forecast.Charts = addLines(forecast.Charts, lines)
	}

	return forecast, nil
}

// project asks hledger for the projection of each account over the forecast's span, all at once.
func (s *Service) project(
	ctx context.Context, ledger domain.Ledger, accounts []domain.Account, forecast Forecast,
) ([]domain.Projection, error) {
	projections := make([]domain.Projection, len(accounts))
	projects := make([]func() error, len(accounts))
	for i, account := range accounts {
		projects[i] = func() (err error) {
			projections[i], err = s.gw.Projection(ctx, domain.ProjectionQuery{
				Account: account.Name, Begin: forecast.Begin, ForecastTo: forecast.End,
				ForecastFrom: start(account, ledger, forecast.Today),
			})
			if err != nil {
				return fmt.Errorf("projecting %s: %w", account.Name, err)
			}

			return nil
		}
	}

	return projections, parallel.RunAll(projects...) //nolint:wrapcheck // each projection names its account
}

// forecastable are the open asset and liability accounts by name; the transfer clearing account is not forecast.
func forecastable(ledger domain.Ledger) []domain.Account {
	accounts := slices.DeleteFunc(slices.Clone(ledger.Accounts), func(account domain.Account) bool {
		return !account.IsClassifiable() || account.IsClosed() || account.Source == domain.SourceClearing
	})
	slices.SortFunc(accounts, func(a, b domain.Account) int { return strings.Compare(a.Name, b.Name) })

	return accounts
}

// start is the day the periodic rules' occurrences count from: the day after the latest transaction of an
// import-fed account, since its balance is only as fresh as the last import, and today for any other: the
// rules never post, so today's occurrence is in no balance yet (the Upcoming list, by contrast, starts tomorrow).
func start(account domain.Account, ledger domain.Ledger, today time.Time) time.Time {
	if account.Source == domain.SourceImport {
		if watermark, ok := ledger.Watermark(account.Name); ok {
			return watermark.AddDate(0, 0, 1)
		}
	}

	return today
}

// addMonths is the date months later, or earlier, on the same day of the month as far as the month has one.
func addMonths(date time.Time, months int) time.Time {
	first := time.Date(date.Year(), date.Month()+time.Month(months), 1, 0, 0, 0, 0, date.Location())
	lastDay := first.AddDate(0, 1, -1).Day()

	return time.Date(first.Year(), first.Month(), min(date.Day(), lastDay), 0, 0, 0, 0, date.Location())
}

// namedLine is a line and the currency it is in.
type namedLine struct {
	commodity string
	line      Line
}

// accountProjection is one account's projection, with the ledger's zero for an account that never held anything.
type accountProjection struct {
	account    string
	rank       int
	projection domain.Projection
	ledgerZero domain.MultiAmount
}

// linesOf splits the projection by currency: one line for each currency the account ever holds. An account
// that never held anything is a flat line in the ledger's own currency.
func linesOf(held accountProjection, today time.Time) []namedLine {
	zeros := zerosHeld(held.projection)
	if len(zeros) == 0 && len(held.ledgerZero) > 0 {
		zeros = []domain.Amount{held.ledgerZero[0]}
	}
	lines := make([]namedLine, 0, len(zeros))
	for _, zero := range zeros {
		lines = append(lines, namedLine{commodity: zero.Commodity, line: held.lineIn(zero, today)})
	}

	return lines
}

// lineIn is the account's line in the currency of zero, which stands for any month it holds none of it.
func (held accountProjection) lineIn(zero domain.Amount, today time.Time) Line {
	points := pointsIn(held.projection, zero)

	return Line{Account: held.account, Rank: held.rank, Points: points, Milestones: milestonesOf(points, zero, today)}
}

// pointsIn is the balance in the currency of zero at the end of each month of the projection.
func pointsIn(projection domain.Projection, zero domain.Amount) []Point {
	points := make([]Point, len(projection.Ends))
	for i, end := range projection.Ends {
		points[i] = Point{End: end, Balance: balanceIn(projection.Balances[i], zero.Commodity, zero)}
	}

	return points
}

// milestonesOf is the balance at each of Milestones(), counted in months from today.
func milestonesOf(points []Point, zero domain.Amount, today time.Time) []domain.Amount {
	milestones := make([]domain.Amount, 0, len(Milestones()))
	for _, months := range Milestones() {
		milestones = append(milestones, balanceAt(points, addMonths(today, months), zero))
	}

	return milestones
}

// zerosHeld is a zero for each currency the projection ever holds, in its style, in the order they first show up.
func zerosHeld(projection domain.Projection) []domain.Amount {
	var zeros []domain.Amount
	seen := map[string]bool{}
	for _, balance := range projection.Balances {
		for _, amount := range balance {
			if !seen[amount.Commodity] {
				seen[amount.Commodity] = true
				zeros = append(zeros, amount.Zero())
			}
		}
	}

	return zeros
}

// balanceIn is the part of the balance in the currency, or zero when it holds none.
func balanceIn(balance domain.MultiAmount, commodity string, zero domain.Amount) domain.Amount {
	for _, amount := range balance {
		if amount.Commodity == commodity {
			return amount
		}
	}

	return zero
}

// balanceAt is the balance at the end of the last month that ends on or before the date.
func balanceAt(points []Point, date time.Time, none domain.Amount) domain.Amount {
	balance := none
	for _, point := range points {
		if !point.End.After(date) {
			balance = point.Balance
		}
	}

	return balance
}

// addLines puts each line in the chart of its currency, opening the chart when it is the first of its currency.
func addLines(charts []Chart, lines []namedLine) []Chart {
	for _, named := range lines {
		i := slices.IndexFunc(charts, func(chart Chart) bool { return chart.Commodity == named.commodity })
		if i < 0 {
			charts = append(charts, Chart{Commodity: named.commodity})
			i = len(charts) - 1
		}
		charts[i].Lines = append(charts[i].Lines, named.line)
	}

	return charts
}
