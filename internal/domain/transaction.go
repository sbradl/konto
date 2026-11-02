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
}

// Recent lists the transactions newest first (later ones first within a day). Each is seen from
// its own asset or liability account: the first such posting that is not the transfer clearing
// account gives the amount, and the posting beside it the category.
func (l Ledger) Recent() []RecentEntry {
	clearing := l.clearingAccounts()
	entries := make([]RecentEntry, 0, len(l.Transactions))
	for _, transaction := range slices.Backward(l.Transactions) {
		entries = append(entries, transaction.recentEntry(clearing))
	}
	slices.SortStableFunc(entries, func(a, b RecentEntry) int { return b.Date.Compare(a.Date) })
	return entries
}

// UpcomingEntries lists the periodic rules' projected occurrences soonest first, shaped like Recent's.
func (l Ledger) UpcomingEntries() []RecentEntry {
	clearing := l.clearingAccounts()
	entries := make([]RecentEntry, 0, len(l.Upcoming))
	for _, transaction := range l.Upcoming {
		entries = append(entries, transaction.recentEntry(clearing))
	}
	slices.SortStableFunc(entries, func(a, b RecentEntry) int { return a.Date.Compare(b.Date) })
	return entries
}

func (l Ledger) clearingAccounts() map[string]bool {
	clearing := map[string]bool{}
	for _, account := range l.Accounts {
		clearing[account.Name] = account.Source == SourceClearing
	}
	return clearing
}

func (t Transaction) recentEntry(clearing map[string]bool) RecentEntry {
	entry := RecentEntry{Date: t.Date, Description: t.Description}
	subject := slices.IndexFunc(t.Postings, func(p Posting) bool {
		return HoldsMoney(p.Account) && !clearing[p.Account]
	})
	subject = max(subject, 0)
	if len(t.Postings) == 0 {
		return entry
	}
	entry.Amount = t.Postings[subject].Amounts
	for i, posting := range t.Postings {
		if i != subject {
			entry.Category = posting.Account
			break
		}
	}
	return entry
}
