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
	Overdue     []Transaction // past occurrences nothing booked covers, oldest first
	Occurrences []Transaction // soonest first
}

// IsOverdue reports whether a past occurrence of the rule is still not booked.
func (s RuleSchedule) IsOverdue() bool { return len(s.Overdue) > 0 }

// pending is the overdue occurrences, then the upcoming ones.
func (s RuleSchedule) pending() []Transaction { return slices.Concat(s.Overdue, s.Occurrences) }

// Next is the date of the rule's next occurrence; false when none falls in the next twelve months.
func (s RuleSchedule) Next() (time.Time, bool) { return firstDate(s.Occurrences) }

// Due is the date of the rule's oldest overdue occurrence, or else of its next one; false when there is neither.
func (s RuleSchedule) Due() (time.Time, bool) { return firstDate(s.pending()) }

func firstDate(occurrences []Transaction) (time.Time, bool) {
	if len(occurrences) == 0 {
		return time.Time{}, false
	}

	return occurrences[0].Date, true
}

// Impact is the signed change the rule brings to net worth over its overdue and upcoming occurrences: what it
// does to asset and liability accounts. A transfer between own accounts nets to zero.
func (s RuleSchedule) Impact() MultiAmount {
	var total MultiAmount
	for _, occurrence := range s.pending() {
		total = total.Plus(netWorthChange(occurrence))
	}

	return total
}

// Amount is what the oldest overdue or next occurrence does to net worth; zero when there is neither.
func (s RuleSchedule) Amount() MultiAmount {
	pending := s.pending()
	if len(pending) == 0 {
		return nil
	}

	return netWorthChange(pending[0])
}

// netWorthChange is what a transaction does to asset and liability accounts.
func netWorthChange(transaction Transaction) MultiAmount {
	var total MultiAmount
	for _, posting := range transaction.Postings {
		if HoldsMoney(posting.Account) {
			total = total.Plus(posting.Amounts)
		}
	}

	return total
}

// Schedules pairs each periodic rule with its overdue and projected occurrences, soonest due first;
// rules with nothing in the next twelve months come last, in journal order.
func (l Ledger) Schedules() []RuleSchedule {
	overdue := l.Overdue()
	schedules := make([]RuleSchedule, len(l.Rules))
	for i, rule := range l.Rules {
		schedules[i] = RuleSchedule{Rule: rule, Overdue: of(rule, overdue), Occurrences: of(rule, l.Upcoming)}
	}
	slices.SortStableFunc(schedules, compareNext)

	return schedules
}

// of is the rule's occurrences among the given ones, soonest first.
func of(rule PeriodicRule, all []Transaction) []Transaction {
	var occurrences []Transaction
	for _, occurrence := range all {
		if occurrence.Source == rule.Pos {
			occurrences = append(occurrences, occurrence)
		}
	}
	slices.SortStableFunc(occurrences, func(a, b Transaction) int { return a.Date.Compare(b.Date) })

	return occurrences
}

// OverdueDays is how far back a rule's unbooked occurrence still counts as overdue: long enough for a booking
// that arrives a few days late, short enough that a payment that never came drops out.
const OverdueDays = 7

// coverLead is how many days before its due date a booking may already pay an occurrence.
const coverLead = 7

// Overdue is the rules' past occurrences that no booked transaction covers, oldest first. A booking covers one
// occurrence when it posts the same amount to the occurrence's category, at most coverLead days before it is due.
func (l Ledger) Overdue() []Transaction {
	past := slices.SortedStableFunc(slices.Values(l.Past), func(a, b Transaction) int { return a.Date.Compare(b.Date) })
	used := make([]bool, len(l.Transactions))
	var overdue []Transaction
	for _, occurrence := range past {
		if i := l.coverOf(occurrence, used); i >= 0 {
			used[i] = true

			continue
		}
		overdue = append(overdue, occurrence)
	}

	return overdue
}

// coverOf is the index of the first unused booking that covers the occurrence; -1 when there is none.
func (l Ledger) coverOf(occurrence Transaction, used []bool) int {
	category, earliest := occurrence.category(), occurrence.Date.AddDate(0, 0, -coverLead)
	for i, booking := range l.Transactions {
		if !used[i] && !booking.Date.Before(earliest) && booking.posts(category) {
			return i
		}
	}

	return -1
}

// category is the posting that says what an occurrence is for: the first not on a money account, or the first
// posting of a transfer between money accounts.
func (t Transaction) category() Posting {
	for _, posting := range t.Postings {
		if !HoldsMoney(posting.Account) {
			return posting
		}
	}

	if len(t.Postings) == 0 {
		return Posting{}
	}

	return t.Postings[0]
}

// posts reports whether the transaction has the posting: the same account and the same amount.
func (t Transaction) posts(want Posting) bool {
	return slices.ContainsFunc(t.Postings, func(posting Posting) bool {
		return posting.Account == want.Account && posting.Amounts.Value() == want.Amounts.Value()
	})
}

// AnnualImpact is the net change of net worth over the next twelve months from every rule.
func (l Ledger) AnnualImpact() MultiAmount {
	var total MultiAmount
	schedules := l.Schedules()
	for i := range schedules {
		total = total.Plus(schedules[i].Impact())
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

// compareNext orders schedules by their due date; those without one sort after those with.
func compareNext(a, b RuleSchedule) int {
	nextA, hasA := a.Due()
	nextB, hasB := b.Due()
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
