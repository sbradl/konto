package hledger

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

const printJSON = `[
  {"tdate": "2026-01-01", "tdescription": "opening balance", "tindex": 1, "tpostings": []},
  {"tdate": "2026-09-14", "tdescription": "Rent", "tindex": 2, "tpostings": []}
]`

type call struct{ args []string }

func recording(out string, err error, calls *[]call) runner {
	return func(_ context.Context, args ...string) ([]byte, error) {
		*calls = append(*calls, call{args})
		return []byte(out), err
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
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Description: "opening balance"},
		{Date: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), Description: "Rent"},
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
