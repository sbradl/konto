package hledger

import (
	"context"
	"errors"
	"io"
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

var euroStyle = domain.DefaultAmount("EUR").Style

type call struct{ args []string }

// newClient is a Client that runs run on the journal of the files.
func newClient(run runner, files ...string) *Client {
	return &Client{run: run, files: func() []string { return files }}
}

func recording(out string, err error, calls *[]call) runner {
	return recordingStderr(out, "", err, calls)
}

func recordingStderr(out, stderr string, err error, calls *[]call) runner {
	return func(_ context.Context, _ io.Reader, args ...string) ([]byte, string, error) {
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
		{Date: date(2026, 1, 1), Description: "opening balance", Postings: []domain.Posting{}},
		{Date: date(2026, 9, 14), Description: "Rent", Postings: []domain.Posting{
			{Account: "expenses:rent", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: 90000, Places: 2, Style: euroStyle}}},
			{Account: "assets:checking", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: -90000, Places: 2, Style: euroStyle}}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transactions = %+v, want %+v", got, want)
	}
}

// Every hledger call names the journal (spec §7.1): a single file with -f, several (ADR 0013) as `-f -` with
// standard input including each of them.
func TestPrintNamesTheJournalExplicitly(t *testing.T) {
	for name, tc := range map[string]struct {
		files     []string
		wantArgs  []string
		wantInput string
	}{
		"one file": {[]string{"/data/main.journal"}, []string{"-f", "/data/main.journal", "print", "-O", "json"}, ""},
		"several files": {
			[]string{"/data/main.journal", "/data/2026.journal"},
			[]string{"-f", "-", "print", "-O", "json"},
			"include /data/main.journal\ninclude /data/2026.journal\n",
		},
	} {
		var input string
		var args []string
		client := newClient(inputReturning("[]", "", nil, &input, &args), tc.files...)

		if _, err := client.Print(context.Background()); err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}

		if !reflect.DeepEqual(args, tc.wantArgs) || input != tc.wantInput {
			t.Errorf("%s: args = %v, input = %q; want %v and %q", name, args, input, tc.wantArgs, tc.wantInput)
		}
	}
}

// A journal without transactions prints an empty list, not an error.
func TestPrintOfEmptyJournalIsEmpty(t *testing.T) {
	client := answering("[]")

	got, err := client.Print(context.Background())

	if err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v; want no transactions and no error", got, err)
	}
}

// A failing hledger call is reported with its cause.
func TestPrintReportsHledgerFailure(t *testing.T) {
	client := failingHledger()

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
		client := answering(out)

		if _, err := client.Print(context.Background()); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// New runs the named hledger binary, passing its arguments through.
func TestNewRunsTheHledgerBinary(t *testing.T) {
	fakeHledger(t, printJSON, 0)

	got, err := New("hledger", func() []string { return []string{"/data/main.journal"} }).Print(context.Background())

	if err != nil || len(got) != 2 {
		t.Errorf("got %+v, %v; want two transactions and no error", got, err)
	}
}

// The real runner reports a binary that fails.
func TestNewReportsBinaryFailure(t *testing.T) {
	fakeHledger(t, "boom", 3)

	if _, err := New("hledger", func() []string { return []string{"/data/main.journal"} }).Print(context.Background()); err == nil {
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

	_, err := New("hledger", func() []string { return []string{"/data/main.journal"} }).Print(context.Background())

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

var upcomingJSON = `[
  {"tdate": "2026-09-14", "tdescription": "Rent", "ttags": [[]], "tpostings": []},
  {"tdate": "2026-10-01", "tdescription": "Vermieter Mueller", "ttags": [["_generated-transaction", "~ monthly from 2026-10-01"]],
   "tsourcepos": [{"sourceName": "/data/main.journal", "sourceLine": 10}, {"sourceName": "/data/main.journal", "sourceLine": 13}], "tpostings": [
    {"paccount": "expenses:rent", "pamount": [` + amountJSON("EUR", 90000, eurStyle) + `]}
  ]}
]`

// Upcoming asks for the occurrences over an explicit period and keeps only the generated occurrences.
func TestUpcomingReturnsOnlyGeneratedOccurrencesInThePeriod(t *testing.T) {
	var calls []call
	client := newClient(recording(upcomingJSON, nil, &calls), "/data/main.journal")
	from, to := date(2026, 9, 30), date(2027, 9, 30)

	got, err := client.Upcoming(context.Background(), from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantArgs := []string{"-f", "/data/main.journal", "print", "--forecast=2026-09-30..2027-09-30", "-I", "-O", "json"}
	assertOneCall(t, calls, wantArgs)

	if len(got) == 1 && got[0].Source != (domain.SourcePos{File: "/data/main.journal", Line: 10}) {
		t.Errorf("source = %+v, want the `~` line", got[0].Source)
	}

	if len(got) != 1 || got[0].Description != "Vermieter Mueller" || got[0].Date != date(2026, 10, 1) {
		t.Errorf("occurrences = %+v, want only the generated Vermieter Mueller", got)
	}
}

func TestUpcomingReportsFailureAndGarbage(t *testing.T) {
	when := date(2026, 9, 30)
	for name, client := range map[string]*Client{
		"hledger fails": failingHledger(),
		"not json":      answering("oops"),
		"bad date":      answering(`[{"tdate": "x", "ttags": [["_generated-transaction", ""]]}]`),
	} {
		if _, err := client.Upcoming(context.Background(), when, when); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// hasConvertArgs reports the shape of a CSV conversion: print -f <csv> --rules-file <rules> -O json.
func hasConvertArgs(args []string) bool {
	return len(args) == 7 && args[0] == "print" && args[1] == "-f" && args[2] == "/downloads/sept.csv" &&
		args[3] == "--rules-file" && args[5] == "-O"
}

func TestConvertCSVRunsHledgerOnTheFileWithTheRulesAndReadsComments(t *testing.T) {
	out := `[{"tdate": "2026-09-30", "tdescription": "REWE", "tcomment": " Einkauf\n", "tpostings": [
	  {"paccount": "expenses:groceries", "pamount": [` + amountJSON("EUR", 4217, eurStyle) + `], "ptags": [["konto-rule", "2"]]},
	  {"paccount": "assets:checking", "pamount": [` + amountJSON("EUR", -4217, eurStyle) + `], "ptags": []}]}]`
	var rulesSeen string
	run := func(_ context.Context, _ io.Reader, args ...string) ([]byte, string, error) {
		if !hasConvertArgs(args) {
			t.Errorf("args = %v", args)
		}
		b, _ := os.ReadFile(args[4])
		rulesSeen = string(b)

		return []byte(out), "", nil
	}

	got, err := newClient(run, "/data/main.journal").ConvertCSV(context.Background(), "/downloads/sept.csv", "skip 7\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rulesSeen != "skip 7\n" {
		t.Errorf("rules file held %q", rulesSeen)
	}

	if len(got) != 1 || got[0].Comment != "Einkauf" || got[0].Postings[0].Tags["konto-rule"] != "2" || got[0].Postings[1].Tags != nil {
		t.Errorf("transactions = %+v, want the comment and the rule tag", got)
	}
}

func TestConvertCSVReportsARulesFileThatCannotBeWritten(t *testing.T) {
	var calls []call
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	_, err := newClient(recording("[]", nil, &calls), "j").ConvertCSV(context.Background(), "x.csv", "")

	if err == nil || !strings.Contains(err.Error(), "writing CSV rules") || len(calls) != 0 {
		t.Errorf("err = %v, calls = %v; want the rules file's failure and no hledger call", err, calls)
	}
}

func TestConvertCSVRemovesItsRulesFile(t *testing.T) {
	var path string
	run := func(_ context.Context, _ io.Reader, args ...string) ([]byte, string, error) {
		path = args[4]

		return []byte("[]"), "", nil
	}

	if _, err := newClient(run, "j").ConvertCSV(context.Background(), "x.csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the temporary rules file %s should be gone, stat gave %v", path, err)
	}
}

func TestConvertCSVReportsHledgersFailureAndGarbage(t *testing.T) {
	failing := newClient(recordingStderr("", "bad rules", errors.New("exit status 1"), new([]call)), "j")
	if _, err := failing.ConvertCSV(context.Background(), "x.csv", ""); err == nil || !strings.Contains(err.Error(), "bad rules") {
		t.Errorf("expected hledger's own words, got %v", err)
	}

	garbage := answering("not json")
	if _, err := garbage.ConvertCSV(context.Background(), "x.csv", ""); err == nil {
		t.Errorf("expected an error for unreadable output")
	}
}

func TestTagMapSkipsMalformedTagsAndIsNilWithoutAny(t *testing.T) {
	if got := tagMap([][]string{{"only-a-key"}, {}}); got != nil {
		t.Errorf("malformed tags give %v, want nil", got)
	}

	if got := tagMap([][]string{{"konto-rule", "2"}, {"broken"}}); len(got) != 1 || got["konto-rule"] != "2" {
		t.Errorf("tags = %v", got)
	}
}

func TestReadTransactionsKeepsTheTransactionTags(t *testing.T) {
	out := `[{"tdate": "2027-01-01", "tdescription": "opening balance", "tcomment": "konto:carry\n",
	  "ttags": [["konto", "carry"]], "tpostings": []}]`

	got, err := readTransactions([]byte(out), nil)

	if err != nil || len(got) != 1 || !got[0].IsCarry() {
		t.Errorf("transactions = %+v, %v; want the carry, read from its tags", got, err)
	}
}

// Balance assertions are checked per -f file but across an include, so a journal of several files reaches
// hledger as one journal on standard input that includes them in order.
func TestQueriesGiveSeveralFilesAsOneJournalThatIncludesThem(t *testing.T) {
	var input string
	var args []string
	client := newClient(nil, "/data/main.journal", "/data/2025.journal", "/data/my 2026.journal")
	client.run = inputReturning(balanceReport(), "", nil, &input, &args)

	if _, err := client.Balances(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantArgs := []string{"-f", "-", "bal", "--flat", "-O", "json", "-E"}

	wantInput := "include /data/main.journal\ninclude /data/2025.journal\ninclude /data/my 2026.journal\n"
	if !reflect.DeepEqual(args, wantArgs) || input != wantInput {
		t.Errorf("args = %v, input = %q; want %v and %q", args, input, wantArgs, wantInput)
	}
}

// A single file is named with -f as it is.
func TestQueriesNameASingleFileWithF(t *testing.T) {
	var calls []call
	client := newClient(recording(balanceReport(), nil, &calls), "/data/main.journal")

	if _, err := client.Balances(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := []string{"-f", "/data/main.journal", "bal", "--flat", "-O", "json", "-E"}; len(calls) != 1 ||
		!reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}

// The files are asked for on every query: a year file created meanwhile joins the next one.
func TestTheFilesAreResolvedOnEveryQuery(t *testing.T) {
	var input string
	var args []string
	files := make([]string, 1, 2)
	files[0] = "/data/main.journal"
	client := newClient(inputReturning(balanceReport(), "", nil, &input, &args))
	client.files = func() []string { return files }

	_, _ = client.Balances(context.Background())
	firstArgs := args
	files = append(files, "/data/2026.journal")
	_, _ = client.Balances(context.Background())

	if firstArgs[1] != "/data/main.journal" || input != "include /data/main.journal\ninclude /data/2026.journal\n" {
		t.Errorf("first args = %v, input = %q; want the second query to include the new file", firstArgs, input)
	}
}

func TestSeveralFilesReportHledgerFailure(t *testing.T) {
	client := newClient(nil, "/data/main.journal", "/data/2026.journal")
	client.run = inputReturning("", "boom", errors.New("exit status 1"), new(string), new([]string))

	if _, err := client.Balances(context.Background()); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected an error carrying hledger's message, got: %v", err)
	}
}

// A hung hledger fails its call instead of keeping the view waiting forever.
func TestACallThatOutlivesItsTimeLimitFails(t *testing.T) {
	for name, run := range map[string]func() error{
		"query": func() error {
			_, _, err := binary{"sleep", 20 * time.Millisecond}.run(context.Background(), nil, "5")

			return err
		},
		"with input": func() error {
			_, _, err := binary{"sleep", 20 * time.Millisecond}.run(context.Background(), strings.NewReader("text"), "5")

			return err
		},
	} {
		start := time.Now()
		err := run()

		if err == nil || !strings.Contains(err.Error(), "did not answer within 20ms") {
			t.Errorf("%s: error = %v, want it to say hledger did not answer in time", name, err)
		}

		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("%s: the call took %s, want it cut off", name, took)
		}
	}
}

// answering is a Client whose hledger prints out and succeeds.
func answering(out string) *Client {
	return newClient(recording(out, nil, new([]call)), "/data/main.journal")
}

// failingHledger is a Client whose hledger exits with status 1.
func failingHledger() *Client {
	return newClient(recording("", errors.New("exit status 1"), new([]call)), "/data/main.journal")
}

// date is midnight UTC on the day, as hledger's dates are read.
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// assertOneCall fails the test unless hledger was called exactly once, with want.
func assertOneCall(t *testing.T, calls []call, want []string) {
	t.Helper()

	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}
