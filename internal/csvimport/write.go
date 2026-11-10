package csvimport

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// ErrNothingToWrite is the refusal to write an import that has no new rows.
var ErrNothingToWrite = domain.RefusalError{Reason: "nothing new to write"}

// errUnnamedAccount is the refusal of an account without a name, which the import cannot create.
var errUnnamedAccount = domain.RefusalError{Reason: "the account needs a name"}

// BalanceMismatchError blocks a write: the journal, with the rows added, would not end at the bank's balance,
// so the export is missing rows or the journal has drifted from the bank.
type BalanceMismatchError struct {
	Bank, Journal, Difference domain.MultiAmount
}

func (e BalanceMismatchError) Error() string {
	return "write blocked — bank " + e.Bank.Format() + " · journal " + e.Journal.Format() +
		" · difference " + e.Difference.Format()
}

// Plan is what writing the import does: the changeset, the lines the user is asked to confirm and what
// it adds up to.
type Plan struct {
	Changeset domain.Changeset
	Preview   []string // the journal and rules text to be written, transactions first
	Count     int      // transactions imported, an opening balance not counted
	Account   string   // the account imported into
}

// AccountName is the account the rows go to: the one the IBAN belongs to, or the name a new account was given.
func (s Staged) AccountName() string {
	if s.NewAccount {
		return s.Named
	}

	return s.Account.Name
}

// Plan checks that the import can be written and plans the write (spec §4.9): every row categorised, the
// journal ending at the bank's balance, and a changeset that declares what is new, writes the rules and
// adds the transactions, the opening balance first when the account is new.
func (s Staged) Plan() (Plan, error) {
	if err := s.writeRefusal(); err != nil {
		return Plan{}, err
	}

	closing, err := s.closing()
	if err != nil {
		return Plan{}, err
	}

	transactions, err := s.balancedTransactions(closing)
	if err != nil {
		return Plan{}, err
	}
	changeset := domain.Changeset{Edits: slices.Concat(
		s.declarations(), []domain.Edit{domain.AddTransactions{Transactions: transactions}},
		s.ruleEdits(),
	)}

	return Plan{
		Changeset: changeset, Preview: preview(changeset), Count: len(s.Rows), Account: s.AccountName(),
	}, nil
}

// writeRefusal says why the import cannot be written yet: no rows, rows without a category, or no account name.
func (s Staged) writeRefusal() error {
	if len(s.Rows) == 0 {
		return ErrNothingToWrite
	}

	if missing := s.NeedCategory(); missing > 0 {
		return domain.RefusalError{Reason: needCategory(missing)}
	}

	if s.AccountName() == "" {
		return errUnnamedAccount
	}

	return nil
}

// balancedTransactions are the rows' transactions ending at the closing balance: a new account opens at the
// balance that gets there, a known one must already get there.
func (s Staged) balancedTransactions(closing domain.Amount) ([]domain.NewTransaction, error) {
	transactions := s.transactions(closing)
	if s.NewAccount {
		return append([]domain.NewTransaction{s.openingBalance(closing)}, transactions...), nil
	}

	if err := s.checkBalance(closing); err != nil {
		return nil, err
	}

	return transactions, nil
}

func needCategory(rows int) string {
	if rows == 1 {
		return "1 row needs a category"
	}

	return strconv.Itoa(rows) + " rows need a category"
}

// closing is the bank's closing balance in the export's currency, in the journal's style, read as the export's
// bank format prints it.
func (s Staged) closing() (domain.Amount, error) {
	format, known := formatNamed(s.Format)
	if !known {
		return domain.Amount{}, fmt.Errorf("the bank format %q is unknown", s.Format)
	}

	like := domain.DefaultAmount(format.currency)
	if len(s.Zero) > 0 {
		like = s.Zero[0]
	}

	amount, err := format.amount(like, s.Closing)
	if err != nil {
		return domain.Amount{}, fmt.Errorf("the bank's closing balance %q is unreadable: %w", s.Closing, err)
	}

	return amount, nil
}

// total is the sum of the rows' amounts.
func (s Staged) total() domain.MultiAmount {
	sum := s.Zero
	for i := range s.Rows {
		sum = sum.Plus(s.Rows[i].Amount)
	}

	return sum
}

// checkBalance blocks the write when the account's balance with the rows added is not the bank's.
func (s Staged) checkBalance(closing domain.Amount) error {
	bank := domain.MultiAmount{closing}
	journal := s.Account.Balance.Plus(s.total())

	difference := bank.Plus(journal.Negated())
	if difference.IsZero() {
		return nil
	}

	return BalanceMismatchError{Bank: bank, Journal: journal, Difference: difference}
}

// transactions are the rows as transactions, oldest first; the newest carries the closing-balance assertion.
func (s Staged) transactions(closing domain.Amount) []domain.NewTransaction {
	transactions := make([]domain.NewTransaction, 0, len(s.Rows))
	for i := len(s.Rows) - 1; i >= 0; i-- {
		row := &s.Rows[i]

		own := domain.NewPosting{Account: s.AccountName()}
		if i == 0 {
			own.Amount, own.Assertion = row.Amount, domain.MultiAmount{closing}
		}
		transactions = append(transactions, domain.NewTransaction{
			Date: row.Date, Description: row.Description, Comment: row.Purpose,
			Postings: []domain.NewPosting{row.counterPosting(), own},
		})
	}

	return transactions
}

// counterPosting is the row's posting to its category or transfer account, tagged with the rule that chose it.
func (r Row) counterPosting() domain.NewPosting {
	posting := domain.NewPosting{Account: r.Target, Amount: r.Amount.Negated()}
	if r.RuleID != "" {
		posting.Comment = domain.RuleTag + ":" + r.RuleID
	}

	return posting
}

// openingBalance is the balance the new account had the day before its oldest row, which brings the
// account to the bank's closing balance once the rows are in.
func (s Staged) openingBalance(closing domain.Amount) domain.NewTransaction {
	opening := domain.MultiAmount{closing}.Plus(s.total().Negated())
	oldest := s.Rows[len(s.Rows)-1].Date

	return domain.NewTransaction{
		Date: oldest.AddDate(0, 0, -1), Description: domain.OpeningDescription,
		Postings: []domain.NewPosting{
			{Account: s.AccountName(), Amount: opening, Assertion: opening},
			{Account: domain.OpeningBalanceAccount},
		},
	}
}

// declarations are the directives the import adds: the new account, the categories it created that are used,
// the payees and the rule tag. A directive the journal has already is left out when written.
func (s Staged) declarations() []domain.Edit {
	return slices.Concat(s.accountDeclarations(), s.payeeDeclarations(), s.ruleTagDeclaration())
}

// accountDeclarations declare the new account, tagged as import-fed, and the used categories this import created.
func (s Staged) accountDeclarations() []domain.Edit {
	var edits []domain.Edit

	if s.NewAccount {
		comment := domain.Declaration{Source: domain.SourceImport, IBAN: s.IBAN, Format: s.Format}.Comment()
		edits = append(edits, domain.DeclareAccount(s.Named, comment))
	}
	for _, name := range s.usedNewAccounts() {
		edits = append(edits, domain.DeclareAccount(name, ""))
	}

	return edits
}

// payeeDeclarations declare the rows' payees, and the opening balance's when the account is new.
func (s Staged) payeeDeclarations() []domain.Edit {
	var edits []domain.Edit
	if s.NewAccount {
		edits = append(edits, domain.DeclarePayee(domain.OpeningDescription))
	}
	for _, payee := range s.payees() {
		edits = append(edits, domain.DeclarePayee(payee))
	}

	return edits
}

// ruleTagDeclaration declares the rule tag when a posting will carry it.
func (s Staged) ruleTagDeclaration() []domain.Edit {
	if len(s.StagedRules()) > 0 || s.usesRules() {
		return []domain.Edit{domain.DeclareTag(domain.RuleTag)}
	}

	return nil
}

func (s Staged) usesRules() bool {
	return slices.ContainsFunc(s.Rows, func(row Row) bool { return row.RuleID != "" })
}

// usedNewAccounts are the categories this import created that a row or a staged rule posts to.
func (s Staged) usedNewAccounts() []string {
	used := map[string]bool{}
	for i := range s.Rows {
		used[s.Rows[i].Target] = true
	}
	for _, rule := range s.StagedRules() {
		used[rule.Category] = true
	}

	return slices.DeleteFunc(slices.Clone(s.NewAccounts), func(name string) bool { return !used[name] })
}

// payees are the payees of the rows' descriptions, each once, newest row first.
func (s Staged) payees() []string {
	var payees []string
	for i := range s.Rows {
		if payee := payeeOf(s.Rows[i].Description); !slices.Contains(payees, payee) {
			payees = append(payees, payee)
		}
	}

	return payees
}

// payeeOf is the payee hledger reads from a description: the text before a `|`, the rest being a note.
func payeeOf(description string) string {
	payee, _, _ := strings.Cut(description, "|")

	return strings.TrimSpace(payee)
}

// ruleEdits write the rules this import adds or widens.
func (s Staged) ruleEdits() []domain.Edit {
	rules := s.StagedRules()
	edits := make([]domain.Edit, 0, len(rules))
	for _, rule := range rules {
		edits = append(edits, domain.SetCategoryRule{ID: rule.ID, Text: rule.Text(), First: rule.Fallback()})
	}

	return edits
}

// preview is the text the changeset writes: the transactions newest first, then the directives and rules.
func preview(changeset domain.Changeset) []string {
	var transactions, others []string
	for _, edit := range changeset.Edits {
		switch change := edit.(type) {
		case domain.AddTransactions:
			for _, transaction := range slices.Backward(change.Transactions) {
				transactions = append(transactions, transaction.Lines()...)
			}
		case domain.Declare:
			others = append(others, change.Line())
		case domain.SetCategoryRule:
			others = append(others, strings.Split(change.Text, "\n")...)
		}
	}

	return append(transactions, others...)
}
