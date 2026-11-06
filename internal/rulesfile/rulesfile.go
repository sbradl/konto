// Package rulesfile is the line syntax of categories.rules that both the import, which reads and spells the
// rules, and the journal adapter, which writes them in place, need: blank lines separate the blocks, and a
// konto rule's block holds the line that tags the posting with the rule's id.
package rulesfile

import (
	"slices"
	"strings"
)

// IDLine is the line of a block that tags the posting with the rule's id, indentation aside: the
// konto-rule posting tag (domain.RuleTag).
func IDLine(id string) string { return "comment2 konto-rule:" + id }

// Block is where the block of the rule with the id is among the lines of categories.rules: its first line and
// the index after its last; false when no block has the id.
func Block(lines []string, id string) (first, end int, found bool) {
	at := slices.IndexFunc(lines, func(line string) bool { return strings.TrimSpace(line) == IDLine(id) })
	if at < 0 {
		return 0, 0, false
	}
	first, end = at, at+1
	for first > 0 && !IsBlank(lines[first-1]) {
		first--
	}
	for end < len(lines) && !IsBlank(lines[end]) {
		end++
	}

	return first, end, true
}

// IsBlank reports a line of nothing but spaces, the line that ends a block.
func IsBlank(line string) bool { return strings.TrimSpace(line) == "" }
