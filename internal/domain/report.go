package domain

import "time"

// ReportPeriod is the stretch of time a report covers; every period ends today.
type ReportPeriod int

// The periods the Reports view cycles through (spec §6.4).
const (
	MonthToDate ReportPeriod = iota
	QuarterToDate
	YearToDate
	AllTime // the loaded window, this year and last (ADR 0013), not every year the journal has
)

const monthsPerQuarter = 3

// String is the period as the Reports view names it.
func (p ReportPeriod) String() string {
	switch p {
	case MonthToDate:
		return "month to date"
	case QuarterToDate:
		return "quarter to date"
	case YearToDate:
		return "year to date"
	case AllTime:
	}

	return "all time" // of the loaded window (ADR 0013)
}

// Next is the period after p, wrapping from all time back to the month.
func (p ReportPeriod) Next() ReportPeriod {
	if p == AllTime {
		return MonthToDate
	}

	return p + 1
}

// Start is the first day of the period ending today; false for all time, which has none.
func (p ReportPeriod) Start(today time.Time) (time.Time, bool) {
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
	Period          ReportPeriod
	BalanceSheet    []ReportRow
	IncomeStatement []ReportRow
}
