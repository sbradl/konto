package domain

import "fmt"

// Severity ranks a check issue. Severity is assigned by check name (spec §5.2).
type Severity int

// The two severities the `!` panel distinguishes.
const (
	SeverityErr  Severity = 1
	SeverityWarn Severity = 2
)

func (s Severity) String() string {
	if s == SeverityErr {
		return "ERR"
	}
	return "WARN"
}

// CheckIssue is one failed journal check, located in the journal text.
type CheckIssue struct {
	Check    string
	Severity Severity
	Message  string
	File     string
	Line     int
	Col      int
}

// Location is the file:line:col reference of the issue.
func (c CheckIssue) Location() string {
	return fmt.Sprintf("%s:%d:%d", c.File, c.Line, c.Col)
}

// JournalError is the failure to load a journal that hledger cannot read.
type JournalError struct {
	Issue CheckIssue
}

func (e JournalError) Error() string {
	return e.Issue.Location() + ": " + e.Issue.Message
}

// Check is one `hledger check` and the severity of its failure (spec §5.2).
type Check struct {
	Name     string
	Severity Severity
}

// ParseableCheck is the check a journal hledger cannot read at all fails.
const ParseableCheck = "parseable"

// HledgerChecks are every `hledger check`, in the order the panel reports them: the checks that make a
// journal wrong are errors, everything else is a warning.
func HledgerChecks() []Check {
	return []Check{
		{ParseableCheck, SeverityErr},
		{"autobalanced", SeverityErr},
		{"assertions", SeverityErr},
		{"balanced", SeverityErr},
		{"commodities", SeverityWarn},
		{"accounts", SeverityWarn},
		{"ordereddates", SeverityWarn},
		{"tags", SeverityWarn},
		{"payees", SeverityWarn},
		{"recentassertions", SeverityWarn},
		{"uniqueleafnames", SeverityWarn},
	}
}

// SeverityOf is the severity of a failed check, assigned by its name; a check konto does not know is a warning.
func SeverityOf(check string) Severity {
	for _, known := range HledgerChecks() {
		if known.Name == check {
			return known.Severity
		}
	}
	return SeverityWarn
}

// CheckRun is the outcome of one pass over every check: the issues found and how many checks ran.
type CheckRun struct {
	Issues []CheckIssue
	Total  int
}

// Count is how many issues have the given severity.
func (r CheckRun) Count(severity Severity) int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == severity {
			n++
		}
	}
	return n
}

// IsClean reports a run without any issue.
func (r CheckRun) IsClean() bool { return len(r.Issues) == 0 }
