package csvimport

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// field is what the row says in the named export column.
func (r Row) field(name string) string {
	switch name {
	case fieldPayee:
		return r.Payee
	case fieldType:
		return r.Type
	case fieldPurpose:
		return r.Purpose
	}

	return ""
}

// MatchField is the column a rule for this row tests by default: the payee, else the transaction type.
func (r Row) MatchField() string {
	if r.Payee == "" {
		return fieldType
	}

	return fieldPayee
}

// IsTransfer reports a row posted against an asset or liability account instead of a category.
func (r Row) IsTransfer() bool { return domain.HoldsMoney(r.Target) }

// Candidate is an account a row can be posted to. Target is where the posting really goes: the clearing
// account stands in for an import-fed account, which imports its own leg of a transfer.
type Candidate struct {
	Name     string
	Target   string
	Transfer bool
}

// clearing is the clearing account's name.
func (s Staged) clearing() string {
	for _, account := range s.Known {
		if account.Source == domain.SourceClearing {
			return account.Name
		}
	}

	return domain.ClearingAccount
}

// Candidates are the accounts the row can go to, best first: the categories of its direction, the
// categories of the other direction (refunds), then the other open accounts as transfer targets.
func (s Staged) Candidates(row Row) []Candidate {
	own, refunds := s.categoriesByDirection(row)

	return slices.Concat(byName(own), byName(refunds), byName(s.transferTargets()))
}

// categoriesByDirection splits the categories into those of the row's direction and those of the other.
func (s Staged) categoriesByDirection(row Row) (own, refunds []Candidate) {
	ownPrefix, refundPrefix := domain.ExpensesPrefix, domain.IncomePrefix
	if !row.Amount.IsAllNegative() {
		ownPrefix, refundPrefix = refundPrefix, ownPrefix
	}

	for _, name := range s.categoryNames() {
		switch {
		case strings.HasPrefix(name, ownPrefix):
			own = append(own, Candidate{Name: name, Target: name})
		case strings.HasPrefix(name, refundPrefix):
			refunds = append(refunds, Candidate{Name: name, Target: name})
		}
	}

	return own, refunds
}

// transferTargets are the open accounts the row can be a transfer to.
func (s Staged) transferTargets() []Candidate {
	var targets []Candidate
	for _, account := range s.Known {
		if candidate, ok := s.transferTo(account); ok {
			targets = append(targets, candidate)
		}
	}

	return targets
}

func byName(candidates []Candidate) []Candidate {
	slices.SortFunc(candidates, func(a, b Candidate) int { return cmp.Compare(a.Name, b.Name) })

	return candidates
}

// categoryNames are the income and expense accounts there are, declared ones and those staged.
func (s Staged) categoryNames() []string {
	var names []string
	for _, account := range s.Known {
		if domain.IsCategory(account.Name) && !account.IsClosed() {
			names = append(names, account.Name)
		}
	}
	for _, name := range s.NewAccounts {
		if domain.IsCategory(name) {
			names = append(names, name)
		}
	}

	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// transferTo is the account as a transfer target; false for the account being imported, closed accounts,
// the clearing account and anything that is no asset or liability.
func (s Staged) transferTo(account domain.Account) (Candidate, bool) {
	if !domain.HoldsMoney(account.Name) || account.IsClosed() || account.Source == domain.SourceClearing ||
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
// of the row unless it already starts with income, expenses, assets or liabilities; insurance:x is a
// subcategory under expenses, not a top level of its own.
func SuggestName(row Row, typed string) string {
	typed = strings.TrimSpace(typed)
	if domain.IsCategory(typed) || domain.HoldsMoney(typed) || typed == "" {
		return typed
	}

	if row.Amount.IsAllNegative() {
		return domain.ExpensesPrefix + typed
	}

	return domain.IncomePrefix + typed
}

// leaf is the last part of an account name.
func leaf(name string) string { return name[strings.LastIndex(name, ":")+1:] }

// Create stages a new category; the name must not end in a leaf some account already has (hledger's
// uniqueleafnames), and an empty name is no name.
func (s Staged) Create(name string) (Staged, Candidate, error) {
	if name == "" {
		return s, Candidate{}, domain.RefusalError{Reason: "the category needs a name"}
	}

	if err := s.checkLeaf(name); err != nil {
		return s, Candidate{}, err
	}
	next := s
	next.NewAccounts = append(slices.Clone(s.NewAccounts), name)

	return next, Candidate{Name: name, Target: name}, nil
}

// checkLeaf refuses a name whose last part some account already ends in.
func (s Staged) checkLeaf(name string) error {
	for _, existing := range s.allNames() {
		if leaf(existing) == leaf(name) {
			return domain.RefusalError{Reason: "the name " + leaf(name) + " is already used by " + existing}
		}
	}

	return nil
}

// SuggestAccountName is the name offered for the account a new IBAN stands for.
func SuggestAccountName(iban string) string {
	return "assets:account-" + iban[max(0, len(iban)-suffixDigits):]
}

const suffixDigits = 4

// WithName gives the account a new IBAN stands for its name: an asset or a liability, with a leaf no other
// account has.
func (s Staged) WithName(name string) (Staged, error) {
	switch {
	case name == "":
		return s, errUnnamedAccount
	case !domain.HoldsMoney(name):
		return s, domain.RefusalError{Reason: "the account must be an assets: or liabilities: account"}
	}

	if err := s.checkLeaf(name); err != nil {
		return s, err
	}
	next := s
	next.Named = name

	return next, nil
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
	next.Rows[index] = next.Rows[index].chosenOnce(candidate.Target)

	return next
}

// chosenOnce is the row posted to target by a choice for it alone, which no rule may take away.
func (r Row) chosenOnce(target string) Row {
	r.Target, r.RuleID, r.Once = target, "", true

	return r
}

// NextRuleID is the id a rule staged now gets.
func (s Staged) NextRuleID() string { return nextRuleID(s.Rules) }

// NextFallbackID is the id a fallback rule staged now gets.
func (s Staged) NextFallbackID() string { return fallbackMark + nextRuleID(s.Rules) }

// RuleDraft is a rule being written: what to match and the extra condition on the purpose.
type RuleDraft struct {
	Field   string
	Pattern string
	Purpose string
}

// RuleDraftFor starts a rule for the row: its payee, else its transaction type, as an escaped literal.
func (s Staged) RuleDraftFor(index int) RuleDraft {
	row := s.Rows[index]

	return RuleDraft{Field: row.MatchField(), Pattern: regexp.QuoteMeta(row.field(row.MatchField()))}
}

// Alternative is the draft as the conditions of one alternative of a rule: the match and the purpose both, or
// the purpose alone when the match is left empty.
func (d RuleDraft) Alternative() []Condition {
	purpose := Condition{Field: fieldPurpose, Pattern: d.Purpose}
	if d.Purpose == "" {
		return []Condition{{Field: d.Field, Pattern: d.Pattern}}
	}

	if d.Pattern == "" {
		return []Condition{purpose}
	}

	return []Condition{{Field: d.Field, Pattern: d.Pattern}, purpose}
}

// Rule is the draft as a rule of the given id and category.
func (d RuleDraft) Rule(id, category string) Rule {
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
	matches := rule.matcher()
	for i := range s.Rows {
		row := &s.Rows[i]
		if !matches(*row) {
			continue
		}

		impact.Matches++
		if rule.wouldTake(*row) {
			impact.Takes++
			if !slices.Contains(impact.From, row.RuleID) {
				impact.From = append(impact.From, row.RuleID)
			}
		}
	}

	return impact
}

// wouldTake reports a row another rule of the same rank categorises: a fallback rule competes only with other
// fallback rules, a rule only with other rules.
func (r Rule) wouldTake(row Row) bool {
	return row.RuleID != "" && row.RuleID != r.ID && row.HeldByFallback() == r.Fallback()
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

		if goOnlyEscape.MatchString(condition.Pattern) {
			return errUnknownEscape
		}
	}

	return nil
}

var errUnknownEscape = domain.RefusalError{
	Reason: `hledger knows no \d, \s or \w: write [0-9], [[:space:]] or [[:alnum:]_]`,
}

// goOnlyEscape finds an escape Go's expressions know and hledger's POSIX ones do not, where it matches nothing,
// so the rule would categorise here and never on the next import: \d, \s or \w after an even run of backslashes.
var goOnlyEscape = regexp.MustCompile(`(^|[^\\])(\\\\)*\\[dDsSwW]`)

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
	rule, index := s.withAlternative(id, alternative)
	if index < 0 {
		return s, domain.RefusalError{Reason: "rule " + id + " does not exist"}
	}

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
	rule, index := s.withAlternative(id, alternative)

	return rule, index >= 0
}

// withAlternative is Widened with where the rule is among the rules; -1 when no rule has the id.
func (s Staged) withAlternative(id string, alternative []Condition) (rule Rule, index int) {
	index = slices.IndexFunc(s.Rules, func(r Rule) bool { return r.ID == id })
	if index < 0 {
		return Rule{}, index
	}
	rule = s.Rules[index]
	rule.Alternatives = append(slices.Clone(rule.Alternatives), alternative)

	return rule, index
}

// apply gives the rule's category to every row it matches that no rule or choice has claimed, or that only a
// fallback rule has while this is none.
func (s Staged) apply(rule Rule) Staged {
	rows, matches := slices.Clone(s.Rows), rule.matcher()
	for i := range rows {
		if rule.mayClaim(rows[i]) && matches(rows[i]) {
			rows[i].Target, rows[i].RuleID = rule.Category, rule.ID
		}
	}
	s.Rows = rows

	return s
}

// mayClaim reports a row open to the rule: one nothing has claimed, or one only a fallback rule holds while
// this rule is none.
func (r Rule) mayClaim(row Row) bool {
	return row.Target == "" || row.HeldByFallback() && !r.Fallback()
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
		if row := &s.Rows[i]; domain.IsCategory(row.Target) && (row.Once || staged[row.RuleID]) {
			total = total.Plus(row.Amount)
		}
	}

	return total
}

// StagedRules are the rules this import adds or changes.
func (s Staged) StagedRules() []Rule {
	return slices.DeleteFunc(slices.Clone(s.Rules), func(r Rule) bool { return !r.Staged })
}

// Restage brings what was staged in the earlier reading of this export over to this one, read again against a
// journal that has since changed: the rules this import staged, the categories it created, the name it
// gave the new account and the choices made for single rows. Rows the staged rules now match are
// categorised by them.
func (s Staged) Restage(earlier Staged) Staged {
	next := s

	next.Edits = earlier.Edits
	if !next.NewAccount {
		next.Named = ""
	}
	next.Rules = withStagedRules(s.Rules, earlier.StagedRules())
	next.Rows = withOnceChoices(s.Rows, earlier.Rows)
	for _, rule := range earlier.StagedRules() {
		next = next.apply(rule)
	}

	return next
}

// withStagedRules are the rules with the staged ones put in, each over the rule of its ID.
func withStagedRules(rules, staged []Rule) []Rule {
	merged := slices.Clone(rules)
	for _, rule := range staged {
		if i := slices.IndexFunc(merged, func(r Rule) bool { return r.ID == rule.ID }); i >= 0 {
			merged[i] = rule
		} else {
			merged = append(merged, rule)
		}
	}

	return merged
}

// withOnceChoices are the rows with the choices made for single earlier rows carried over to the same rows.
func withOnceChoices(rows, earlier []Row) []Row {
	chosenRows := slices.Clone(rows)
	choices := map[rowKey][]Row{}
	for i := range earlier {
		if row := earlier[i]; row.Once {
			choices[row.key()] = append(choices[row.key()], row)
		}
	}

	for i := range chosenRows {
		key := chosenRows[i].key()
		if chosen := choices[key]; len(chosen) > 0 {
			chosenRows[i] = chosenRows[i].chosenOnce(chosen[0].Target)
			choices[key] = chosen[1:]
		}
	}

	return chosenRows
}
