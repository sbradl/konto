package journal

import (
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
		if !strings.HasPrefix(line, "~") {
			continue
		}
		lines := []string{line}
		for _, next := range source.lines[i+1:] {
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break
			}
			lines = append(lines, next)
		}
		period, _, _ := strings.Cut(headerBody(line), "  ")
		rules = append(rules, domain.PeriodicRule{
			Period: strings.TrimSpace(period), Description: ruleName(line), Lines: lines,
			Pos: domain.SourcePos{File: source.path, Line: i + 1},
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
		rules = append(rules, rulesIn(source.lines, account)...)
	}
	return rules, nil
}

// rulesIn finds the `~` blocks of lines that post to account: a header line, then indented postings.
func rulesIn(lines []string, account string) []string {
	var rules []string
	name, inRule, references := "", false, false
	flush := func() {
		if inRule && references {
			rules = append(rules, name)
		}
		inRule, references = false, false
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "~"):
			flush()
			name, inRule = ruleName(line), true
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"):
			references = references || postingAccount(line) == account
		default:
			flush()
		}
	}
	flush()
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

// postingAccount is the account a posting line names, without a virtual-posting bracket.
func postingAccount(line string) string {
	text := strings.TrimSpace(line)
	if strings.HasPrefix(text, ";") {
		return ""
	}
	account, _, _ := strings.Cut(text, "  ")
	account, _, _ = strings.Cut(account, "\t")
	account = strings.TrimLeft(account, "!* ")
	return strings.Trim(account, "()[]")
}
