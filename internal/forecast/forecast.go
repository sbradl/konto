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
	projections := make([]domain.Projection, len(accounts))
	projects := make([]func() error, len(accounts))
	for i, account := range accounts {
		projects[i] = func() (err error) {
			projections[i], err = s.gw.Projection(ctx, domain.ProjectionQuery{
				Account: account.Name, Begin: forecast.Begin, ForecastTo: forecast.End,
				ForecastFrom: start(account, ledger, today),
			})
			if err != nil {
				return fmt.Errorf("projecting %s: %w", account.Name, err)
			}
			return nil
		}
	}
	if err := parallel.RunAll(projects...); err != nil {
		return Forecast{}, err //nolint:wrapcheck // each projection names its account
	}
	for rank := range accounts {
		lines := linesOf(accountProjection{accounts[rank].Name, rank, projections[rank], ledger.Zero()}, today)
		forecast.Charts = addLines(forecast.Charts, lines)
	}
	return forecast, nil
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
	projection := held.projection
	commodities, zeros := commoditiesOf(projection)
	if zero := held.ledgerZero; len(commodities) == 0 && len(zero) > 0 {
		commodities, zeros = []string{zero[0].Commodity}, map[string]domain.Amount{zero[0].Commodity: zero[0]}
	}
	lines := make([]namedLine, 0, len(commodities))
	for _, commodity := range commodities {
		line := Line{Account: held.account, Rank: held.rank, Points: make([]Point, len(projection.Ends))}
		for i, end := range projection.Ends {
			line.Points[i] = Point{End: end, Balance: balanceIn(projection.Balances[i], commodity, zeros[commodity])}
		}
		for _, months := range Milestones() {
			milestone := addMonths(today, months)
			line.Milestones = append(line.Milestones, balanceAt(line.Points, milestone, zeros[commodity]))
		}
		lines = append(lines, namedLine{commodity: commodity, line: line})
	}
	return lines
}

// commoditiesOf are the currencies the projection ever holds, in the order they first show up, each with a
// zero in its style.
func commoditiesOf(projection domain.Projection) (commodities []string, zeros map[string]domain.Amount) {
	zeros = map[string]domain.Amount{}
	for _, balance := range projection.Balances {
		for _, amount := range balance {
			if _, seen := zeros[amount.Commodity]; !seen {
				commodities = append(commodities, amount.Commodity)
				zeros[amount.Commodity] = amount.Zero()
			}
		}
	}
	return commodities, zeros
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
