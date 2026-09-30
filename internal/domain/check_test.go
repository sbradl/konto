package domain

import (
	"errors"
	"strings"
	"testing"
)

// A check issue reads as "SEV check file:line:col message" for the user.
func TestCheckIssueLocation(t *testing.T) {
	issue := CheckIssue{Check: "parseable", Severity: SeverityErr, Message: "unexpected 't'", File: "/data/main.journal", Line: 3, Col: 1}

	if got, want := issue.Location(), "/data/main.journal:3:1"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// A JournalError is an error that carries its issue, found through errors.As.
func TestJournalErrorCarriesItsIssue(t *testing.T) {
	issue := CheckIssue{Check: "parseable", Severity: SeverityErr, Message: "unexpected 't'", File: "j", Line: 1, Col: 1}
	var err error = JournalError{Issue: issue}

	var got JournalError
	if !errors.As(err, &got) || got.Issue != issue {
		t.Errorf("errors.As gave %+v, want the issue back", got)
	}
	if !strings.Contains(err.Error(), "j:1:1") || !strings.Contains(err.Error(), "unexpected 't'") {
		t.Errorf("Error() = %q, want location and message", err.Error())
	}
}

// The severities print as the words the panel shows.
func TestSeverityString(t *testing.T) {
	if SeverityErr.String() != "ERR" || SeverityWarn.String() != "WARN" {
		t.Errorf("got %q and %q, want ERR and WARN", SeverityErr, SeverityWarn)
	}
}
