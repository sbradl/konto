//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// hledgerChecks is every check konto runs (spec §4.6). The basic ones run
// implicitly; -s adds balanced, commodities and accounts.
var hledgerChecks = []string{
	"parseable", "autobalanced", "assertions",
	"balanced", "commodities", "accounts",
	"ordereddates", "tags", "payees", "recentassertions", "uniqueleafnames",
}

// normaliseLine trims a line and collapses runs of whitespace to one space.
func normaliseLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// containsBlock reports whether want's lines appear consecutively in text,
// both compared whitespace-normalised. Leading and trailing blank lines of
// want are ignored.
func containsBlock(text, want string) bool {
	norm := func(s string) []string {
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			lines[i] = normaliseLine(l)
		}
		return lines
	}
	have := norm(text)
	block := norm(strings.Trim(want, "\n"))
	for i := 0; i+len(block) <= len(have); i++ {
		if equalLines(have[i:i+len(block)], block) {
			return true
		}
	}
	return false
}

func equalLines(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// runChecks runs hledger's checks on the journal: all at once, or one
// invocation per check so each failure is reported (hledger stops at the
// first).
func runChecks(hledger, journal string, individually bool) error {
	groups := [][]string{hledgerChecks}
	if individually {
		groups = make([][]string, 0, len(hledgerChecks))
		for _, c := range hledgerChecks {
			groups = append(groups, []string{c})
		}
	}
	var failures []string
	for _, g := range groups {
		args := append([]string{"-f", journal, "check"}, g...)
		if out, err := exec.CommandContext(context.Background(), hledger, args...).CombinedOutput(); err != nil {
			failures = append(failures, fmt.Sprintf("check %s: %s", strings.Join(g, " "), strings.TrimSpace(string(out))))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("hledger checks failed:\n%s", strings.Join(failures, "\n"))
	}
	return nil
}
