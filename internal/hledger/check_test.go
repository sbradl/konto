package hledger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestCheckTextPipesTheTextToTheNamedChecks(t *testing.T) {
	var gotInput string
	var gotArgs []string
	client := &Client{runInput: func(_ context.Context, input string, args ...string) ([]byte, string, error) {
		gotInput, gotArgs = input, args
		return nil, "", nil
	}}

	err := client.CheckText(context.Background(), "journal text", []string{"balanced", "assertions"})

	if err != nil || gotInput != "journal text" {
		t.Errorf("err = %v, input = %q", err, gotInput)
	}
	if want := []string{"-f", "-", "check", "balanced", "assertions"}; !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("args = %v, want %v", gotArgs, want)
	}
}

func TestCheckTextReportsTheFirstFailureInHledgersWords(t *testing.T) {
	client := &Client{runInput: func(context.Context, string, ...string) ([]byte, string, error) {
		return nil, "hledger: Error: -:5:\n5 | x\n\nbalance assertion: assets:checking\nthe asserted balance is 1\n", errors.New("exit 1")
	}}

	err := client.CheckText(context.Background(), "", nil)

	if err == nil || !strings.Contains(err.Error(), "balance assertion: assets:checking") {
		t.Errorf("error = %v", err)
	}
}

func TestNewChecksTextThroughTheBinary(t *testing.T) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "stdin")
	script := "#!/bin/sh\n/bin/cat > " + seen + "\necho 'where' >&2\necho 'broken' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "hledger"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	err := New("hledger", "/j").CheckText(context.Background(), "the text", []string{"balanced"})

	if err == nil || err.Error() != "broken" {
		t.Errorf("error = %v, want hledger's stderr", err)
	}
	if got, _ := os.ReadFile(seen); string(got) != "the text" {
		t.Errorf("hledger read %q from stdin", got)
	}
}
