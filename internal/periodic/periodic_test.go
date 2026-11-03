package periodic

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

type fakeGateway struct {
	dates []time.Time
	err   error
	texts []string
	spans [][2]time.Time
}

func (f *fakeGateway) Occurrences(_ context.Context, text string, first, last time.Time) ([]time.Time, error) {
	f.texts = append(f.texts, text)
	f.spans = append(f.spans, [2]time.Time{first, last})
	return f.dates, f.err
}

func today() time.Time { return day("2026-09-30") }

var euro = domain.DefaultAmount("EUR").Style

func ledger() domain.Ledger {
	return domain.Ledger{
		Accounts: []domain.Account{{Name: "assets:checking", Balance: domain.MultiAmount{
			{Commodity: "EUR", Mantissa: 1324000, Places: 2, Style: euro},
		}}},
		Rules: []domain.PeriodicRule{
			{Description: "A", Pos: domain.SourcePos{File: "/j/main.journal", Line: 10}},
			{Description: "B", Pos: domain.SourcePos{File: "/j/rules.journal", Line: 3}},
		},
	}
}

func valid() Input {
	return Input{
		Date: "2026-10-15", Frequency: Monthly, Description: "Book club", Amount: "12,00", Currency: "EUR",
		From: "assets:checking", To: "expenses:books",
	}
}

func lines(t *testing.T, changeset domain.Changeset, err error) []string {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	switch edit := changeset.Edits[0].(type) {
	case domain.AddRule:
		return edit.Lines
	case domain.ReplaceRule:
		return edit.Lines
	}
	t.Fatalf("edit = %+v", changeset.Edits[0])
	return nil
}

func TestEachFrequencyWritesItsPeriodExpression(t *testing.T) {
	for frequency, header := range map[Frequency]string{
		Weekly: "~ weekly from 2026-10-15  Book club", Biweekly: "~ biweekly from 2026-10-15  Book club",
		Monthly: "~ monthly from 2026-10-15  Book club", Quarterly: "~ quarterly from 2026-10-15  Book club",
		HalfYearly: "~ every 6 months from 2026-10-15  Book club", Yearly: "~ yearly from 2026-10-15  Book club",
		Once: "~ 2026-10-15  Book club",
	} {
		input := valid()
		input.Frequency = frequency

		changeset, err := NewService(&fakeGateway{}, today).Add(context.Background(), input, ledger())

		want := []string{header, "    expenses:books  12,00 EUR", "    assets:checking"}
		if got := lines(t, changeset, err); !slices.Equal(got, want) {
			t.Errorf("%s: block = %q, want %q", frequency, got, want)
		}
	}
}

func TestTheAmountIsWrittenInTheJournalsStyle(t *testing.T) {
	for typed, want := range map[string]string{"1.234,50": "1.234,50 EUR", "12": "12,00 EUR", "0,5": "0,50 EUR"} {
		input := valid()
		input.Amount = typed

		changeset, err := NewService(&fakeGateway{}, today).Add(context.Background(), input, ledger())

		if got := lines(t, changeset, err)[1]; got != "    expenses:books  "+want {
			t.Errorf("%s: posting = %q", typed, got)
		}
	}
}

func TestRemainingPaymentsBecomeAnExclusiveEndOneDayAfterTheLastOccurrence(t *testing.T) {
	gw := &fakeGateway{dates: []time.Time{day("2026-10-15"), day("2026-11-15"), day("2026-12-15")}}
	input := valid()
	input.Remaining = "3"

	changeset, err := NewService(gw, today).Add(context.Background(), input, ledger())

	if got := lines(t, changeset, err)[0]; got != "~ monthly from 2026-10-15 to 2026-12-16  Book club" {
		t.Errorf("header = %q", got)
	}
	if want := "~ monthly from 2026-10-15  Book club\n    expenses:books  12,00 EUR\n    assets:checking\n"; gw.texts[0] != want {
		t.Errorf("hledger was asked about %q, want the rule without an end %q", gw.texts[0], want)
	}
	if !gw.spans[0][0].Equal(day("2026-10-15")) || !gw.spans[0][1].After(day("2029-10-15")) {
		t.Errorf("span = %v", gw.spans[0])
	}
}

func TestOnePaymentEndsTheDayAfter(t *testing.T) {
	gw := &fakeGateway{dates: []time.Time{day("2026-10-15"), day("2026-11-15")}}
	input := valid()
	input.Remaining = "1"

	changeset, err := NewService(gw, today).Add(context.Background(), input, ledger())

	if got := lines(t, changeset, err)[0]; got != "~ monthly from 2026-10-15 to 2026-10-16  Book club" {
		t.Errorf("header = %q", got)
	}
}

func TestOnceIgnoresARemainingCount(t *testing.T) {
	gw := &fakeGateway{}
	input := valid()
	input.Frequency, input.Remaining = Once, "abc"

	changeset, err := NewService(gw, today).Add(context.Background(), input, ledger())

	if got := lines(t, changeset, err)[0]; got != "~ 2026-10-15  Book club" || len(gw.texts) != 0 {
		t.Errorf("header = %q, asked hledger %d times", got, len(gw.texts))
	}
}

func TestReplacePlansTheNewBlockInPlace(t *testing.T) {
	rule := domain.PeriodicRule{Pos: domain.SourcePos{File: "/j/main.journal", Line: 10}}

	changeset, err := NewService(&fakeGateway{}, today).Replace(context.Background(), valid(), rule, ledger())

	replace, ok := changeset.Edits[0].(domain.ReplaceRule)
	if err != nil || !ok || replace.Pos != rule.Pos || len(replace.Lines) != 3 {
		t.Errorf("changeset = %+v, %v", changeset, err)
	}
}

func TestPlanningRefusesInputThatIsMissingOrWrong(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*Input)
		want   string
	}{
		"no date":        {func(in *Input) { in.Date = "" }, "next due date is required"},
		"no description": {func(in *Input) { in.Description = "" }, "description is required"},
		"no amount":      {func(in *Input) { in.Amount = "" }, "amount is required"},
		"no currency":    {func(in *Input) { in.Currency = "" }, "currency is required"},
		"no from":        {func(in *Input) { in.From = "" }, "from account is required"},
		"no to":          {func(in *Input) { in.To = "" }, "to account is required"},
		"same accounts":  {func(in *Input) { in.To = in.From }, "must differ"},
		"bad date":       {func(in *Input) { in.Date = "2026-13-45" }, "next due date must be a date"},
		"word as date":   {func(in *Input) { in.Date = "tomorrow" }, "next due date must be a date"},
		"letters":        {func(in *Input) { in.Amount = "abc" }, "amount must be a positive number"},
		"zero":           {func(in *Input) { in.Amount = "0" }, "amount must be a positive number"},
		"negative":       {func(in *Input) { in.Amount = "-5,00" }, "amount must be a positive number"},
		"decimals":       {func(in *Input) { in.Amount = "1,234" }, "amount must be a positive number"},
		"zero payments":  {func(in *Input) { in.Remaining = "0" }, "remaining payments must be a positive whole number"},
		"word payments":  {func(in *Input) { in.Remaining = "x" }, "remaining payments must be a positive whole number"},
	} {
		input := valid()
		tc.change(&input)

		_, err := NewService(&fakeGateway{}, today).Add(context.Background(), input, ledger())

		var refusal domain.RefusalError
		if !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, tc.want) {
			t.Errorf("%s: err = %v, want a refusal mentioning %q", name, err, tc.want)
		}
	}
}

func TestPlanningReportsHledgerFailingOrGivingTooFewDates(t *testing.T) {
	input := valid()
	input.Remaining = "3"

	failing := &fakeGateway{err: errors.New("hledger failed")}
	if _, err := NewService(failing, today).Add(context.Background(), input, ledger()); err == nil {
		t.Error("expected hledger's error")
	}
	short := &fakeGateway{dates: []time.Time{day("2026-10-15")}}
	if _, err := NewService(short, today).Add(context.Background(), input, ledger()); err == nil {
		t.Error("expected a refusal for a rule with fewer payments than asked for")
	}
}

func rule(lines ...string) domain.PeriodicRule {
	return domain.PeriodicRule{Lines: lines, Pos: domain.SourcePos{File: "/j/main.journal", Line: 4}}
}

func TestFormShowsTheRuleWithTheNextOccurrenceHledgerGives(t *testing.T) {
	gym := rule("~ monthly from 2026-01-15  Gym", "    expenses:rent  25,00 EUR", "    assets:checking")
	snapshot := ledger()
	snapshot.Rules = []domain.PeriodicRule{gym}
	snapshot.Upcoming = []domain.Transaction{{Date: day("2026-10-15"), Source: gym.Pos}}

	got, err := NewService(&fakeGateway{}, today).Form(context.Background(), gym, snapshot)

	want := Input{
		Date: "2026-10-15", Frequency: Monthly, Description: "Gym", Amount: "25,00", Currency: "EUR",
		From: "assets:checking", To: "expenses:rent",
	}
	if err != nil || got != want {
		t.Errorf("form = %+v, %v; want %+v", got, err, want)
	}
}

func TestFormKeepsTheStoredDateWhenNothingIsUpcoming(t *testing.T) {
	tax := rule("~ 2025-11-01  Tax", "    expenses:rent  5,00 EUR", "    assets:checking")

	got, err := NewService(&fakeGateway{}, today).Form(context.Background(), tax, domain.Ledger{Rules: []domain.PeriodicRule{tax}})

	if err != nil || got.Date != "2025-11-01" || got.Frequency != Once {
		t.Errorf("form = %+v, %v", got, err)
	}
}

func TestFormCountsTheRemainingPaymentsFromTodayUntilTheEnd(t *testing.T) {
	gw := &fakeGateway{dates: []time.Time{day("2026-10-15"), day("2026-11-15"), day("2026-12-15")}}
	gym := rule("~ monthly from 2026-10-15 to 2026-12-16  Gym", "    expenses:rent  25,00 EUR", "    assets:checking")

	got, err := NewService(gw, today).Form(context.Background(), gym, domain.Ledger{})

	if err != nil || got.Remaining != "3" {
		t.Errorf("form = %+v, %v", got, err)
	}
	if !gw.spans[0][0].Equal(day("2026-09-30")) || !gw.spans[0][1].Equal(day("2026-12-16")) ||
		gw.texts[0] != gym.Entry()+"\n" {
		t.Errorf("hledger was asked %q over %v", gw.texts[0], gw.spans[0])
	}
}

func TestFormReportsHledgerFailing(t *testing.T) {
	gw := &fakeGateway{err: errors.New("hledger failed")}
	gym := rule("~ monthly from 2026-10-15 to 2026-12-16  Gym", "    expenses:rent  25,00 EUR", "    assets:checking")

	if _, err := NewService(gw, today).Form(context.Background(), gym, domain.Ledger{}); err == nil {
		t.Error("expected hledger's error")
	}
}

func TestFormRefusesARuleThatDoesNotFit(t *testing.T) {
	for _, lines := range [][]string{
		{"~ every 2 weeks from 2026-10-20  Gym", "    expenses:rent  25,00 EUR", "    assets:checking"},
		{"~ monthly in 2026  Gym", "    expenses:rent  25,00 EUR", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym  ; note", "    expenses:rent  25,00 EUR", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR  ; tag:x", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR", "    expenses:books  5,00 EUR", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR", "    assets:checking  -25,00 EUR"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR @ 1,10 USD", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR = 100,00 EUR", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "expenses:rent  25,00 EUR", "    assets:checking"},
		{"~ monthly from 2026-10-20  Gym", "    expenses:rent  25,00 EUR"},
	} {
		_, err := NewService(&fakeGateway{}, today).Form(context.Background(), rule(lines...), domain.Ledger{})

		if !errors.Is(err, ErrNotEditable) {
			t.Errorf("%q: err = %v, want ErrNotEditable", lines[0]+" … "+lines[len(lines)-1], err)
		}
	}
}

// The form reads every style the amount is written in: the symbol left or right, spaced or not, any commodity.
func TestFormReadsTheAmountInEveryStyleItIsWritten(t *testing.T) {
	for text, want := range map[string][2]string{
		"25,00 EUR": {"25,00", "EUR"}, "$25.00": {"25.00", "$"}, "$ 1,025.00": {"1,025.00", "$"},
		"25,00€": {"25,00", "€"}, "-25,00 Kč": {"25,00", "Kč"}, "€-25": {"25", "€"},
	} {
		gym := rule("~ monthly from 2026-10-20  Gym", "    expenses:rent  "+text, "    assets:checking")

		got, err := NewService(&fakeGateway{}, today).Form(context.Background(), gym, domain.Ledger{})

		if err != nil || got.Amount != want[0] || got.Currency != want[1] {
			t.Errorf("%q: form = %+v, %v; want %v", text, got, err, want)
		}
	}
}

func TestFormNormalisesAnAmountOnTheSecondPosting(t *testing.T) {
	cases := map[string]struct{ from, to string }{
		"negative second": {"    expenses:rent|    assets:checking  -25,00 EUR", "assets:checking→expenses:rent"},
		"positive second": {"    assets:checking|    expenses:rent  25,00 EUR", "assets:checking→expenses:rent"},
	}
	for name, tc := range cases {
		postings := strings.Split(tc.from, "|")
		gym := rule("~ monthly from 2026-10-20  Gym", postings[0], postings[1])

		got, err := NewService(&fakeGateway{}, today).Form(context.Background(), gym, domain.Ledger{})

		if err != nil || got.From+"→"+got.To != tc.to || got.Amount != "25,00" {
			t.Errorf("%s: form = %+v, %v", name, got, err)
		}
	}
}

func TestFrequenciesAreTheDropdownInOrder(t *testing.T) {
	names := make([]string, 0, len(Frequencies()))
	for _, frequency := range Frequencies() {
		names = append(names, frequency.String())
	}
	if want := []string{"once", "weekly", "biweekly", "monthly", "quarterly", "half-yearly", "yearly"}; !slices.Equal(names, want) {
		t.Errorf("frequencies = %v", names)
	}
	if got, ok := frequencyOf("every 6 months"); !ok || got != HalfYearly {
		t.Errorf("every 6 months = %v, %v", got, ok)
	}
	if _, ok := frequencyOf("every 2 weeks"); ok {
		t.Error("every 2 weeks is no dropdown choice")
	}
}

func TestReplaceRefusesInputThatIsWrong(t *testing.T) {
	input := valid()
	input.Description = ""

	if _, err := NewService(&fakeGateway{}, today).Replace(context.Background(), input, domain.PeriodicRule{}, ledger()); err == nil {
		t.Error("expected a refusal")
	}
}

func TestInputFromCopiesATransactionIntoANewMonthlyRule(t *testing.T) {
	style := domain.DefaultAmount("EUR").Style
	rent := domain.Transaction{
		Date: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Description: "Vermieter Mueller",
		Postings: []domain.Posting{
			{Account: "expenses:rent", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: 120000, Places: 2, Style: style}}},
			{Account: "assets:checking", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: -120000, Places: 2, Style: style}}},
			{Account: "assets:other", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: -1, Places: 2, Style: style}}},
			{Account: "expenses:other", Amounts: domain.MultiAmount{{Commodity: "EUR", Mantissa: 1, Places: 2, Style: style}}},
		},
	}
	want := Input{
		Date: "2026-10-02", Frequency: Monthly, Description: "Vermieter Mueller",
		Amount: "1.200,00", Currency: "EUR", From: "assets:checking", To: "expenses:rent",
	}
	if got := InputFrom(rent); got != want {
		t.Errorf("InputFrom = %+v, want %+v", got, want)
	}
	if got := InputFrom(domain.Transaction{Description: "empty", Postings: []domain.Posting{{Account: "a"}}}); got.From != "" || got.To != "" || got.Amount != "" {
		t.Errorf("a posting without an amount fills nothing: %+v", got)
	}
}
