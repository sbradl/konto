package domain

import (
	"slices"
	"testing"
	"time"
)

func TestCarryClosesAndOpensTheBalancesOnTheGivenDays(t *testing.T) {
	carry := SetCarry{Year: 2027, Balances: []Account{
		{Name: "assets:checking", Balance: euros(150000)},
		{Name: "liabilities:card", Balance: euros(-2000)},
	}}
	closing := carry.Closing(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	opening := carry.Opening(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	wantClosing := []string{
		"2026-12-31 closing balance  ; konto:carry",
		"    assets:checking  -1.500,00 EUR",
		"    liabilities:card  20,00 EUR",
		"    equity:opening-balances",
	}
	wantOpening := []string{
		"2027-01-01 opening balance  ; konto:carry",
		"    assets:checking  1.500,00 EUR",
		"    liabilities:card  -20,00 EUR",
		"    equity:opening-balances",
	}
	if got := closing.Lines(); !slices.Equal(got, wantClosing) {
		t.Errorf("closing =\n%q\nwant\n%q", got, wantClosing)
	}
	if got := opening.Lines(); !slices.Equal(got, wantOpening) {
		t.Errorf("opening =\n%q\nwant\n%q", got, wantOpening)
	}
}

func TestCarryPostsEachCommodityOfAnAccountOnItsOwn(t *testing.T) {
	both := append(euros(100), Amount{Commodity: "USD", Mantissa: 200, Places: 2, Style: AmountStyle{Spaced: true, DecimalMark: ","}})
	carry := SetCarry{Year: 2027, Balances: []Account{{Name: "assets:wallet", Balance: both}}}

	lines := carry.Opening(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)).Lines()

	if len(lines) != 4 || lines[1] != "    assets:wallet  1,00 EUR" || lines[2] != "    assets:wallet  2,00 USD" {
		t.Errorf("lines = %q", lines)
	}
}
