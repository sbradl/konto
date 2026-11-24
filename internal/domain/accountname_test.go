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

func TestCategoryNameGoesUnderIncomeOrExpensesUnlessItHasAnAccountType(t *testing.T) {
	for _, c := range []struct {
		typed, topLevel, want string
	}{
		{" tea ", ExpensesPrefix, "expenses:tea"},
		{"bonus", IncomePrefix, "income:bonus"},
		{"insurance:life", ExpensesPrefix, "expenses:insurance:life"},
		{"income:x", ExpensesPrefix, "income:x"},
		{"assets:cash", IncomePrefix, "assets:cash"},
		{"liabilities:card", ExpensesPrefix, "liabilities:card"},
		{"  ", ExpensesPrefix, ""},
	} {
		if got := CategoryName(c.typed, c.topLevel); got != c.want {
			t.Errorf("CategoryName(%q, %q) = %q, want %q", c.typed, c.topLevel, got, c.want)
		}
	}
}

func TestCheckLeafRefusesALastPartAnotherAccountEndsIn(t *testing.T) {
	names := []string{"assets:checking", "expenses:food:tea"}
	if err := CheckLeaf("expenses:coffee", names); err != nil {
		t.Errorf("a free leaf: %v", err)
	}

	err := CheckLeaf("income:tea", names)
	if err == nil || err.Error() != "the name tea is already used by expenses:food:tea" {
		t.Errorf("a taken leaf: %v", err)
	}
}
