package domain

// Ledger is a point-in-time snapshot of everything konto has loaded.
type Ledger struct {
	Transactions []Transaction
}

// TransactionCount is the number of transactions in the journal.
func (l Ledger) TransactionCount() int { return len(l.Transactions) }
