package hledger

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// multiPeriodReport is the part of `hledger bal -M -O json` konto reads.
type multiPeriodReport struct {
	Dates [][]struct {
		Contents string `json:"contents"`
	} `json:"prDates"`
	Rows []struct {
		Amounts [][]jsonAmount `json:"prrAmounts"`
	} `json:"prRows"`
}

// Projection returns the account's balance at the end of each month from the query's Begin on. Months from
// ForecastFrom to ForecastTo include the occurrences of the periodic rules; the forecast start is explicit
// because hledger's default starts after the latest transaction of the whole journal. A forecast that starts
// before the latest transaction would break the journal's balance assertions, which the projection has no use
// for, so they are ignored.
func (c *Client) Projection(ctx context.Context, query domain.ProjectionQuery) (domain.Projection, error) {
	out, err := c.query(ctx, "bal", "acct:^"+regexp.QuoteMeta(query.Account)+"$",
		"-M", "-H", "-E", "-I", "--layout=bare", "-b", query.Begin.Format(time.DateOnly),
		"--forecast="+query.ForecastFrom.Format(time.DateOnly)+".."+query.ForecastTo.Format(time.DateOnly),
		"-O", "json")
	if err != nil {
		return domain.Projection{}, err
	}
	var report multiPeriodReport
	if err := json.Unmarshal(out, &report); err != nil {
		return domain.Projection{}, fmt.Errorf("reading hledger bal output: %w", err)
	}
	projection := domain.Projection{
		Ends:     make([]time.Time, len(report.Dates)),
		Balances: make([]domain.MultiAmount, len(report.Dates)),
	}
	for i, period := range report.Dates {
		end, err := periodEnd(period[len(period)-1].Contents)
		if err != nil {
			return domain.Projection{}, err
		}
		projection.Ends[i] = end
	}
	if len(report.Rows) == 0 {
		return projection, nil
	}
	for i, raw := range report.Rows[0].Amounts {
		balance := make(domain.MultiAmount, len(raw))
		for j, amount := range raw {
			if balance[j], err = amount.amount(); err != nil {
				return domain.Projection{}, fmt.Errorf("reading hledger bal output: %w", err)
			}
		}
		projection.Balances[i] = balance
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
