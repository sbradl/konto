package csvimport

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// Condition is one `%field pattern` test of a categorisation rule. The pattern is a case-insensitive
// regular expression, as hledger reads it.
type Condition struct {
	Field   string // payee, type or purpose
	Pattern string
}

// Rule is one `if` block of categories.rules: rows that satisfy any of its alternatives (each the
// conditions that all hold) are posted to Category. Its ID is the `konto-rule` tag the posting carries.
type Rule struct {
	ID           string
	Category     string
	Alternatives [][]Condition
	Staged       bool // not yet in categories.rules
}

// Matches reports whether any alternative of the rule holds for the row.
func (r Rule) Matches(row Row) bool { return r.matcher()(row) }

// matcher is Matches with the patterns compiled once, for testing many rows; an invalid pattern holds for none.
func (r Rule) matcher() func(Row) bool {
	alternatives := make([][]func(Row) bool, len(r.Alternatives))
	for i, alternative := range r.Alternatives {
		for _, condition := range alternative {
			alternatives[i] = append(alternatives[i], condition.matcher())
		}
	}
	return func(row Row) bool {
		return slices.ContainsFunc(alternatives, func(conditions []func(Row) bool) bool {
			return !slices.ContainsFunc(conditions, func(holds func(Row) bool) bool { return !holds(row) })
		})
	}
}

func (c Condition) matcher() func(Row) bool {
	pattern, err := regexp.Compile("(?i)" + c.Pattern)
	if err != nil {
		return func(Row) bool { return false }
	}
	return func(row Row) bool { return pattern.MatchString(row.field(c.Field)) }
}

// Text is the rule as categories.rules spells it.
func (r Rule) Text() string {
	var lines []string
	for i, alternative := range r.Alternatives {
		for j, condition := range alternative {
			line := "%" + condition.Field + " " + condition.Pattern
			switch {
			case j > 0:
				line = "& " + line
			case i == 0:
				line = "if " + line
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "    account2 "+r.Category, "    "+domain.RuleIDLine(r.ID))
	return strings.Join(lines, "\n")
}

// ParseRules reads the rules konto wrote into categories.rules; a block without a konto-rule id is
// somebody else's and is left out.
func ParseRules(text string) []Rule {
	var rules []Rule
	current := Rule{}
	open := false
	flush := func() {
		if open && current.ID != "" && current.Category != "" {
			rules = append(rules, current)
		}
		current, open = Rule{}, false
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "if "):
			flush()
			current, open = current.orCondition(strings.TrimPrefix(line, "if ")), true
		case domain.IsBlankLine(line):
			flush()
		case open:
			current = current.withLine(line)
		}
	}
	flush()
	return rules
}

// withLine applies one line of a block: a condition, the category or the rule id.
func (r Rule) withLine(line string) Rule {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "& "):
		return r.andCondition(strings.TrimPrefix(line, "& "))
	case strings.HasPrefix(line, "%"):
		return r.orCondition(line)
	case strings.HasPrefix(trimmed, "account2 "):
		r.Category = strings.TrimSpace(strings.TrimPrefix(trimmed, "account2 "))
	case strings.HasPrefix(trimmed, domain.RuleIDLine("")):
		r.ID = strings.TrimSpace(strings.TrimPrefix(trimmed, domain.RuleIDLine("")))
	}
	return r
}

func parseCondition(text string) Condition {
	field, pattern, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(text), "%"), " ")
	return Condition{Field: field, Pattern: strings.TrimSpace(pattern)}
}

// orCondition is the rule with a new alternative that holds the condition.
func (r Rule) orCondition(text string) Rule {
	r.Alternatives = append(slices.Clone(r.Alternatives), []Condition{parseCondition(text)})
	return r
}

// andCondition is the rule with the condition added to its last alternative, which a rule being read
// always has by the time a line can follow its `if`.
func (r Rule) andCondition(text string) Rule {
	last := len(r.Alternatives) - 1
	r.Alternatives = slices.Clone(r.Alternatives)
	r.Alternatives[last] = append(slices.Clone(r.Alternatives[last]), parseCondition(text))
	return r
}

// nextRuleID is the id after the highest numeric one among rules.
func nextRuleID(rules []Rule) string {
	highest := 0
	for _, rule := range rules {
		if n, err := strconv.Atoi(rule.ID); err == nil {
			highest = max(highest, n)
		}
	}
	return strconv.Itoa(highest + 1)
}
