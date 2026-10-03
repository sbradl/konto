package domain

import "time"

// Period is the stretch of time a report covers; every period ends today.
type Period int

// The periods the Reports view cycles through (spec §6.4).
const (
	MonthToDate Period = iota
	QuarterToDate
	YearToDate
	AllTime
)

const monthsPerQuarter = 3

// String is the period as the Reports view names it.
func (p Period) String() string {
	switch p {
	case MonthToDate:
		return "month to date"
	case QuarterToDate:
		return "quarter to date"
	case YearToDate:
		return "year to date"
	case AllTime:
	}
	return "all time"
}

// Next is the period after p, wrapping from all time back to the month.
func (p Period) Next() Period {
	if p == AllTime {
		return MonthToDate
	}
	return p + 1
}

// Start is the first day of the period ending today; false for all time, which has none.
func (p Period) Start(today time.Time) (time.Time, bool) {
	month := today.Month()
	switch p {
	case MonthToDate:
	case QuarterToDate:
		month -= (month - 1) % monthsPerQuarter
	case YearToDate:
		month = time.January
	case AllTime:
		return time.Time{}, false
	}
	return time.Date(today.Year(), month, 1, 0, 0, 0, 0, today.Location()), true
}

// RowKind says how a report row is drawn.
type RowKind int

// The kinds of report rows.
const (
	RowHeader     RowKind = iota // a section title
	RowLeaf                      // one account
	RowSubtotal                  // the total of a section
	RowGrandTotal                // the bottom line of a table
)

// ReportRow is one line of a report table. Delta is the change within the period, nil where the
// table has no such column or the row has none.
type ReportRow struct {
	Kind   RowKind
	Label  string
	Amount MultiAmount
	Delta  *MultiAmount
}

// Report is both tables for one period.
type Report struct {
	Period          Period
	BalanceSheet    []ReportRow
	IncomeStatement []ReportRow
}
