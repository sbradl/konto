//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

var yearFile = regexp.MustCompile(`^\d{4}\.journal$`)

// hledgerOn builds an hledger command on the ledger whose main journal is at journal: the main journal and the
// year files beside it, as one journal that includes them (hledger checks balance assertions per -f file, but
// across an include).
func hledgerOn(hledger, journal string, args ...string) *exec.Cmd {
	var includes strings.Builder
	includes.WriteString("include " + journal + "\n")
	entries, _ := os.ReadDir(filepath.Dir(journal))
	for _, entry := range entries {
		if yearFile.MatchString(entry.Name()) {
			includes.WriteString("include " + filepath.Join(filepath.Dir(journal), entry.Name()) + "\n")
		}
	}
	cmd := exec.CommandContext(context.Background(), hledger, append([]string{"-f", "-"}, args...)...)
	cmd.Stdin = strings.NewReader(includes.String())

	return cmd
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
		if out, err := hledgerOn(hledger, journal, append([]string{"check"}, g...)...).CombinedOutput(); err != nil {
			failures = append(failures, fmt.Sprintf("check %s: %s", strings.Join(g, " "), strings.TrimSpace(string(out))))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("hledger checks failed:\n%s", strings.Join(failures, "\n"))
	}

	return nil
}
