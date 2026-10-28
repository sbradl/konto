package domain

import "time"

// MonthSummary is what came in and went out in a calendar month to date: income and expense
// accounts only, so transfers between own accounts never show up. Income is positive.
type MonthSummary struct {
	Month    time.Month
	Income   MultiAmount
	Expenses MultiAmount
}

// Net is income less expenses.
func (s MonthSummary) Net() MultiAmount { return s.Income.Plus(s.Expenses.Negated()) }

// MonthSpend is what went out on expense accounts in one calendar month.
type MonthSpend struct {
	Month time.Time // the first day of the month
	Total MultiAmount
}

// Ledger is a point-in-time snapshot of everything konto has loaded.
type Ledger struct {
	Accounts     []Account
	Transactions []Transaction
	Upcoming     []Transaction // the periodic rules' occurrences for the next year, soonest first; never in the journal
	Month        MonthSummary
	Rules        []PeriodicRule // every `~` block of the journal, in file order
	Commodities  []string       // the currencies the journal declares, in order
	Spend        []MonthSpend   // the last six complete months, oldest first
}

// TransactionCount is the number of transactions in the journal.
func (l Ledger) TransactionCount() int { return len(l.Transactions) }

// NetWorth is the sum of every open asset and liability account, the clearing account included.
func (l Ledger) NetWorth() MultiAmount {
	var total MultiAmount
	for _, account := range l.Accounts {
		if account.IsClassifiable() && !account.IsClosed() {
			total = total.Plus(account.Balance)
		}
	}
	return total
}

// Zero is a zero amount in the first commodity and style the ledger's balances use, for figures
// that came to nothing; nil when no account has a balance.
func (l Ledger) Zero() MultiAmount {
	for _, account := range l.Accounts {
		if len(account.Balance) > 0 {
			first := account.Balance[0]
			return MultiAmount{{Commodity: first.Commodity, Places: first.Places, Style: first.Style}}
		}
	}
	return nil
}

// MonthIncome is the month's income, a zero if there was none.
func (l Ledger) MonthIncome() MultiAmount { return l.orZero(l.Month.Income) }

// MonthExpenses is the month's expenses, a zero if there were none.
func (l Ledger) MonthExpenses() MultiAmount { return l.orZero(l.Month.Expenses) }

// MonthNet is the month's income less expenses, a zero if nothing happened.
func (l Ledger) MonthNet() MultiAmount { return l.orZero(l.Month.Net()) }

func (l Ledger) orZero(amounts MultiAmount) MultiAmount {
	if len(amounts) == 0 {
		return l.Zero()
	}
	return amounts
}

// UnclassifiedIssues is konto's own check: one WARN per asset or liability without a source.
func (l Ledger) UnclassifiedIssues() []CheckIssue {
	var issues []CheckIssue
	for _, account := range l.Accounts {
		if account.IsUnclassified() {
			issues = append(issues, CheckIssue{
				Check:    UnclassifiedCheck,
				Severity: SeverityWarn,
				Message:  account.Name + " has no konto:source tag",
			})
		}
	}
	return issues
}

// Watermark is the date of the account's latest transaction (ADR 0009), the carry left out: another account's
// import can write it. An account with nothing but the carry in the loaded years takes the carry's date, as
// the carry stands in for its older rows. False when it has neither.
func (l Ledger) Watermark(account string) (time.Time, bool) {
	var latest, carried time.Time
	found, carry := false, false
	for i := range l.Transactions {
		transaction := &l.Transactions[i]
		for _, posting := range transaction.Postings {
			if posting.Account != account {
				continue
			}
			if transaction.IsCarry() {
				carried, carry = laterOf(carried, transaction.Date), true
			} else {
				latest, found = laterOf(latest, transaction.Date), true
			}
		}
	}
	if !found {
		return carried, carry
	}
	return latest, true
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

const defaultGroupSize = 3 // digits per group of thousands

// AmountLike is a zero in the commodity written the way the journal shows it: the style and the places of an
// account holding it, else the default.
func (l Ledger) AmountLike(commodity string) Amount {
	for _, account := range l.Accounts {
		for _, amount := range account.Balance {
			if amount.Commodity == commodity {
				return Amount{Commodity: commodity, Places: amount.Places, Style: amount.Style}
			}
		}
	}
	return DefaultAmount(commodity)
}

// DefaultAmount is a zero in the commodity in the plain comma-decimal style with two places, for when the
// journal has no amount to take a style from.
func DefaultAmount(commodity string) Amount {
	return Amount{Commodity: commodity, Places: 2, Style: AmountStyle{
		Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: defaultGroupSize,
	}}
}
