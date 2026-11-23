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

// Ledger is a point-in-time snapshot of everything konto has loaded.
type Ledger struct {
	Accounts     []Account
	Transactions []Transaction
	Upcoming     []Transaction // the periodic rules' occurrences for the next year, soonest first; never in the journal
	Month        MonthSummary
	Rules        []PeriodicRule // every `~` block of the journal, in file order
	Commodities  []string       // the currencies the journal declares, in order
	Today        time.Time      // the day the snapshot was loaded: Upcoming starts the day after
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
			return MultiAmount{account.Balance[0].Zero()}
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
	if latest, found := l.latestDate(account, isBooking); found {
		return latest, true
	}

	return l.latestDate(account, Transaction.IsCarry)
}

func isBooking(t Transaction) bool { return !t.IsCarry() }

// latestDate is the date of the latest transaction that posts to the account and is of the kind; false when
// there is none.
func (l Ledger) latestDate(account string, ofKind func(Transaction) bool) (time.Time, bool) {
	var latest time.Time
	found := false
	for i := range l.Transactions {
		transaction := &l.Transactions[i]
		if ofKind(*transaction) && transaction.postsTo(account) {
			latest, found = laterOf(latest, transaction.Date), true
		}
	}

	return latest, found
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
				return amount.Zero()
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
