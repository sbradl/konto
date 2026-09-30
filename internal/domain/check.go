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
