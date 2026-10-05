package domain

import "time"

// NewPosting is one posting of a transaction to be written. An empty Amount is left for hledger to infer.
type NewPosting struct {
	Account   string
	Amount    MultiAmount
	Assertion MultiAmount // the balance assertion after `=`; empty for none
	Comment   string      // the posting's comment, without the semicolon
}

// NewTransaction is a transaction to be written: booking date only, no status, no code.
type NewTransaction struct {
	Date        time.Time
	Description string
	Comment     string
	Postings    []NewPosting
}

// Lines is the transaction as journal text, one entry per line.
func (t NewTransaction) Lines() []string {
	header := t.Date.Format(time.DateOnly) + " " + t.Description
	lines := make([]string, 0, 1+len(t.Postings))
	lines = append(lines, header+commentOf(t.Comment))
	for _, posting := range t.Postings {
		lines = append(lines, "    "+posting.text())
	}
	return lines
}

func (p NewPosting) text() string {
	text := p.Account
	if len(p.Amount) > 0 || len(p.Assertion) > 0 {
		text += " "
	}
	if len(p.Amount) > 0 {
		text += " " + p.Amount[0].Format()
	}
	if len(p.Assertion) > 0 {
		text += " = " + p.Assertion[0].Format()
	}
	return text + commentOf(p.Comment)
}

func commentOf(comment string) string {
	if comment == "" {
		return ""
	}
	return "  ; " + comment
}

// AddTransactions writes transactions into the year file of their date (ADR 0013), each at the position
// its date gives it, so the file stays date-ordered.
type AddTransactions struct {
	Transactions []NewTransaction
}

// Declare puts a directive on the main journal unless some file of the journal already has it.
type Declare struct {
	Directive string // "account", "payee" or "tag"
	Name      string
	Comment   string // the directive's comment, without the semicolon; empty for none
}

// SetCategoryRule adds the rule block, or replaces the block whose `konto-rule` id is ID, in the
// categories.rules file beside the journal.
type SetCategoryRule struct {
	ID   string
	Text string
}

func (Declare) isEdit()         {}
func (AddTransactions) isEdit() {}
func (SetCategoryRule) isEdit() {}
