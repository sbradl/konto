package journal

import (
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// PeriodicRules lists the `~` blocks of the journal and the files it includes, in file order.
func (f File) PeriodicRules() ([]domain.PeriodicRule, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	var rules []domain.PeriodicRule
	for _, source := range sources {
		rules = append(rules, blocksIn(source)...)
	}

	return rules, nil
}

// blocksIn reads the rule blocks of one file: a `~` line and the indented lines that follow it.
func blocksIn(source *source) []domain.PeriodicRule {
	var rules []domain.PeriodicRule
	for i, line := range source.lines {
		if !isRuleHeader(line) {
			continue
		}
		period, _, _ := strings.Cut(headerBody(line), "  ")
		rules = append(rules, domain.PeriodicRule{
			Period: strings.TrimSpace(period), Description: ruleName(line),
			Lines: slices.Clone(source.lines[i:blockEnd(source.lines, i)]),
			Pos:   domain.SourcePos{File: source.path, Line: i + 1},
		})
	}

	return rules
}

// headerBody is a `~` line without its marker and trailing comment.
func headerBody(header string) string {
	body, _, _ := strings.Cut(strings.TrimPrefix(header, "~"), ";")

	return strings.TrimSpace(body)
}

// RulesReferencing lists the descriptions of the periodic (`~`) rules that post to the account,
// whether or not they have ended. A rule without a description is named by its period.
func (f File) RulesReferencing(account string) ([]string, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	var rules []string
	for _, source := range sources {
		rules = append(rules, rulesIn(source, account)...)
	}

	return rules, nil
}

// rulesIn names the `~` blocks of the file that have a posting to account.
func rulesIn(source *source, account string) []string {
	var rules []string
	for _, rule := range blocksIn(source) {
		if slices.ContainsFunc(rule.Lines[1:], func(line string) bool { return postingAccount(line) == account }) {
			rules = append(rules, rule.Description)
		}
	}

	return rules
}

// ruleName is the description after the period expression (two spaces apart), else the period.
func ruleName(header string) string {
	body := headerBody(header)
	if _, description, found := strings.Cut(body, "  "); found {
		return strings.TrimSpace(description)
	}

	return body
}
