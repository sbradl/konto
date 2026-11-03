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
	rule := PeriodicRule{Pos: SourcePos{File: "j", Line: 7}}

	got := rule.Delete()

	if len(got.Edits) != 1 || got.Edits[0] != (DeleteRule{Pos: SourcePos{File: "j", Line: 7}}) {
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
