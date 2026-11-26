package hledger

import (
	"context"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

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
