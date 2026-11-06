package domain

import (
	"slices"
	"strings"
	"time"
)

// SourcePos is a line of a journal file.
type SourcePos struct {
	File string
	Line int
}

// PeriodicRule is a `~` block of the journal: its raw period expression, its description and
// its lines as written, comments included. konto never computes a rule's dates; hledger does.
type PeriodicRule struct {
	Period      string
	Description string
	Lines       []string // the block as written: the `~` line, then its indented lines
	Pos         SourcePos
}

// RuleSchedule is a rule with the occurrences hledger projects for it over the next twelve months.
type RuleSchedule struct {
	Rule        PeriodicRule
	Occurrences []Transaction // soonest first
}

// Next is the date of the rule's next occurrence; false when none falls in the next twelve months.
func (s RuleSchedule) Next() (time.Time, bool) {
	if len(s.Occurrences) == 0 {
		return time.Time{}, false
	}

	return s.Occurrences[0].Date, true
}

// Impact is the signed change the rule brings to net worth over its occurrences: what it does to
// asset and liability accounts. A transfer between own accounts nets to zero.
func (s RuleSchedule) Impact() MultiAmount {
	var total MultiAmount
	for _, occurrence := range s.Occurrences {
		for _, posting := range occurrence.Postings {
			if HoldsMoney(posting.Account) {
				total = total.Plus(posting.Amounts)
			}
		}
	}

	return total
}

// Schedules pairs each periodic rule with its projected occurrences, soonest next occurrence first;
// rules with nothing in the next twelve months come last, in journal order.
func (l Ledger) Schedules() []RuleSchedule {
	schedules := make([]RuleSchedule, len(l.Rules))
	for i, rule := range l.Rules {
		schedules[i] = RuleSchedule{Rule: rule, Occurrences: l.occurrencesOf(rule)}
	}
	slices.SortStableFunc(schedules, compareNext)

	return schedules
}

// occurrencesOf is the rule's upcoming occurrences, soonest first.
func (l Ledger) occurrencesOf(rule PeriodicRule) []Transaction {
	var occurrences []Transaction
	for _, occurrence := range l.Upcoming {
		if occurrence.Source == rule.Pos {
			occurrences = append(occurrences, occurrence)
		}
	}
	slices.SortStableFunc(occurrences, func(a, b Transaction) int { return a.Date.Compare(b.Date) })

	return occurrences
}

// AnnualImpact is the net change of net worth over the next twelve months from every rule.
func (l Ledger) AnnualImpact() MultiAmount {
	var total MultiAmount
	for _, schedule := range l.Schedules() {
		total = total.Plus(schedule.Impact())
	}

	return total
}

// Entry is the rule as written in the journal.
func (r PeriodicRule) Entry() string { return strings.Join(r.Lines, "\n") }

// Delete is the changeset that removes the rule from the journal. Any rule can be deleted,
// whether or not the form could have edited it.
func (r PeriodicRule) Delete() Changeset {
	return Changeset{Edits: []Edit{DeleteRule{Pos: r.Pos, Was: r.Entry()}}}
}

// compareNext orders schedules by their next occurrence; those without one sort after those with.
func compareNext(a, b RuleSchedule) int {
	nextA, hasA := a.Next()
	nextB, hasB := b.Next()
	switch {
	case hasA && hasB:
		return nextA.Compare(nextB)
	case hasA:
		return -1
	case hasB:
		return 1
	}

	return 0
}
