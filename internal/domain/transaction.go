package domain

import (
	"slices"
	"time"
)

// Posting is one line of a transaction: an account and the amount it moved.
type Posting struct {
	Account string
	Amounts MultiAmount
	Tags    map[string]string // the posting's `key:value` tags; nil when it has none
}

// Transaction is one journal entry as hledger reports it.
type Transaction struct {
	Date        time.Time
	Description string
	Comment     string            // the transaction's own comment, without the semicolon
	Tags        map[string]string // the transaction's `key:value` tags, as hledger parsed them; nil when it has none
	Source      SourcePos         // where the entry is written; a projected occurrence points at its `~` line
	Postings    []Posting
}

// RecentEntry is a transaction as the Dashboard's Recent panel lists it: what moved on the
// account it belongs to, and the counterpart that says what it was for.
type RecentEntry struct {
	Date        time.Time
	Description string
	Category    string // the counterpart account: an expense or income category, or a transfer
	Amount      MultiAmount
	Balance     MultiAmount // the account's balance once the transaction is booked
	Overdue     bool        // a past occurrence of a periodic rule that nothing booked covers
}

// Recent lists the transactions newest first (later ones first within a day). Each is seen from
// its own asset or liability account: the first such posting that is not the transfer clearing
// account gives the amount, and the posting beside it the category.
func (l Ledger) Recent() []RecentEntry {
	opening := l.balances()
	for _, transaction := range l.Transactions {
		for _, posting := range transaction.Postings {
			opening[posting.Account] = opening[posting.Account].Plus(posting.Amounts.Negated())
		}
	}
	entries := l.entries(l.Transactions, opening)
	slices.Reverse(entries)

	return entries
}

// UpcomingEntries lists the periodic rules' overdue occurrences, then their projected ones, soonest first,
// shaped like Recent's.
func (l Ledger) UpcomingEntries() []RecentEntry {
	overdue := l.Overdue()
	entries := l.entries(slices.Concat(overdue, l.Upcoming), l.balances())
	for i := range overdue { // overdue dates are up to today, before every upcoming one
		entries[i].Overdue = true
	}

	return entries
}

// entries books the transactions oldest first (in their given order within a day) onto the running
// balances, each entry taking its account's balance once it is booked.
func (l Ledger) entries(transactions []Transaction, running map[string]MultiAmount) []RecentEntry {
	clearing := l.clearingAccounts()
	byDate := func(a, b Transaction) int { return a.Date.Compare(b.Date) }
	oldest := slices.SortedStableFunc(slices.Values(transactions), byDate)
	entries := make([]RecentEntry, 0, len(oldest))
	for _, transaction := range oldest {
		for _, posting := range transaction.Postings {
			running[posting.Account] = running[posting.Account].Plus(posting.Amounts)
		}
		entry, account := transaction.recentEntry(clearing)
		entry.Balance = running[account]
		entries = append(entries, entry)
	}

	return entries
}

// balances are the accounts' current balances by name.
func (l Ledger) balances() map[string]MultiAmount {
	balances := make(map[string]MultiAmount, len(l.Accounts))
	for _, account := range l.Accounts {
		balances[account.Name] = account.Balance
	}

	return balances
}

func (l Ledger) clearingAccounts() map[string]bool {
	clearing := map[string]bool{}
	for _, account := range l.Accounts {
		clearing[account.Name] = account.Source == SourceClearing
	}

	return clearing
}

// recentEntry is the transaction seen from its subject posting, and that posting's account.
func (t Transaction) recentEntry(clearing map[string]bool) (entry RecentEntry, account string) {
	entry = RecentEntry{Date: t.Date, Description: t.Description}
	if len(t.Postings) == 0 {
		return entry, ""
	}
	subject := t.subjectPosting(clearing)
	entry.Amount = t.Postings[subject].Amounts
	entry.Category = t.counterpart(subject)

	return entry, t.Postings[subject].Account
}

// subjectPosting is the index of the posting the transaction is seen from: its first asset or liability posting
// that is not a clearing account, else its first posting.
func (t Transaction) subjectPosting(clearing map[string]bool) int {
	subject := slices.IndexFunc(t.Postings, func(p Posting) bool {
		return HoldsMoney(p.Account) && !clearing[p.Account]
	})

	return max(subject, 0)
}

// counterpart is the account of the first posting other than the subject; empty when there is none.
func (t Transaction) counterpart(subject int) string {
	for i, posting := range t.Postings {
		if i != subject {
			return posting.Account
		}
	}

	return ""
}

// postsTo reports whether any posting of the transaction is to the account.
func (t Transaction) postsTo(account string) bool {
	return slices.ContainsFunc(t.Postings, func(p Posting) bool { return p.Account == account })
}
