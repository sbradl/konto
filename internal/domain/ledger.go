package domain

// Ledger is a point-in-time snapshot of everything konto has loaded.
type Ledger struct {
	Accounts     []Account
	Transactions []Transaction
}

// TransactionCount is the number of transactions in the journal.
func (l Ledger) TransactionCount() int { return len(l.Transactions) }

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
