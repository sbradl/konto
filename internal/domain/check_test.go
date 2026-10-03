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

func TestSeverityOfIsAssignedByCheckName(t *testing.T) {
	for check, want := range map[string]Severity{
		"parseable": SeverityErr, "autobalanced": SeverityErr, "assertions": SeverityErr, "balanced": SeverityErr,
		"commodities": SeverityWarn, "accounts": SeverityWarn, "ordereddates": SeverityWarn, "tags": SeverityWarn,
		"payees": SeverityWarn, "recentassertions": SeverityWarn, "uniqueleafnames": SeverityWarn,
		"unclassified": SeverityWarn,
	} {
		if got := SeverityOf(check); got != want {
			t.Errorf("SeverityOf(%q) = %v, want %v", check, got, want)
		}
	}
}

func TestCheckRunCountsIssuesBySeverity(t *testing.T) {
	run := CheckRun{Total: 12, Issues: []CheckIssue{
		{Severity: SeverityErr}, {Severity: SeverityWarn}, {Severity: SeverityWarn},
	}}

	if run.Count(SeverityErr) != 1 || run.Count(SeverityWarn) != 2 {
		t.Errorf("counts = %d err, %d warn, want 1 and 2", run.Count(SeverityErr), run.Count(SeverityWarn))
	}
	if run.IsClean() {
		t.Error("a run with issues is not clean")
	}
	if !(CheckRun{Total: 12}).IsClean() {
		t.Error("a run without issues is clean")
	}
}
