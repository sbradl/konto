package domain

import (
	"reflect"
	"testing"
	"time"
)

func occurrence(rule SourcePos, date string, postings ...Posting) Transaction {
	parsed, _ := time.Parse(time.DateOnly, date)

	return Transaction{Date: parsed, Source: rule, Postings: postings}
}

func scheduleLedger() Ledger {
	rent, salary := SourcePos{"j", 10}, SourcePos{"j", 14}
	transfer, never := SourcePos{"j", 18}, SourcePos{"j", 22}

	return Ledger{
		Rules: []PeriodicRule{
			{Description: "Never", Pos: never},
			{Description: "Never too", Pos: SourcePos{"j", 26}},
			{Description: "Salary", Pos: salary},
			{Description: "Transfer", Pos: transfer},
			{Description: "Rent", Pos: rent},
		},
		Upcoming: []Transaction{
			occurrence(rent, "2026-10-01", posting("expenses:rent", 90000), posting("assets:checking", -90000)),
			occurrence(salary, "2026-10-25", posting("assets:checking", 300000), posting("income:salary", -300000)),
			occurrence(rent, "2026-11-01", posting("expenses:rent", 90000), posting("assets:checking", -90000)),
			occurrence(transfer, "2026-10-26", posting("assets:savings", 50000), posting("assets:checking", -50000)),
		},
	}
}

func TestSchedulesAreSortedByNextOccurrenceWithNothingUpcomingLast(t *testing.T) {
	schedules := scheduleLedger().Schedules()
	order := make([]string, 0, len(schedules))
	for _, schedule := range schedules {
		order = append(order, schedule.Rule.Description)
	}

	if want := []string{"Rent", "Salary", "Transfer", "Never", "Never too"}; !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestEachScheduleHoldsItsOwnOccurrencesSoonestFirst(t *testing.T) {
	ledger := scheduleLedger()
	ledger.Upcoming[0], ledger.Upcoming[2] = ledger.Upcoming[2], ledger.Upcoming[0] // out of order

	rent := ledger.Schedules()[0]

	next, ok := rent.Next()
	if len(rent.Occurrences) != 2 || !ok || next.Format(time.DateOnly) != "2026-10-01" {
		t.Errorf("rent = %+v, want two occurrences from 2026-10-01", rent)
	}

	if _, ok := ledger.Schedules()[3].Next(); ok {
		t.Errorf("a rule without occurrences has no next date")
	}
}

func TestImpactIsTheSignedChangeOfNetWorth(t *testing.T) {
	schedules := scheduleLedger().Schedules()

	got := map[string]string{}
	for _, schedule := range schedules {
		got[schedule.Rule.Description] = schedule.Impact().Format()
	}

	want := map[string]string{"Rent": "-1.800,00 EUR", "Salary": "3.000,00 EUR", "Transfer": "0,00 EUR", "Never": "0,00 EUR", "Never too": "0,00 EUR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("impacts = %v, want %v", got, want)
	}
}

func TestAmountIsTheNextOccurrencesChangeOfNetWorth(t *testing.T) {
	got := map[string]string{}
	for _, schedule := range scheduleLedger().Schedules() {
		got[schedule.Rule.Description] = schedule.Amount().Format()
	}

	want := map[string]string{"Rent": "-900,00 EUR", "Salary": "3.000,00 EUR", "Transfer": "0,00 EUR", "Never": "0,00 EUR", "Never too": "0,00 EUR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("amounts = %v, want %v", got, want)
	}
}

func TestAnnualImpactTotalsEveryRule(t *testing.T) {
	if got := scheduleLedger().AnnualImpact().Format(); got != "1.200,00 EUR" {
		t.Errorf("annual impact = %s, want 1.200,00 EUR", got)
	}
}

func TestAnEntryIsTheBlockAsWritten(t *testing.T) {
	rule := PeriodicRule{Lines: []string{"~ monthly  A", "    x  1,00 EUR", "    y"}}

	if got := rule.Entry(); got != "~ monthly  A\n    x  1,00 EUR\n    y" {
		t.Errorf("entry = %q", got)
	}
}

func TestDeletingARuleRemovesTheBlockAtItsPosition(t *testing.T) {
	rule := PeriodicRule{Pos: SourcePos{File: "j", Line: 7}, Lines: []string{"~ monthly  Rent", "    a"}}

	got := rule.Delete()

	if len(got.Edits) != 1 || got.Edits[0] != (DeleteRule{Pos: SourcePos{File: "j", Line: 7}, Was: "~ monthly  Rent\n    a"}) {
		t.Errorf("changeset = %+v, want one DeleteRule at j:7", got)
	}
}

func TestCompareNextPutsRulesWithoutAnOccurrenceLast(t *testing.T) {
	soon := RuleSchedule{Occurrences: []Transaction{{Date: day(1)}}}
	later := RuleSchedule{Occurrences: []Transaction{{Date: day(9)}}}
	never := RuleSchedule{}

	for name, tc := range map[string]struct {
		a, b RuleSchedule
		want int
	}{
		"earlier first": {soon, later, -1}, "later after": {later, soon, 1}, "equal": {soon, soon, 0},
		"with before without": {soon, never, -1}, "without after with": {never, soon, 1}, "neither": {never, never, 0},
	} {
		if got := compareNext(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: compareNext = %d, want %d", name, got, tc.want)
		}
	}
}

func booked(date string, postings ...Posting) Transaction {
	return occurrence(SourcePos{"j", 99}, date, postings...)
}

func rentOn(date string) Transaction {
	return occurrence(SourcePos{"j", 10}, date, posting("expenses:rent", 90000), posting("assets:checking", -90000))
}

func dates(transactions []Transaction) []string {
	list := make([]string, len(transactions))
	for i, transaction := range transactions {
		list[i] = transaction.Date.Format(time.DateOnly)
	}

	return list
}

func TestAPastOccurrenceNothingBookedCoversIsOverdue(t *testing.T) {
	ledger := Ledger{Past: []Transaction{rentOn("2026-09-01"), rentOn("2026-08-01")}}

	if got := dates(ledger.Overdue()); !reflect.DeepEqual(got, []string{"2026-08-01", "2026-09-01"}) {
		t.Errorf("overdue = %v, want both, oldest first", got)
	}
}

func TestABookingInTheSameCategoryWithTheSameAmountNearTheDueDateCoversAnOccurrence(t *testing.T) {
	ledger := Ledger{
		Past: []Transaction{rentOn("2026-08-01"), rentOn("2026-09-01")},
		Transactions: []Transaction{
			booked("2026-07-25", posting("expenses:rent", 90000), posting("assets:cash", -90000)),     // a week early
			booked("2026-08-20", posting("expenses:rent", 80000), posting("assets:checking", -80000)), // other amount
			booked("2026-08-21", posting("expenses:food", 90000), posting("assets:checking", -90000)), // other category
			booked("2026-08-24", posting("expenses:rent", 90000), posting("assets:checking", -90000)), // too early for 09-01
		},
	}

	if got := dates(ledger.Overdue()); !reflect.DeepEqual(got, []string{"2026-09-01"}) {
		t.Errorf("overdue = %v, want only 2026-09-01", got)
	}
}

func TestOneBookingCoversOneOccurrence(t *testing.T) {
	ledger := Ledger{
		Past:         []Transaction{rentOn("2026-08-01"), rentOn("2026-09-01")},
		Transactions: []Transaction{booked("2026-09-02", posting("expenses:rent", 90000), posting("assets:checking", -90000))},
	}

	if got := dates(ledger.Overdue()); !reflect.DeepEqual(got, []string{"2026-09-01"}) {
		t.Errorf("overdue = %v, want the later one left", got)
	}
}

func TestATransferIsCoveredByItsFirstPosting(t *testing.T) {
	transfer := occurrence(SourcePos{"j", 18}, "2026-09-26", posting("assets:savings", 50000), posting("assets:checking", -50000))
	ledger := Ledger{
		Past:         []Transaction{transfer},
		Transactions: []Transaction{booked("2026-09-26", posting("assets:savings", 50000), posting("assets:checking", -50000))},
	}

	if got := ledger.Overdue(); len(got) != 0 {
		t.Errorf("overdue = %v, want none", dates(got))
	}
}

func TestAnOverdueRuleComesFirstWithItsOldestOverdueAmount(t *testing.T) {
	ledger := scheduleLedger()
	ledger.Past = []Transaction{
		occurrence(SourcePos{"j", 22}, "2026-09-15", posting("expenses:gym", 3000), posting("assets:checking", -3000)),
	}
	never := ledger.Schedules()[0]

	if never.Rule.Description != "Never" || !never.IsOverdue() || never.Amount().Format() != "-30,00 EUR" {
		t.Errorf("first schedule = %+v, want Never overdue at -30,00 EUR", never)
	}

	if due, _ := never.Due(); due.Format(time.DateOnly) != "2026-09-15" {
		t.Errorf("due = %v, want the overdue date", due)
	}

	if _, ok := never.Next(); ok {
		t.Errorf("an overdue date is not a next occurrence")
	}

	if ledger.Schedules()[1].IsOverdue() {
		t.Errorf("a rule without overdue occurrences is not overdue")
	}

	if got := never.Impact().Format(); got != "-30,00 EUR" {
		t.Errorf("impact = %s, want the overdue payment counted", got)
	}
}

func TestAnOccurrenceWithoutPostingsIsNeverCovered(t *testing.T) {
	ledger := Ledger{
		Past:         []Transaction{occurrence(SourcePos{"j", 30}, "2026-09-01")},
		Transactions: []Transaction{booked("2026-09-01", posting("expenses:rent", 0))},
	}

	if got := ledger.Overdue(); len(got) != 1 {
		t.Errorf("overdue = %v, want the empty occurrence", dates(got))
	}
}
