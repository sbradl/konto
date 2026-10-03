package hledger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

var printJSON = `[
  {"tdate": "2026-01-01", "tdescription": "opening balance", "tindex": 1, "tpostings": []},
  {"tdate": "2026-09-14", "tdescription": "Rent", "tindex": 2, "tpostings": [
    {"paccount": "expenses:rent", "pamount": [` + amountJSON("EUR", 90000, eurStyle) + `]},
    {"paccount": "assets:checking", "pamount": [` + amountJSON("EUR", -90000, eurStyle) + `]}
  ]}
]`

var euroStyle = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

type call struct{ args []string }

func recording(out string, err error, calls *[]call) runner {
	return recordingStderr(out, "", err, calls)
}

func recordingStderr(out, stderr string, err error, calls *[]call) runner {
	return func(_ context.Context, args ...string) ([]byte, string, error) {
		*calls = append(*calls, call{args})
		return []byte(out), stderr, err
	}
}

// Print returns the journal's transactions, oldest first as hledger prints them.
func TestPrintReturnsTransactions(t *testing.T) {
	var calls []call
	client := newClient(recording(printJSON, nil, &calls), "/data/main.journal")

	got, err := client.Print(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []domain.Transaction{
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Description: "opening balance", Postings: []domain.Posting{}},
		{Date: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), Description: "Rent", Postings: []domain.Posting{
			{Account: "expenses:rent", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: 90000, Places: 2, Style: euroStyle}}},
			{Account: "assets:checking", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: -90000, Places: 2, Style: euroStyle}}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transactions = %+v, want %+v", got, want)
	}
}

// Every hledger call names the journal explicitly with -f (spec §7.1).
func TestPrintPassesJournalPathExplicitly(t *testing.T) {
	var calls []call
	client := newClient(recording("[]", nil, &calls), "/data/main.journal")

	if _, err := client.Print(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "/data/main.journal", "print", "-O", "json"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}

// A journal without transactions prints an empty list, not an error.
func TestPrintOfEmptyJournalIsEmpty(t *testing.T) {
	client := newClient(recording("[]", nil, new([]call)), "/data/main.journal")

	got, err := client.Print(context.Background())

	if err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v; want no transactions and no error", got, err)
	}
}

// A failing hledger call is reported with its cause.
func TestPrintReportsHledgerFailure(t *testing.T) {
	client := newClient(recording("", errors.New("exit status 1"), new([]call)), "/data/main.journal")

	_, err := client.Print(context.Background())

	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}

// Output that is not the expected JSON is reported, never guessed at.
func TestPrintRejectsGarbage(t *testing.T) {
	for name, out := range map[string]string{
		"not json":     "hledger: oops",
		"bad date":     `[{"tdate": "yesterday", "tdescription": "x"}]`,
		"wrong shape":  `{"tdate": "2026-01-01"}`,
		"empty output": "",
		"bad posting amount": `[{"tdate": "2026-01-01", "tdescription": "x", "tpostings": [{"paccount": "a", "pamount": [` +
			amountJSON("EUR", 1, `{"asdigitgroups": [1, [3]]}`) + `]}]}]`,
	} {
		client := newClient(recording(out, nil, new([]call)), "/data/main.journal")

		if _, err := client.Print(context.Background()); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// New runs the named hledger binary, passing its arguments through.
func TestNewRunsTheHledgerBinary(t *testing.T) {
	fakeHledger(t, printJSON, 0)

	got, err := New("hledger", "/data/main.journal").Print(context.Background())

	if err != nil || len(got) != 2 {
		t.Errorf("got %+v, %v; want two transactions and no error", got, err)
	}
}

// The real runner reports a binary that fails.
func TestNewReportsBinaryFailure(t *testing.T) {
	fakeHledger(t, "boom", 3)

	if _, err := New("hledger", "/data/main.journal").Print(context.Background()); err == nil {
		t.Error("expected an error when hledger exits non-zero, got nil")
	}
}

func printErrors(t *testing.T, stderr string) error {
	t.Helper()
	client := newClient(recordingStderr("", stderr, errors.New("exit status 1"), new([]call)), "/data/main.journal")
	_, err := client.Print(context.Background())
	return err
}

// hledger's parse errors become a located ERR issue for the parse-error screen.
func TestPrintTurnsParseErrorIntoJournalError(t *testing.T) {
	err := printErrors(t, "hledger: Error: /data/main.journal:3:5:\n  |\n3 | bad line\n  | ^\n"+
		"unexpected 't'\nexpecting end of input or transaction or directive\n")

	var got domain.JournalError
	if !errors.As(err, &got) {
		t.Fatalf("error = %v, want a JournalError", err)
	}
	want := domain.CheckIssue{
		Check: "parseable", Severity: domain.SeverityErr, Message: "unexpected 't'",
		File: "/data/main.journal", Line: 3, Col: 5,
	}
	if got.Issue != want {
		t.Errorf("issue = %+v, want %+v", got.Issue, want)
	}
}

// A line range without a column locates the issue at the range's first line.
func TestPrintReadsLineRanges(t *testing.T) {
	err := printErrors(t, "hledger: Error: /data/main.journal:2-4:\n2 | 2026-01-01 x\n  |     a   1 EUR\n\n"+
		"This transaction is unbalanced.\nThe real postings all have the same sign.\n")

	var got domain.JournalError
	if !errors.As(err, &got) {
		t.Fatalf("error = %v, want a JournalError", err)
	}
	if got.Issue.Line != 2 || got.Issue.Col != 0 || got.Issue.Message != "This transaction is unbalanced." {
		t.Errorf("issue = %+v, want line 2, no column and the first message line", got.Issue)
	}
}

// Stderr that is not a located hledger error is reported as plain text.
func TestPrintKeepsUnlocatedStderr(t *testing.T) {
	err := printErrors(t, "hledger: Error: something odd\n")

	if _, ok := errors.AsType[domain.JournalError](err); ok {
		t.Fatalf("error = %v, should not be a JournalError", err)
	}
	if !strings.Contains(err.Error(), "something odd") {
		t.Errorf("error = %v, want the stderr text", err)
	}
}

// The real runner hands hledger's stderr to the parse-error mapping.
func TestNewMapsRealStderrToJournalError(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'hledger: Error: /data/main.journal:1:1:' >&2\necho 'unexpected x' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "hledger"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	_, err := New("hledger", "/data/main.journal").Print(context.Background())

	var got domain.JournalError
	if !errors.As(err, &got) || got.Issue.Message != "unexpected x" {
		t.Errorf("error = %v, want a JournalError carrying the message", err)
	}
}

// A located error without any message text still locates the issue.
func TestPrintToleratesMissingMessage(t *testing.T) {
	err := printErrors(t, "hledger: Error: /data/main.journal:1:1:\n  | ^\n")

	var got domain.JournalError
	if !errors.As(err, &got) || got.Issue.Message != "" || got.Issue.Line != 1 {
		t.Errorf("error = %v, want a located JournalError with an empty message", err)
	}
}
