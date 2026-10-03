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
