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

// parseIssue turns hledger's stderr for a journal it cannot load into the located issue
// (the parseable issue of the parse-error screen), reading its message with messageOf.
// ok is false when stderr does not carry a location.
func parseIssue(stderr string, messageOf func(text string) string) (domain.CheckIssue, bool) {
	header, rest, _ := strings.Cut(stderr, "\n")
	groups := errorHeader.FindStringSubmatch(header)
	if groups == nil {
		return domain.CheckIssue{}, false
	}
	line, _ := strconv.Atoi(groups[2])
	col, _ := strconv.Atoi(groups[3])
	return domain.CheckIssue{
		Check:    domain.ParseableCheck,
		Severity: domain.SeverityOf(domain.ParseableCheck),
		Message:  messageOf(rest),
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

// firstParagraph is the message up to the next blank line, joined onto one line: the lines
// after the quoted source. Check failures explain themselves over several lines.
func firstParagraph(text string) string {
	var words []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if sourceSnippet.MatchString(line) {
			continue
		}
		if line == "" && len(words) > 0 {
			return strings.Join(words, " ")
		}
		if line != "" {
			words = append(words, line)
		}
	}
	return strings.Join(words, " ")
}
