package hledger

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

const generatedJSON = `[
  {"tdate": "2026-10-15", "tdescription": "Gym", "ttags": [["_generated-transaction", "~ monthly"]], "tpostings": []},
  {"tdate": "2026-09-01", "tdescription": "real", "ttags": [], "tpostings": []},
  {"tdate": "2026-11-15", "tdescription": "Gym", "ttags": [["_generated-transaction", "~ monthly"]], "tpostings": []}
]`

// inputReturning is a runner that keeps what it read on stdin and its args, then answers out, stderr and err.
func inputReturning(out, stderr string, err error, gotInput *string, gotArgs *[]string) runner {
	return func(_ context.Context, stdin io.Reader, args ...string) ([]byte, string, error) {
		*gotInput, *gotArgs = readAll(stdin), args

		return []byte(out), stderr, err
	}
}

// readAll is what stdin holds; nothing when there is none.
func readAll(stdin io.Reader) string {
	if stdin == nil {
		return ""
	}
	text, _ := io.ReadAll(stdin)

	return string(text)
}

func TestOccurrencesAreTheDatesOfTheGeneratedTransactions(t *testing.T) {
	var input string
	var args []string
	client := &Client{run: inputReturning(generatedJSON, "", nil, &input, &args)}

	got, err := client.Occurrences(context.Background(), "~ monthly  Gym\n", date(2026, 10, 1), date(2027, 10, 1))

	want := []time.Time{date(2026, 10, 15), date(2026, 11, 15)}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("dates = %v, %v; want %v", got, err, want)
	}

	if input != "~ monthly  Gym\n" || !reflect.DeepEqual(args, []string{
		"-f", "-", "print", "--forecast=2026-10-01..2027-10-01", "-O", "json",
	}) {
		t.Errorf("input %q, args %v", input, args)
	}
}

func TestOccurrencesReportWhatHledgerSaysAboutARuleItCannotRead(t *testing.T) {
	client := hledgerOnText("", "bad period expression\n", errors.New("exit 1"))

	_, err := client.Occurrences(context.Background(), "~ nonsense\n", time.Time{}, time.Time{})

	if err == nil || !strings.Contains(err.Error(), "bad period expression") {
		t.Errorf("error = %v", err)
	}
}

func TestOccurrencesReportOutputItCannotRead(t *testing.T) {
	client := hledgerOnText("oops", "", nil)

	if _, err := client.Occurrences(context.Background(), "", time.Time{}, time.Time{}); err == nil {
		t.Error("expected an error for output that is no JSON")
	}
}

// hledgerOnText is a Client for journal text whose hledger answers out, stderr and err.
func hledgerOnText(out, stderr string, err error) *Client {
	return &Client{run: inputReturning(out, stderr, err, new(string), new([]string))}
}
