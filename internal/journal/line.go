package journal

import (
	"regexp"
	"strings"
	"time"
)

// The journal line syntax konto reads, in one place: what follows a directive's keyword, a `~` rule header, a
// transaction's date, a posting's account, and the indented lines that belong to the line above. hledger
// separates the parts of a line by spaces or a tab, so each reader here takes either.

const blanks = " \t"

// directiveArgs is what follows the keyword on a line of that directive; false for any other line.
func directiveArgs(line, keyword string) (string, bool) {
	rest, found := strings.CutPrefix(line, keyword)
	args := strings.TrimLeft(rest, blanks)
	return args, found && len(args) < len(rest) // a blank must end the keyword: `includes` is no include
}

// isRuleHeader reports whether the line opens a periodic (`~`) rule.
func isRuleHeader(line string) bool { return strings.HasPrefix(line, "~") }

// indented reports whether the line belongs to the block above it: a posting, a comment or a subdirective.
func indented(line string) bool { return strings.TrimLeft(line, blanks) != line }

// headerDate is the date a transaction's first line starts with, its parts separated by -, / or . as hledger
// reads them (2026-09-30, 2026/9/30, 2026.09.30), for a file edited by hand.
func headerDate(line string) (time.Time, bool) {
	date, err := time.Parse("2006-1-2", strings.NewReplacer("/", "-", ".", "-").Replace(leadingDate.FindString(line)))
	return date, err == nil
}

var leadingDate = regexp.MustCompile(`^\d{4}[-/.]\d{1,2}[-/.]\d{1,2}`)

// postingAccount is the account a posting line names, without its status mark or virtual-posting bracket.
func postingAccount(line string) string {
	text := strings.TrimSpace(line)
	if strings.HasPrefix(text, ";") {
		return ""
	}
	return strings.Trim(accountField(strings.TrimLeft(text, "!*"+blanks)), "()[]")
}

// accountField is the text up to where hledger ends an account name: two spaces or a tab.
func accountField(text string) string {
	name, _, _ := strings.Cut(text, "\t")
	name, _, _ = strings.Cut(name, "  ")
	return name
}
