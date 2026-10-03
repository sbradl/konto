package journal

import "strings"

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
	body := strings.TrimSpace(strings.TrimPrefix(header, "~"))
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
