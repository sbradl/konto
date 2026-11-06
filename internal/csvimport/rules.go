package csvimport

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/rulesfile"
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

// fallbackMark starts the id of a fallback rule: a rule that holds only the rows no other rule matches. It has its
// place in the id because the posting's tag is all a row read back from hledger knows of its rule.
const fallbackMark = "f"

// Fallback reports a rule that yields to any other rule; only other fallback rules can overlap it.
func (r Rule) Fallback() bool { return strings.HasPrefix(r.ID, fallbackMark) }

// HeldByFallback reports a row categorised by a fallback rule, which any other rule may take it from.
func (r Row) HeldByFallback() bool { return strings.HasPrefix(r.RuleID, fallbackMark) }

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
		for _, conditions := range alternatives {
			if allHold(conditions, row) {
				return true
			}
		}

		return false
	}
}

func allHold(conditions []func(Row) bool, row Row) bool {
	for _, holds := range conditions {
		if !holds(row) {
			return false
		}
	}

	return true
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
			lines = append(lines, conditionLine(condition, i, j))
		}
	}
	lines = append(lines, "    account2 "+r.Category, "    "+rulesfile.IDLine(r.ID))

	return strings.Join(lines, "\n")
}

// conditionLine spells the condition at position j of alternative i: the rule's first condition opens it with
// `if`, a later condition of the same alternative joins it with `&`, the first of another alternative stands
// alone.
func conditionLine(condition Condition, i, j int) string {
	line := "%" + condition.Field + " " + condition.Pattern
	if j > 0 {
		return "& " + line
	}

	if i == 0 {
		return "if " + line
	}

	return line
}

// ParseRules reads the rules konto wrote into categories.rules; a block without a konto-rule id is
// somebody else's and is left out.
func ParseRules(text string) []Rule {
	var rules []Rule
	for _, block := range ruleBlocks(text) {
		if rule := parseRule(block); rule.ID != "" && rule.Category != "" {
			rules = append(rules, rule)
		}
	}

	return rules
}

// ruleBlocks are the `if` blocks of the text: each its `if` line and the lines up to the next blank or `if`
// line. Lines outside a block are left out.
func ruleBlocks(text string) [][]string {
	var blocks [][]string
	open := false
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "if "):
			blocks, open = append(blocks, []string{line}), true
		case rulesfile.IsBlank(line):
			open = false
		case open:
			blocks[len(blocks)-1] = append(blocks[len(blocks)-1], line)
		}
	}

	return blocks
}

// parseRule reads one `if` block.
func parseRule(block []string) Rule {
	rule := Rule{}.orCondition(strings.TrimPrefix(block[0], "if "))
	for _, line := range block[1:] {
		rule = rule.withLine(line)
	}

	return rule
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
	case strings.HasPrefix(trimmed, rulesfile.IDLine("")):
		r.ID = strings.TrimSpace(strings.TrimPrefix(trimmed, rulesfile.IDLine("")))
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
		if n, err := strconv.Atoi(strings.TrimPrefix(rule.ID, fallbackMark)); err == nil {
			highest = max(highest, n)
		}
	}

	return strconv.Itoa(highest + 1)
}
