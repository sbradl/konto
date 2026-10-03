package domain

import (
	"strconv"
	"strings"
)

// AmountStyle is how hledger displays a commodity (its `astyle`).
type AmountStyle struct {
	SymbolLeft  bool   // the commodity symbol precedes the number
	Spaced      bool   // a space separates symbol and number
	DecimalMark string // "," or "."
	GroupMark   string // digit-group separator; empty when ungrouped
	GroupSize   int    // digits per group; 0 when ungrouped
}

// Amount is an exact quantity of one commodity: Mantissa × 10^-Places.
type Amount struct {
	Commodity string
	Mantissa  int64
	Places    int
	Style     AmountStyle
}

// IsNegative reports whether the quantity is below zero.
func (a Amount) IsNegative() bool { return a.Mantissa < 0 }

// Format renders the amount the way hledger would, e.g. "-13.240,00 EUR".
func (a Amount) Format() string {
	number := a.formatNumber()
	if a.Commodity == "" {
		return number
	}
	gap := ""
	if a.Style.Spaced {
		gap = " "
	}
	if a.Style.SymbolLeft {
		return a.Commodity + gap + number
	}
	return number + gap + a.Commodity
}

func (a Amount) formatNumber() string {
	magnitude := a.Mantissa
	sign := ""
	if magnitude < 0 {
		magnitude, sign = -magnitude, "-"
	}
	digits := strconv.FormatInt(magnitude, 10)
	if len(digits) <= a.Places {
		digits = strings.Repeat("0", a.Places-len(digits)+1) + digits
	}
	whole, fraction := digits[:len(digits)-a.Places], digits[len(digits)-a.Places:]
	number := a.group(whole)
	if a.Places > 0 {
		number += a.Style.DecimalMark + fraction
	}
	return sign + number
}

// group inserts the digit-group mark between groups of GroupSize, counted from the right.
func (a Amount) group(whole string) string {
	size := a.Style.GroupSize
	if size <= 0 {
		return whole
	}
	var groups []string
	for len(whole) > size {
		groups = append([]string{whole[len(whole)-size:]}, groups...)
		whole = whole[:len(whole)-size]
	}
	return strings.Join(append([]string{whole}, groups...), a.Style.GroupMark)
}

// MultiAmount holds one Amount per commodity, in hledger's order.
type MultiAmount []Amount

// ExtraCount is how many commodities there are beyond the first.
func (m MultiAmount) ExtraCount() int { return max(0, len(m)-1) }

// IsAllNegative reports whether there is at least one amount and every one is negative.
func (m MultiAmount) IsAllNegative() bool {
	for _, amount := range m {
		if !amount.IsNegative() {
			return false
		}
	}
	return len(m) > 0
}

// Format renders the first commodity, plus " + N cur" when there are more.
func (m MultiAmount) Format() string {
	if len(m) == 0 {
		return "0"
	}
	text := m[0].Format()
	if extra := m.ExtraCount(); extra > 0 {
		text += " + " + strconv.Itoa(extra) + " cur"
	}
	return text
}
