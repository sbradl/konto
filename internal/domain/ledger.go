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
				Check:    "unclassified",
				Severity: SeverityWarn,
				Message:  account.Name + " has no konto:source tag",
			})
		}
	}
	return issues
}
