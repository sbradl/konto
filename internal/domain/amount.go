package domain

import (
	"errors"
	"fmt"
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

// minPlaces is the fewest decimals an amount is shown with: hledger hands out a quantity with the places it was
// written with, so "-6" and "-6,5" would otherwise stand beside "-6,50".
const minPlaces = 2

func (a Amount) digits() string {
	shown := a
	if a.Style.DecimalMark != "" { // without a mark the decimals would run into the whole number: 100 as 10000
		shown = a.plus(Amount{Places: minPlaces})
	}
	sign, magnitude := splitSign(shown.Mantissa)
	whole, fraction := splitDecimals(magnitude, shown.Places)

	number := a.group(whole)
	if fraction != "" {
		number += a.Style.DecimalMark + fraction
	}

	return sign + number
}

// splitSign is the mantissa's sign, "-" or nothing, and its magnitude.
func splitSign(mantissa int64) (sign string, magnitude int64) {
	if mantissa < 0 {
		return "-", -mantissa
	}

	return "", mantissa
}

// splitDecimals spells the magnitude out as the digits before and after the decimal mark, at least one before.
func splitDecimals(magnitude int64, places int) (whole, fraction string) {
	digits := fmt.Sprintf("%0*d", places+1, magnitude)

	return digits[:len(digits)-places], digits[len(digits)-places:]
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

	whole, fraction, _ := strings.Cut(number, decimalMark)
	if !groupedInThrees(whole, groupMark) {
		return 0, errors.New("not a number")
	}
	whole = strings.ReplaceAll(whole, groupMark, "")

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

// groupedInThrees reports whether every group mark in whole stands between groups of three digits, the first
// group one to three: 9.99, typed with a dot for the decimals, is no 999.
func groupedInThrees(whole, groupMark string) bool {
	if groupMark == "" || !strings.Contains(whole, groupMark) {
		return true
	}

	groups := strings.Split(whole, groupMark)
	if first := len(groups[0]); first < 1 || first > groupSize {
		return false
	}

	return !slices.ContainsFunc(groups[1:], func(group string) bool { return len(group) != groupSize })
}

const groupSize = 3

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
		return DefaultAmount("EUR").Format()
	}

	text := m[0].Format()
	if extra := m.ExtraCount(); extra > 0 {
		text += " + " + strconv.Itoa(extra) + " cur"
	}

	return text
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

// Value is the quantities and commodities without their display style or decimals that say nothing, so equal
// amounts compare equal however the journal or the export writes them: "1234.56EUR".
func (m MultiAmount) Value() string {
	values := make([]string, len(m))
	for i, amount := range m {
		amount = amount.withoutTrailingZeros()
		amount.Style = AmountStyle{DecimalMark: "."}
		values[i] = amount.Format()
	}

	return strings.Join(values, " ")
}

// withoutTrailingZeros is the same quantity with the decimals that are zero dropped: 12.50 as 12.5.
func (a Amount) withoutTrailingZeros() Amount {
	for a.Places != 0 && a.Mantissa%decimalBase == 0 {
		a.Mantissa, a.Places = a.Mantissa/decimalBase, a.Places-1
	}

	return a
}

// Zeroed is the same commodities and styles with nothing in them.
func (m MultiAmount) Zeroed() MultiAmount {
	zero := make(MultiAmount, len(m))
	for i, amount := range m {
		zero[i] = amount.Zero()
	}

	return zero
}
