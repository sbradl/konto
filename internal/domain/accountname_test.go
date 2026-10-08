package domain

import "testing"

func TestAccountNamesAreSortedByTheirTopLevel(t *testing.T) {
	for name, want := range map[string][4]bool{ // money, category, income, expense
		"assets:checking":        {true, false, false, false},
		"liabilities:creditcard": {true, false, false, false},
		"income:salary":          {false, true, true, false},
		"expenses:groceries":     {false, true, false, true},
		"equity:opening":         {false, false, false, false},
		"assets":                 {false, false, false, false}, // a top level alone is not postable
		"assetsx:checking":       {false, false, false, false},
	} {
		got := [4]bool{HoldsMoney(name), IsCategory(name), IsIncome(name), IsExpense(name)}
		if got != want {
			t.Errorf("%s: money, category, income, expense = %v, want %v", name, got, want)
		}
	}
}
