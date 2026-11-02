package domain

import (
	"strings"
	"time"
)

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
	lines = append(lines, header+TrailingComment(t.Comment))
	for _, posting := range t.Postings {
		lines = append(lines, posting.Line())
	}
	return lines
}

// amountGap ends the account name: hledger reads a single space as part of the name.
const amountGap = "  "

// Line is the posting as an indented journal line, in a transaction or a `~` rule alike.
func (p NewPosting) Line() string {
	var after []string
	if len(p.Amount) > 0 {
		after = append(after, p.Amount[0].Format())
	}
	if len(p.Assertion) > 0 {
		after = append(after, "= "+p.Assertion[0].Format())
	}
	text := p.Account
	if len(after) > 0 {
		text += amountGap + strings.Join(after, " ")
	}
	return "    " + text + TrailingComment(p.Comment)
}

// TrailingComment is the comment as it ends a journal line, two spaces and a semicolon before it; nothing for
// no comment.
func TrailingComment(comment string) string {
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
	Directive string // DirectiveAccount, DirectivePayee or DirectiveTag
	Name      string
	Comment   string // the directive's comment, without the semicolon; empty for none
}

// The directives konto declares names with.
const (
	DirectiveAccount = "account"
	DirectivePayee   = "payee"
	DirectiveTag     = "tag"
)

// DeclareAccount declares the account, the comment carrying its tags.
func DeclareAccount(name, comment string) Declare {
	return Declare{Directive: DirectiveAccount, Name: name, Comment: comment}
}

// DeclarePayee declares the payee.
func DeclarePayee(name string) Declare { return Declare{Directive: DirectivePayee, Name: name} }

// DeclareTag declares the tag.
func DeclareTag(name string) Declare { return Declare{Directive: DirectiveTag, Name: name} }

// Line is the directive as the journal writes it.
func (d Declare) Line() string { return d.Directive + " " + d.Name + TrailingComment(d.Comment) }

// RuleTag is the posting tag that carries the id of the import rule that categorised a row.
const RuleTag = "konto-rule"

// SetCategoryRule adds the rule block, or replaces the block whose `konto-rule` id is ID, in the
// categories.rules file beside the journal.
type SetCategoryRule struct {
	ID   string
	Text string // the block as categories.rules spells it
}

func (Declare) isEdit()         {}
func (AddTransactions) isEdit() {}
func (SetCategoryRule) isEdit() {}
