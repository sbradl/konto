package domain

import "time"

// Names of the managed carry transactions (ADR 0013).
const (
	CarryTag           = "konto:carry"
	CarryAccount       = "equity:opening-balances"
	ClosingDescription = "closing balance"
	OpeningDescription = "opening balance"
)

// SetCarry rewrites the managed carry into Year: the closing transaction at the end of the year file before it
// and the opening transaction at the start of Year's file, both from the balances the accounts have at the end
// of the earlier year. No balances leave neither transaction in place.
type SetCarry struct {
	Year     int
	Balances []Account
}

func (SetCarry) isEdit() {}

// Closing is the transaction that brings the balances to zero on date.
func (c SetCarry) Closing(date time.Time) NewTransaction {
	return c.transaction(date, ClosingDescription, MultiAmount.Negated)
}

// Opening is the transaction that gives the balances back on date.
func (c SetCarry) Opening(date time.Time) NewTransaction {
	return c.transaction(date, OpeningDescription, asBalanced)
}

func asBalanced(m MultiAmount) MultiAmount { return m }

func (c SetCarry) transaction(date time.Time, description string, signed func(MultiAmount) MultiAmount) NewTransaction {
	var postings []NewPosting
	for _, account := range c.Balances {
		for _, amount := range signed(account.Balance) {
			postings = append(postings, NewPosting{Account: account.Name, Amount: MultiAmount{amount}})
		}
	}
	return NewTransaction{
		Date: date, Description: description, Comment: CarryTag,
		Postings: append(postings, NewPosting{Account: CarryAccount}),
	}
}
