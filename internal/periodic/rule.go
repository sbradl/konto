package periodic

import (
	"regexp"
	"strings"
	"time"
)

var (
	onceHeader = regexp.MustCompile(`^~ (\d{4}-\d{2}-\d{2}) {2}(\S.*)$`)
	// periodHeader is `~ <period> from <date>[ to <date>]  <description>`, the period one the dropdown offers.
	periodHeader = regexp.MustCompile(`^~ (weekly|biweekly|monthly|quarterly|every 6 months|yearly)` +
		` from (\d{4}-\d{2}-\d{2})(?: to (\d{4}-\d{2}-\d{2}))? {2}(\S.*)$`)
	postingLine = regexp.MustCompile(`^[ \t]+(\S(?:.*?\S)?)(?: {2,}(\S.*))?$`)
	// rightAmount and leftAmount are a single-commodity amount the way domain.Amount.Format writes it: the
	// symbol after the number or before it, spaced or not. The symbol is anything but digits, spaces, the
	// number's marks and the characters of a price, an assertion or a comment.
	rightAmount = regexp.MustCompile(`^(-?[\d.,]+) ?([^\d\s.,;"=@*()+-]+)$`)
	leftAmount  = regexp.MustCompile(`^([^\d\s.,;"=@*()+-]+) ?(-?[\d.,]+)$`)
)

// readAmount splits an amount into its number and its commodity; false for anything more than one amount.
func readAmount(text string) (number, symbol string, ok bool) {
	if match := rightAmount.FindStringSubmatch(text); match != nil {
		return match[1], match[2], true
	}

	if match := leftAmount.FindStringSubmatch(text); match != nil {
		return match[2], match[1], true
	}

	return "", "", false
}

// parsed is a rule of the journal read as the form shows it: what the form fields hold, and the exclusive end
// date the rule has, if any.
type parsed struct {
	input Input
	until time.Time
}

// parseRule reads a `~` block into the form, or reports that it does not fit (spec §4.8): a period expression
// the dropdown offers, exactly two postings, exactly one single-commodity amount without a price or an
// assertion, and no comments or tags anywhere.
func parseRule(lines []string) (parsed, bool) {
	if len(lines) != 3 || strings.Contains(strings.Join(lines, "\n"), ";") {
		return parsed{}, false
	}

	found, ok := readHeader(lines[0])
	if !ok {
		return parsed{}, false
	}

	return readPostings(lines[1], lines[2], found)
}

// readHeader reads the `~` line: the period, the dates and the description.
func readHeader(line string) (parsed, bool) {
	if match := onceHeader.FindStringSubmatch(line); match != nil {
		return parsed{input: Input{Frequency: Once, Date: match[1], Description: match[2]}}, true
	}

	match := periodHeader.FindStringSubmatch(line)
	if match == nil {
		return parsed{}, false
	}
	frequency, _ := frequencyOf(match[1])

	found := parsed{input: Input{Frequency: frequency, Date: match[2], Description: match[4]}}
	if match[3] != "" {
		found.until, _ = time.Parse(time.DateOnly, match[3])
	}

	return found, true
}

// readPostings reads the two postings: one has the amount, the sign of which says which is the account the
// money comes from and which the one it goes to.
func readPostings(first, second string, found parsed) (parsed, bool) {
	accounts, amounts, ok := splitPostings(first, second)
	if !ok || (amounts[0] == "") == (amounts[1] == "") {
		return parsed{}, false
	}

	which := 0
	if amounts[0] == "" {
		which = 1
	}

	signed, symbol, ok := readAmount(amounts[which])
	if !ok {
		return parsed{}, false
	}
	number, negative := strings.CutPrefix(signed, "-")
	found.input.Amount, found.input.Currency = number, symbol

	from, to := accounts[1-which], accounts[which]
	if negative {
		from, to = to, from
	}
	found.input.From, found.input.To = from, to

	return found, true
}

// splitPostings is the account and the amount text of each of the two posting lines.
func splitPostings(first, second string) (accounts, amounts [2]string, ok bool) {
	for i, line := range [2]string{first, second} {
		match := postingLine.FindStringSubmatch(line)
		if match == nil {
			return accounts, amounts, false
		}
		accounts[i], amounts[i] = match[1], match[2]
	}

	return accounts, amounts, true
}
