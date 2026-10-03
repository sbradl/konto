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
	plainAmount = regexp.MustCompile(`^(-?[\d.,]+) ([A-Za-z]+)$`)
)

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
	var found parsed
	if !readHeader(lines[0], &found) {
		return parsed{}, false
	}
	return readPostings(lines[1], lines[2], found)
}

// readHeader reads the `~` line: the period, the dates and the description.
func readHeader(line string, into *parsed) bool {
	if match := onceHeader.FindStringSubmatch(line); match != nil {
		into.input = Input{Frequency: Once, Date: match[1], Description: match[2]}
		return true
	}
	match := periodHeader.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	frequency, _ := frequencyOf(match[1])
	into.input = Input{Frequency: frequency, Date: match[2], Description: match[4]}
	if match[3] != "" {
		into.until, _ = time.Parse(time.DateOnly, match[3])
	}
	return true
}

// readPostings reads the two postings: one has the amount, the sign of which says which is the account the
// money comes from and which the one it goes to.
func readPostings(first, second string, found parsed) (parsed, bool) {
	accounts, amounts := [2]string{}, [2]string{}
	for i, line := range [2]string{first, second} {
		match := postingLine.FindStringSubmatch(line)
		if match == nil {
			return parsed{}, false
		}
		accounts[i], amounts[i] = match[1], match[2]
	}
	if (amounts[0] == "") == (amounts[1] == "") {
		return parsed{}, false
	}
	which := 0
	if amounts[0] == "" {
		which = 1
	}
	amount := plainAmount.FindStringSubmatch(amounts[which])
	if amount == nil {
		return parsed{}, false
	}
	number, negative := strings.CutPrefix(amount[1], "-")
	found.input.Amount, found.input.Currency = number, amount[2]
	other := accounts[1-which]
	if negative {
		found.input.From, found.input.To = accounts[which], other
	} else {
		found.input.From, found.input.To = other, accounts[which]
	}
	return found, true
}
