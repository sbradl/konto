// Package periodic is the use case behind the Scheduled view's form: it turns what the user typed into the
// `~` block to write, and a rule of the journal back into what the form shows. Dates come from hledger, never
// from konto (spec §4.8, ADR 0006).
package periodic

// Frequency is how often a rule recurs, as the form offers it.
type Frequency int

// The frequencies of the form's dropdown, in its order.
const (
	Once Frequency = iota
	Weekly
	Biweekly
	Monthly
	Quarterly
	HalfYearly
	Yearly
)

// Frequencies are the choices of the form's dropdown, in order.
func Frequencies() []Frequency {
	return []Frequency{Once, Weekly, Biweekly, Monthly, Quarterly, HalfYearly, Yearly}
}

// String is the name the dropdown shows.
func (f Frequency) String() string {
	return [...]string{"once", "weekly", "biweekly", "monthly", "quarterly", "half-yearly", "yearly"}[f]
}

// expression is the period expression hledger reads for the frequency; a once rule has a bare date instead.
func (f Frequency) expression() string {
	return [...]string{"", "weekly", "biweekly", "monthly", "quarterly", "every 6 months", "yearly"}[f]
}

// frequencyOf is the frequency whose period expression this is.
func frequencyOf(expression string) (Frequency, bool) {
	for _, frequency := range Frequencies()[1:] {
		if frequency.expression() == expression {
			return frequency, true
		}
	}

	return Once, false
}
