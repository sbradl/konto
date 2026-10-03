package hledger

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

const eurStyle = `{"ascommodityside": "R", "ascommodityspaced": true, "asdecimalmark": ",", "asdigitgroups": [".", [3]]}`

func amountJSON(commodity string, mantissa int, style string) string {
	return `{"acommodity": "` + commodity + `", "aquantity": {"decimalMantissa": ` + strconv.Itoa(mantissa) +
		`, "decimalPlaces": 2, "floatingPoint": 0}, "astyle": ` + style + `}`
}

func balanceReport(rows ...string) string { return "[[" + strings.Join(rows, ",") + "], []]" }

func row(name string, amounts ...string) string {
	return `["` + name + `", "` + name + `", 0, [` + strings.Join(amounts, ",") + `]]`
}

func TestBalancesReturnsEveryAccountWithItsAmounts(t *testing.T) {
	out := balanceReport(
		row("assets:checking", amountJSON("EUR", 1324000, eurStyle)),
		row("assets:empty"),
		row("assets:wallet", amountJSON("PLN", -500, eurStyle), amountJSON("USD", 100, `{"ascommodityside": "L", "ascommodityspaced": false, "asdecimalmark": ".", "asdigitgroups": null}`)),
	)
	client := newClient(recording(out, nil, new([]call)), "/data/main.journal")

	got, err := client.Balances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	euro := domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}
	want := []domain.Account{
		{Name: "assets:checking", Balance: domain.MultiAmount{{Commodity: "EUR", Mantissa: 1324000, Places: 2, Style: euro}}},
		{Name: "assets:empty", Balance: domain.MultiAmount{}},
		{Name: "assets:wallet", Balance: domain.MultiAmount{
			{Commodity: "PLN", Mantissa: -500, Places: 2, Style: euro},
			{Commodity: "USD", Mantissa: 100, Places: 2, Style: domain.AmountStyle{SymbolLeft: true, DecimalMark: "."}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("accounts =\n%+v\nwant\n%+v", got, want)
	}
}

func TestBalancesAsksForAFlatReportIncludingEmptyAccounts(t *testing.T) {
	var calls []call
	client := newClient(recording(balanceReport(), nil, &calls), "/data/main.journal")

	if _, err := client.Balances(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "/data/main.journal", "bal", "--flat", "-O", "json", "-E"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}

func TestBalancesReportsHledgerFailure(t *testing.T) {
	client := newClient(recording("", errors.New("exit status 1"), new([]call)), "/data/main.journal")

	if _, err := client.Balances(context.Background()); err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected an error carrying the cause, got: %v", err)
	}
}

func TestBalancesRejectsGarbage(t *testing.T) {
	for name, out := range map[string]string{
		"not json":            "oops",
		"empty report":        "[]",
		"rows not a list":     `[{}, []]`,
		"row not a list":      `[[{}], []]`,
		"short row":           `[[["assets:a", "assets:a", 0]], []]`,
		"name not a string":   `[[[1, "a", 0, []]], []]`,
		"amounts not a list":  `[[["a", "a", 0, 5]], []]`,
		"digit mark not text": balanceReport(row("a", amountJSON("EUR", 1, `{"asdigitgroups": [1, [3]]}`))),
		"no group sizes":      balanceReport(row("a", amountJSON("EUR", 1, `{"asdigitgroups": [".", []]}`))),
		"sizes not numbers":   balanceReport(row("a", amountJSON("EUR", 1, `{"asdigitgroups": [".", "x"]}`))),
	} {
		client := newClient(recording(out, nil, new([]call)), "/data/main.journal")

		if _, err := client.Balances(context.Background()); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

func TestPeriodActivityAsksForIncomeAndExpensesOverTheInclusivePeriod(t *testing.T) {
	var calls []call
	client := newClient(recording(balanceReport(row("expenses:rent", amountJSON("EUR", 90000, eurStyle))), nil, &calls), "/data/main.journal")

	got, err := client.PeriodActivity(context.Background(),
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "/data/main.journal", "bal", "--flat", "-O", "json", "-b", "2026-09-01", "-e", "2026-10-01", "^expenses:", "^income:"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
	if len(got) != 1 || got[0].Name != "expenses:rent" {
		t.Errorf("accounts = %+v, want expenses:rent", got)
	}
}

func TestActivityAsksForTheGivenQueriesOverTheInclusivePeriod(t *testing.T) {
	var calls []call
	client := newClient(recording(balanceReport(), nil, &calls), "/data/main.journal")

	_, err := client.Activity(context.Background(),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), "^assets:", "^liabilities:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "/data/main.journal", "bal", "--flat", "-O", "json", "-b", "2026-07-01", "-e", "2026-10-01", "^assets:", "^liabilities:"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("calls = %+v, want one call with %v", calls, want)
	}
}

func TestBalancesUntilCountsEverythingUpToAndIncludingTheDay(t *testing.T) {
	var calls []call
	client := newClient(recording(balanceReport(row("assets:a", amountJSON("EUR", 100, eurStyle))), nil, &calls), "/data/main.journal")

	got, err := client.BalancesUntil(context.Background(), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), "^assets:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "/data/main.journal", "bal", "--flat", "-O", "json", "-e", "2026-10-01", "^assets:"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].args, want) || len(got) != 1 {
		t.Errorf("calls = %+v, got %+v, want one call with %v", calls, got, want)
	}
}

func periodReport(ends []string, rows ...string) string {
	dates := make([]string, len(ends))
	for i, end := range ends {
		dates[i] = `[{"contents": "2026-01-01", "tag": "Exact"}, {"contents": "` + end + `", "tag": "Exact"}]`
	}
	return `{"prDates": [` + strings.Join(dates, ",") + `], "prRows": [` + strings.Join(rows, ",") + `]}`
}

func periodRow(periods ...[]string) string {
	cells := make([]string, len(periods))
	for i, amounts := range periods {
		cells[i] = "[" + strings.Join(amounts, ",") + "]"
	}
	return `{"prrName": "assets:wallet", "prrAmounts": [` + strings.Join(cells, ",") + `]}`
}

func TestProjectionReadsTheMonthEndBalancesOfOneAccount(t *testing.T) {
	var calls []call
	out := periodReport([]string{"2026-09-30", "2026-10-30"}, periodRow(
		nil,
		[]string{amountJSON("EUR", 5000, eurStyle), amountJSON("PLN", 40000, eurStyle)},
	))
	client := newClient(recording(out, nil, &calls), "/data/main.journal")

	got, err := client.Projection(context.Background(), domain.ProjectionQuery{
		Account: "assets:wallet", Begin: time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		ForecastFrom: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC), ForecastTo: time.Date(2027, 9, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantEnds := []time.Time{time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)}
	if !reflect.DeepEqual(got.Ends, wantEnds) {
		t.Errorf("ends = %v, want %v", got.Ends, wantEnds)
	}
	if len(got.Balances[0]) != 0 || len(got.Balances[1]) != 2 || got.Balances[1][1].Commodity != "PLN" ||
		got.Balances[1][1].Mantissa != 40000 {
		t.Errorf("balances = %+v", got.Balances)
	}
	want := []string{
		"-f", "/data/main.journal", "bal", `acct:^assets:wallet$`, "-M", "-H", "-E", "-I", "--layout=bare", "-b", "2026-03-30",
		"--forecast=2026-09-27..2027-09-30", "-O", "json",
	}
	if !reflect.DeepEqual(calls[0].args, want) {
		t.Errorf("args = %v, want %v", calls[0].args, want)
	}
}

func TestProjectionEscapesTheAccountNameInTheQuery(t *testing.T) {
	var calls []call
	client := newClient(recording(periodReport(nil), nil, &calls), "/data/main.journal")

	if _, err := client.Projection(context.Background(), domain.ProjectionQuery{Account: "assets:a.b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := calls[0].args[3]; got != `acct:^assets:a\.b$` {
		t.Errorf("query = %q", got)
	}
}

func TestProjectionOfAnAccountWithoutPostingsHasPeriodsButNoBalances(t *testing.T) {
	client := newClient(recording(periodReport([]string{"2026-09-30"}), nil, new([]call)), "j")

	got, err := client.Projection(context.Background(), domain.ProjectionQuery{Account: "assets:none"})

	if err != nil || len(got.Ends) != 1 || len(got.Balances) != 1 || len(got.Balances[0]) != 0 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestProjectionReportsWhatItCannotRead(t *testing.T) {
	badAmount := `{"acommodity": "EUR", "aquantity": {"decimalMantissa": 1, "decimalPlaces": 2}, "astyle": {"asdigitgroups": [".", []]}}`
	for name, client := range map[string]*Client{
		"hledger fails": newClient(recording("", errors.New("exit status 1"), new([]call)), "j"),
		"not json":      newClient(recording("oops", nil, new([]call)), "j"),
		"bad date":      newClient(recording(periodReport([]string{"soon"}), nil, new([]call)), "j"),
		"bad amount":    newClient(recording(periodReport([]string{"2026-09-30"}, periodRow([]string{badAmount})), nil, new([]call)), "j"),
	} {
		if _, err := client.Projection(context.Background(), domain.ProjectionQuery{Account: "assets:x"}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
