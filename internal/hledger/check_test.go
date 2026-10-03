package hledger

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

const accountsFailure = `hledger: Error: /data/main.journal:10:
   | 2026-09-01 Kiosk
10 |     expenses:coffee         2,00 EUR
   |     ^^^^^^^^^^^^^^^

Strict account checking is enabled, and
account "expenses:coffee" has not been declared.
`

func TestCheckRunsOneNamedCheck(t *testing.T) {
	var calls []call
	client := newClient(recording("", nil, &calls), "/data/main.journal")

	issue, err := client.Check(context.Background(), "payees")

	if issue != nil || err != nil {
		t.Errorf("Check = %+v, %v; want a pass", issue, err)
	}
	want := []string{"-f", "/data/main.journal", "check", "payees"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}

func TestCheckReturnsTheFirstFailureNamedAndRanked(t *testing.T) {
	client := newClient(recordingStderr("", accountsFailure, errors.New("exit status 1"), new([]call)), "/data/main.journal")

	issue, err := client.Check(context.Background(), "accounts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.CheckIssue{
		Check: "accounts", Severity: domain.SeverityWarn, File: "/data/main.journal", Line: 10,
		Message: `Strict account checking is enabled, and account "expenses:coffee" has not been declared.`,
	}
	if issue == nil || *issue != want {
		t.Errorf("issue = %+v, want %+v", issue, want)
	}
}

func TestCheckRanksByTheCheckName(t *testing.T) {
	client := newClient(recordingStderr("", accountsFailure, errors.New("exit status 1"), new([]call)), "/data/main.journal")

	issue, _ := client.Check(context.Background(), "balanced")

	if issue == nil || issue.Severity != domain.SeverityErr || issue.Check != "balanced" {
		t.Errorf("issue = %+v, want an error for balanced", issue)
	}
}

func TestCheckReportsAFailureWithoutALocation(t *testing.T) {
	client := newClient(recordingStderr("", "boom", errors.New("exit status 1"), new([]call)), "/data/main.journal")

	issue, err := client.Check(context.Background(), "tags")

	if issue != nil || err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("Check = %+v, %v; want an error carrying the stderr", issue, err)
	}
}

func TestFirstParagraphJoinsTheMessageUpToTheNextBlankLine(t *testing.T) {
	for name, tc := range map[string]struct{ text, want string }{
		"two paragraphs": {"   | quoted\n\nfirst line\nsecond line\n\nanother\n", "first line second line"},
		"no blank after": {"first\nsecond", "first second"},
		"only source":    {"   | quoted\n", ""},
		"leading blanks": {"\n\nonly\n", "only"},
	} {
		if got := firstParagraph(tc.text); got != tc.want {
			t.Errorf("%s: firstParagraph = %q, want %q", name, got, tc.want)
		}
	}
}
