package domain

import (
	"reflect"
	"testing"
)

var euro = AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

func eur(mantissa int64) Amount {
	return Amount{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}
}

func TestAmountFormat(t *testing.T) {
	dollar := AmountStyle{SymbolLeft: true, DecimalMark: ".", GroupMark: ",", GroupSize: 3}
	for name, tc := range map[string]struct {
		amount Amount
		want   string
	}{
		"grouped":          {eur(1324000), "13.240,00 EUR"},
		"negative":         {eur(-14000), "-140,00 EUR"},
		"zero":             {eur(0), "0,00 EUR"},
		"cents only":       {eur(5), "0,05 EUR"},
		"one group":        {eur(99999), "999,99 EUR"},
		"exact group":      {eur(100000), "1.000,00 EUR"},
		"two groups":       {eur(123456789), "1.234.567,89 EUR"},
		"symbol left":      {Amount{Commodity: "$", Mantissa: 12345, Places: 2, Style: dollar}, "$123.45"},
		"symbol left gap":  {Amount{Commodity: "USD", Mantissa: 100, Places: 0, Style: AmountStyle{SymbolLeft: true, Spaced: true}}, "USD 100"},
		"no space":         {Amount{Commodity: "EUR", Mantissa: 100, Places: 2, Style: AmountStyle{DecimalMark: ","}}, "1,00EUR"},
		"no commodity":     {Amount{Mantissa: 150, Places: 1, Style: AmountStyle{DecimalMark: "."}}, "15.0"},
		"ungrouped":        {Amount{Commodity: "EUR", Mantissa: 1234567, Places: 2, Style: AmountStyle{Spaced: true, DecimalMark: ","}}, "12345,67 EUR"},
		"no decimal place": {Amount{Commodity: "EUR", Mantissa: -2500, Places: 0, Style: euro}, "-2.500 EUR"},
	} {
		if got := tc.amount.Format(); got != tc.want {
			t.Errorf("%s: Format() = %q, want %q", name, got, tc.want)
		}
	}
}

func TestMultiAmountFormat(t *testing.T) {
	pln := Amount{Commodity: "PLN", Mantissa: 10000, Places: 2, Style: euro}

	for name, tc := range map[string]struct {
		amounts MultiAmount
		want    string
	}{
		"empty is a bare zero": {nil, "0"},
		"one commodity":        {MultiAmount{eur(100)}, "1,00 EUR"},
		"two commodities":      {MultiAmount{eur(100), pln}, "1,00 EUR + 1 cur"},
		"three commodities":    {MultiAmount{eur(100), pln, pln}, "1,00 EUR + 2 cur"},
	} {
		if got := tc.amounts.Format(); got != tc.want {
			t.Errorf("%s: Format() = %q, want %q", name, got, tc.want)
		}
	}
}

func TestMultiAmountExtraCount(t *testing.T) {
	if got := (MultiAmount{}).ExtraCount(); got != 0 {
		t.Errorf("empty ExtraCount = %d, want 0", got)
	}
	if got := (MultiAmount{eur(1), eur(2)}).ExtraCount(); got != 1 {
		t.Errorf("two-commodity ExtraCount = %d, want 1", got)
	}
}

func TestMultiAmountIsAllNegative(t *testing.T) {
	for name, tc := range map[string]struct {
		amounts MultiAmount
		want    bool
	}{
		"empty":    {nil, false},
		"negative": {MultiAmount{eur(-1)}, true},
		"zero":     {MultiAmount{eur(0)}, false},
		"mixed":    {MultiAmount{eur(-1), eur(1)}, false},
		"all":      {MultiAmount{eur(-1), eur(-2)}, true},
	} {
		if got := tc.amounts.IsAllNegative(); got != tc.want {
			t.Errorf("%s: IsAllNegative() = %v, want %v", name, got, tc.want)
		}
	}
}

func TestAmountFormatPadsAFractionOnlyQuantity(t *testing.T) {
	if got := eur(50).Format(); got != "0,50 EUR" {
		t.Errorf("Format() = %q, want 0,50 EUR", got)
	}
}

func TestAmountFormatNumberOmitsTheCommodity(t *testing.T) {
	if got := eur(-1324000).FormatNumber(); got != "-13.240,00" {
		t.Errorf("FormatNumber() = %q, want -13.240,00", got)
	}
}

func TestAmountNegatedFlipsTheSign(t *testing.T) {
	if got := eur(5).Negated(); got.Mantissa != -5 || got.Commodity != "EUR" {
		t.Errorf("Negated() = %+v", got)
	}
}

func TestMultiAmountPlusAddsCommodityWise(t *testing.T) {
	pln := Amount{Commodity: "PLN", Mantissa: 100, Places: 2, Style: euro}

	got := MultiAmount{eur(100)}.Plus(MultiAmount{pln, eur(250)})

	if want := (MultiAmount{eur(350), pln}); !reflect.DeepEqual(got, want) {
		t.Errorf("Plus = %+v, want %+v", got, want)
	}
	if got := MultiAmount(nil).Plus(MultiAmount{eur(1)}); !reflect.DeepEqual(got, MultiAmount{eur(1)}) {
		t.Errorf("nil.Plus = %+v", got)
	}
}

func TestMultiAmountPlusAlignsThePrecision(t *testing.T) {
	coarse := Amount{Commodity: "EUR", Mantissa: 5, Places: 0, Style: euro} // 5
	fine := Amount{Commodity: "EUR", Mantissa: 125, Places: 2, Style: euro} // 1,25

	for name, got := range map[string]MultiAmount{
		"coarse first": MultiAmount{coarse}.Plus(MultiAmount{fine}),
		"fine first":   MultiAmount{fine}.Plus(MultiAmount{coarse}),
	} {
		if len(got) != 1 || got[0].Mantissa != 625 || got[0].Places != 2 {
			t.Errorf("%s: sum = %+v, want 6,25", name, got)
		}
	}
}

func TestMultiAmountNegatedFlipsEveryAmount(t *testing.T) {
	got := MultiAmount{eur(1), eur(-2)}.Negated()

	if got[0].Mantissa != -1 || got[1].Mantissa != 2 {
		t.Errorf("Negated = %+v", got)
	}
}
