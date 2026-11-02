package domain

import (
	"errors"
	"slices"
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

// Zero is the same commodity, places and style with nothing in it.
func (a Amount) Zero() Amount {
	return Amount{Commodity: a.Commodity, Places: a.Places, Style: a.Style}
}

// IsNegative reports whether the quantity is below zero.
func (a Amount) IsNegative() bool { return a.Mantissa < 0 }

// Format renders the amount the way hledger would, e.g. "-13.240,00 EUR".
func (a Amount) Format() string {
	number := a.digits()
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

func (a Amount) digits() string {
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

// ParseNumber reads a decimal number as the given separators write it, with at most places decimals, into
// the mantissa of that many places: ParseNumber("1.234,5", ",", ".", 2) is 123450. An empty text, a sign
// anywhere but the front and anything but digits are errors.
func ParseNumber(text, decimalMark, groupMark string, places int) (int64, error) {
	number, negative := strings.CutPrefix(strings.TrimSpace(text), "-")
	if groupMark != "" {
		number = strings.ReplaceAll(number, groupMark, "")
	}
	whole, fraction, _ := strings.Cut(number, decimalMark)
	if len(fraction) > places {
		return 0, errors.New("too many decimals")
	}
	mantissa, err := strconv.ParseInt(whole+fraction+strings.Repeat("0", places-len(fraction)), 10, 64)
	if err != nil || whole == "" || strings.ContainsAny(whole+fraction, "+-") {
		return 0, errors.New("not a number")
	}
	if negative {
		mantissa = -mantissa
	}
	return mantissa, nil
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

// FormatNumber renders the quantity alone, without the commodity symbol: "13.240,00".
func (a Amount) FormatNumber() string { return a.digits() }

// WithPlaces is the same value shown with at least the given decimals; finer amounts keep theirs.
func (m MultiAmount) WithPlaces(places int) MultiAmount {
	shown := slices.Clone(m)
	for i := range shown {
		shown[i] = shown[i].plus(Amount{Places: places})
	}
	return shown
}

// Negated is the amount with its sign flipped.
func (a Amount) Negated() Amount {
	a.Mantissa = -a.Mantissa
	return a
}

// plus adds an amount of the same commodity, keeping the finer precision of the two.
func (a Amount) plus(b Amount) Amount {
	for a.Places < b.Places {
		a.Mantissa, a.Places = a.Mantissa*decimalBase, a.Places+1
	}
	for b.Places < a.Places {
		b.Mantissa, b.Places = b.Mantissa*decimalBase, b.Places+1
	}
	a.Mantissa += b.Mantissa
	return a
}

const decimalBase = 10

// Plus is the commodity-wise sum of two multi-amounts, in the order commodities first appear.
func (m MultiAmount) Plus(other MultiAmount) MultiAmount {
	sum := slices.Clone(m)
	for _, amount := range other {
		i := slices.IndexFunc(sum, func(have Amount) bool { return have.Commodity == amount.Commodity })
		if i < 0 {
			sum = append(sum, amount)
			continue
		}
		sum[i] = sum[i].plus(amount)
	}
	return sum
}

// Negated flips the sign of every amount.
func (m MultiAmount) Negated() MultiAmount {
	negated := make(MultiAmount, len(m))
	for i, amount := range m {
		negated[i] = amount.Negated()
	}
	return negated
}

// IsZero reports whether every commodity balance is zero.
func (m MultiAmount) IsZero() bool {
	for _, amount := range m {
		if amount.Mantissa != 0 {
			return false
		}
	}
	return true
}

// Value is the quantities and commodities without their display style or trailing zero decimals, so equal
// amounts compare equal however the journal or the export writes them: "1234.56EUR".
func (m MultiAmount) Value() string {
	values := make([]string, len(m))
	for i, amount := range m {
		for amount.Places > 0 && amount.Mantissa%decimalBase == 0 {
			amount.Mantissa, amount.Places = amount.Mantissa/decimalBase, amount.Places-1
		}
		amount.Style = AmountStyle{DecimalMark: "."}
		values[i] = amount.Format()
	}
	return strings.Join(values, " ")
}

// Zeroed is the same commodities and styles with nothing in them.
func (m MultiAmount) Zeroed() MultiAmount {
	zero := make(MultiAmount, len(m))
	for i, amount := range m {
		zero[i] = amount.Zero()
	}
	return zero
}
