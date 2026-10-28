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
	months, err := report.months()
	if err != nil {
		return domain.Projection{}, err
	}
	projection := domain.Projection{
		Ends:     make([]time.Time, len(report.Dates)),
		Balances: make([]domain.MultiAmount, len(report.Dates)),
	}
	for i, period := range report.Dates {
		if len(period) == 0 {
			return domain.Projection{}, errors.New("reading hledger bal output: a period without dates")
		}
		if projection.Ends[i], err = periodEnd(period[len(period)-1].Contents); err != nil {
			return domain.Projection{}, err
		}
	}
	for i, accounts := range months {
		if len(accounts) > 0 { // the one account, absent in a month it held nothing
			projection.Balances[i] = accounts[0].Balance
		}
	}
	return projection, nil
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
	dates := make([]time.Time, len(transactions))
	for i, transaction := range transactions {
		dates[i] = transaction.Date
	}
	return dates, nil
}
