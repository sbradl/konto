package hledger

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// errorHeader matches the first line of a located hledger error:
// "hledger: Error: file:line:col:" or "file:first-last:" for a line range.
var errorHeader = regexp.MustCompile(`^hledger: Error: (.+?):(\d+)(?:-\d+)?(?::(\d+))?:\s*$`)

// sourceSnippet matches the quoted source lines hledger prints under the header.
var sourceSnippet = regexp.MustCompile(`^\s*(\d+ )?\|`)

// parseIssue turns hledger's stderr for a journal it cannot load into the
// parseable issue the parse-error screen shows. ok is false when stderr does
// not carry a location.
func parseIssue(stderr string) (domain.CheckIssue, bool) {
	header, rest, _ := strings.Cut(stderr, "\n")
	groups := errorHeader.FindStringSubmatch(header)
	if groups == nil {
		return domain.CheckIssue{}, false
	}
	line, _ := strconv.Atoi(groups[2])
	col, _ := strconv.Atoi(groups[3])
	return domain.CheckIssue{
		Check:    "parseable",
		Severity: domain.SeverityErr,
		Message:  firstMessageLine(rest),
		File:     groups[1],
		Line:     line,
		Col:      col,
	}, true
}

// firstMessageLine is the first line that is neither blank nor quoted source.
func firstMessageLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) != "" && !sourceSnippet.MatchString(line) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
