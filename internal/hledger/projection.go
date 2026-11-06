package hledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// Projection returns the account's balance at the end of each month from the query's Begin on. Months from
// ForecastFrom up to, not including, ForecastTo include the occurrences of the periodic rules; the forecast
// start is explicit because hledger's default starts after the latest transaction of the whole journal. A
// forecast that starts before the latest transaction would break the journal's balance assertions, which the
// projection has no use for, so they are ignored.
func (c *Client) Projection(ctx context.Context, query domain.ProjectionQuery) (domain.Projection, error) {
	out, err := c.query(ctx, "bal", exactly(query.Account),
		"-M", "-H", "-E", "-I", "--layout=bare", "-b", day(query.Begin),
		span{query.ForecastFrom, query.ForecastTo}.forecast(), "-O", "json")
	if err != nil {
		return domain.Projection{}, err
	}

	var report multiPeriodReport
	if err := json.Unmarshal(out, &report); err != nil {
		return domain.Projection{}, fmt.Errorf("reading hledger bal output: %w", err)
	}

	return report.projection()
}

// projection is the report's one account at the end of each of its months.
func (r multiPeriodReport) projection() (domain.Projection, error) {
	months, err := r.months()
	if err != nil {
		return domain.Projection{}, err
	}

	ends, err := r.ends()
	if err != nil {
		return domain.Projection{}, err
	}

	projection := domain.Projection{Ends: ends, Balances: make([]domain.MultiAmount, len(r.Dates))}
	for i, accounts := range months {
		if len(accounts) > 0 { // the one account, absent in a month it held nothing
			projection.Balances[i] = accounts[0].Balance
		}
	}

	return projection, nil
}

// ends are the last days of the report's periods.
func (r multiPeriodReport) ends() ([]time.Time, error) {
	ends := make([]time.Time, len(r.Dates))
	for i, period := range r.Dates {
		if len(period) == 0 {
			return nil, errors.New("reading hledger bal output: a period without dates")
		}

		end, err := periodEnd(period[len(period)-1].Contents)
		if err != nil {
			return nil, err
		}
		ends[i] = end
	}

	return ends, nil
}

func periodEnd(text string) (time.Time, error) {
	end, err := time.Parse(time.DateOnly, text)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading hledger bal output: %w", err)
	}

	return end, nil
}

// Occurrences are the dates the periodic rules in the journal text give rise to from begin up to, not
// including, until, as hledger projects them. The text goes to hledger on standard input; no file is read or
// written.
func (c *Client) Occurrences(ctx context.Context, text string, begin, until time.Time) ([]time.Time, error) {
	out, err := c.queryText(ctx, text, "print", span{begin, until}.forecast(), "-O", "json")
	if err != nil {
		return nil, err
	}

	transactions, err := readTransactions(out, jsonTransaction.isGenerated)
	if err != nil {
		return nil, err
	}

	return datesOf(transactions), nil
}

// datesOf is the date of each transaction.
func datesOf(transactions []domain.Transaction) []time.Time {
	dates := make([]time.Time, len(transactions))
	for i, transaction := range transactions {
		dates[i] = transaction.Date
	}

	return dates
}
