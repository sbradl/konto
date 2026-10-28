package domain

import "time"

// Projection is one account's balance at the end of each month of a span, the months before today as they
// were and those after as the periodic rules make them (spec §4.10). Ends[i] is the day period i ends, which
// is exclusive: Balances[i] counts everything dated before it.
type Projection struct {
	Ends     []time.Time
	Balances []MultiAmount // one per period; empty before the account has any posting
}

// ProjectionQuery asks for an account's projection: months from Begin on, the periodic rules' occurrences
// counted from ForecastFrom up to, not including, ForecastTo.
type ProjectionQuery struct {
	Account      string
	Begin        time.Time
	ForecastFrom time.Time
	ForecastTo   time.Time
}
