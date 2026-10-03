package csvimport

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// ClearingAccount is the transfer clearing account when the journal does not name one.
const ClearingAccount = "assets:transfers"

// field is what the row says in the named export column.
func (r Row) field(name string) string {
	switch name {
	case "payee":
		return r.Payee
	case "type":
		return r.Type
	case "purpose":
		return r.Purpose
	}
	return ""
}

// MatchField is the column a rule for this row tests by default: the payee, else the transaction type.
func (r Row) MatchField() string {
	if r.Payee == "" {
		return "type"
	}
	return "payee"
}

// IsTransfer reports a row posted against an asset or liability account instead of a category.
func (r Row) IsTransfer() bool {
	return strings.HasPrefix(r.Target, "assets:") || strings.HasPrefix(r.Target, "liabilities:")
}

// Candidate is an account a row can be posted to. Target is where the posting really goes: the clearing
// account stands in for an import-fed account, which imports its own leg of a transfer.
type Candidate struct {
	Name     string
	Target   string
	Transfer bool
}

func isCategory(name string) bool {
	return strings.HasPrefix(name, "expenses:") || strings.HasPrefix(name, "income:")
}

func isBalanceAccount(name string) bool {
	return strings.HasPrefix(name, "assets:") || strings.HasPrefix(name, "liabilities:")
}

// clearing is the clearing account's name.
func (s Staged) clearing() string {
	for _, account := range s.Known {
		if account.Source == domain.SourceClearing {
			return account.Name
		}
	}
	return ClearingAccount
}

// Candidates are the accounts the row can go to, best first: the categories of its direction, the
// categories of the other direction (refunds), then the other open accounts as transfer targets.
func (s Staged) Candidates(row Row) []Candidate {
	inflow := !row.Amount.IsAllNegative()
	first, second := "expenses:", "income:"
	if inflow {
		first, second = second, first
	}
	var groups [3][]Candidate
	for _, name := range s.categoryNames() {
		switch {
		case strings.HasPrefix(name, first):
			groups[0] = append(groups[0], Candidate{Name: name, Target: name})
		case strings.HasPrefix(name, second):
			groups[1] = append(groups[1], Candidate{Name: name, Target: name})
		}
	}
	for _, account := range s.Known {
		if candidate, ok := s.transferTo(account); ok {
			groups[2] = append(groups[2], candidate)
		}
	}
	var all []Candidate
	for _, group := range groups {
		slices.SortFunc(group, func(a, b Candidate) int { return cmp.Compare(a.Name, b.Name) })
		all = append(all, group...)
	}
	return all
}

// categoryNames are the income and expense accounts there are, declared ones and those staged.
func (s Staged) categoryNames() []string {
	var names []string
	for _, account := range s.Known {
		if isCategory(account.Name) && !account.IsClosed() {
			names = append(names, account.Name)
		}
	}
	for _, name := range s.NewAccounts {
		if isCategory(name) {
			names = append(names, name)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// transferTo is the account as a transfer target; false for the account being imported, closed accounts,
// the clearing account and anything that is no asset or liability.
func (s Staged) transferTo(account domain.Account) (Candidate, bool) {
	if !isBalanceAccount(account.Name) || account.IsClosed() || account.Source == domain.SourceClearing ||
		account.Name == s.Account.Name {
		return Candidate{}, false
	}
	target := account.Name
	if account.Source == domain.SourceImport {
		target = s.clearing()
	}
	return Candidate{Name: account.Name, Target: target, Transfer: true}, true
}

// Matching are the candidates whose name contains the filter, case-insensitively.
func Matching(candidates []Candidate, filter string) []Candidate {
	needle := strings.ToLower(filter)
	return slices.DeleteFunc(slices.Clone(candidates), func(c Candidate) bool {
		return !strings.Contains(strings.ToLower(c.Name), needle)
	})
}

// SuggestName is the account a new category is declared as: the typed name, prefixed from the direction
// of the row when it names no account type.
func SuggestName(row Row, typed string) string {
	typed = strings.TrimSpace(typed)
	if strings.Contains(typed, ":") || typed == "" {
		return typed
	}
	if row.Amount.IsAllNegative() {
		return "expenses:" + typed
	}
	return "income:" + typed
}

// leaf is the last part of an account name.
func leaf(name string) string { return name[strings.LastIndex(name, ":")+1:] }

// Create stages a new category; the name must not end in a leaf some account already has (hledger's
// uniqueleafnames), and an empty name is no name.
func (s Staged) Create(name string) (Staged, Candidate, error) {
	if name == "" {
		return s, Candidate{}, domain.RefusalError{Reason: "the category needs a name"}
	}
	for _, existing := range s.allNames() {
		if leaf(existing) == leaf(name) {
			return s, Candidate{}, domain.RefusalError{
				Reason: "the name " + leaf(name) + " is already used by " + existing,
			}
		}
	}
	next := s
	next.NewAccounts = append(slices.Clone(s.NewAccounts), name)
	return next, Candidate{Name: name, Target: name}, nil
}

func (s Staged) allNames() []string {
	names := slices.Clone(s.NewAccounts)
	for _, account := range s.Known {
		names = append(names, account.Name)
	}
	return names
}

// Once puts a category on one row and nothing else: no rule.
func (s Staged) Once(index int, candidate Candidate) Staged {
	next := s
	next.Rows = slices.Clone(s.Rows)
	next.Rows[index].Target, next.Rows[index].RuleID, next.Rows[index].Once = candidate.Target, "", true
	return next
}

// NextRuleID is the id a rule staged now gets.
func (s Staged) NextRuleID() string { return nextRuleID(s.Rules) }

// Draft is a rule being written: what to match and the extra condition on the purpose.
type Draft struct {
	Field   string
	Pattern string
	Purpose string
}

// DraftFor starts a rule for the row: its payee, else its transaction type, as an escaped literal.
func (s Staged) DraftFor(index int) Draft {
	row := s.Rows[index]
	return Draft{Field: row.MatchField(), Pattern: regexp.QuoteMeta(row.field(row.MatchField()))}
}

// Alternative is the draft as the conditions of one alternative of a rule.
func (d Draft) Alternative() []Condition {
	conditions := []Condition{{Field: d.Field, Pattern: d.Pattern}}
	if d.Purpose != "" {
		conditions = append(conditions, Condition{Field: "purpose", Pattern: d.Purpose})
	}
	return conditions
}

// Rule is the draft as a rule of the given id and category.
func (d Draft) Rule(id, category string) Rule {
	return Rule{ID: id, Category: category, Alternatives: [][]Condition{d.Alternative()}, Staged: true}
}

// Impact is what a rule would do to the rows of this file.
type Impact struct {
	Matches int      // rows it matches
	Takes   int      // of those, rows another rule categorises
	From    []string // the other rules' ids, in order
}

// Impact tells how many rows the rule matches and how many it would take from other rules.
func (s Staged) Impact(rule Rule) Impact {
	var impact Impact
	for i := range s.Rows {
		row := &s.Rows[i]
		if !rule.Matches(*row) {
			continue
		}
		impact.Matches++
		if row.RuleID != "" && row.RuleID != rule.ID {
			impact.Takes++
			if !slices.Contains(impact.From, row.RuleID) {
				impact.From = append(impact.From, row.RuleID)
			}
		}
	}
	return impact
}

// checkPattern refuses a rule whose newest alternative has no pattern, or one that is no expression.
func checkPattern(rule Rule) error {
	for _, condition := range rule.Alternatives[len(rule.Alternatives)-1] {
		if condition.Pattern == "" {
			return domain.RefusalError{Reason: "the pattern is required"}
		}
		if _, err := regexp.Compile(condition.Pattern); err != nil {
			return domain.RefusalError{Reason: "the pattern is not a valid expression"}
		}
	}
	return nil
}

// checkOverlap refuses a rule that would take rows another rule categorises.
func (s Staged) checkOverlap(rule Rule) error {
	if impact := s.Impact(rule); impact.Takes > 0 {
		return domain.RefusalError{Reason: "rule " + impact.From[0] + " already categorises rows this pattern matches"}
	}
	return nil
}

// AddRule stages a new rule and categorises every row it matches that no rule has claimed.
func (s Staged) AddRule(rule Rule) (Staged, error) {
	if err := checkPattern(rule); err != nil {
		return s, err
	}
	if rule.Category == "" {
		return s, domain.RefusalError{Reason: "choose a category"}
	}
	if err := s.checkOverlap(rule); err != nil {
		return s, err
	}
	next := s
	next.Rules = append(slices.Clone(s.Rules), rule)
	return next.apply(rule), nil
}

// Widen adds an alternative to a rule, which then claims the rows it now matches.
func (s Staged) Widen(id string, alternative []Condition) (Staged, error) {
	index := slices.IndexFunc(s.Rules, func(r Rule) bool { return r.ID == id })
	if index < 0 {
		return s, domain.RefusalError{Reason: "rule " + id + " does not exist"}
	}
	rule := s.Rules[index]
	rule.Alternatives = append(slices.Clone(rule.Alternatives), alternative)
	rule.Staged = true
	if err := checkPattern(rule); err != nil {
		return s, err
	}
	if err := s.checkOverlap(rule); err != nil {
		return s, err
	}
	next := s
	next.Rules = slices.Clone(s.Rules)
	next.Rules[index] = rule
	return next.apply(rule), nil
}

// Widened is the rule as it would read after adding the alternative.
func (s Staged) Widened(id string, alternative []Condition) (Rule, bool) {
	index := slices.IndexFunc(s.Rules, func(r Rule) bool { return r.ID == id })
	if index < 0 {
		return Rule{}, false
	}
	rule := s.Rules[index]
	rule.Alternatives = append(slices.Clone(rule.Alternatives), alternative)
	return rule, true
}

// apply gives the rule's category to every row it matches that no rule or choice has claimed.
func (s Staged) apply(rule Rule) Staged {
	rows := slices.Clone(s.Rows)
	for i := range rows {
		if rows[i].Target == "" && rule.Matches(rows[i]) {
			rows[i].Target, rows[i].RuleID = rule.Category, rule.ID
		}
	}
	s.Rows = rows
	return s
}

// NetIncomeChange is what the categories chosen here would add to net income: the rows posted to income
// and expense accounts by a choice made once or a rule this import stages. A transfer changes none.
func (s Staged) NetIncomeChange() domain.MultiAmount {
	staged := map[string]bool{}
	for _, rule := range s.StagedRules() {
		staged[rule.ID] = true
	}
	total := s.Zero
	for i := range s.Rows {
		if row := &s.Rows[i]; isCategory(row.Target) && (row.Once || staged[row.RuleID]) {
			total = total.Plus(row.Amount)
		}
	}
	return total
}

// StagedRules are the rules this import adds or changes.
func (s Staged) StagedRules() []Rule {
	return slices.DeleteFunc(slices.Clone(s.Rules), func(r Rule) bool { return !r.Staged })
}
