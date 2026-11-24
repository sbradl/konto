package domain

import "strings"

// The top levels of an account name, colon included: assets and liabilities are the places money is in,
// income and expenses are categories, what money came from or went on (CONTEXT: Account, Category).
const (
	AssetsPrefix      = "assets:"
	LiabilitiesPrefix = "liabilities:"
	IncomePrefix      = "income:"
	ExpensesPrefix    = "expenses:"
)

// HoldsMoney reports an asset or liability: an account in the UI's sense, a real place money is in.
func HoldsMoney(name string) bool {
	return strings.HasPrefix(name, AssetsPrefix) || strings.HasPrefix(name, LiabilitiesPrefix)
}

// IsCategory reports an income or expense account.
func IsCategory(name string) bool { return IsIncome(name) || IsExpense(name) }

// IsIncome reports an account under income.
func IsIncome(name string) bool { return strings.HasPrefix(name, IncomePrefix) }

// IsExpense reports an account under expenses.
func IsExpense(name string) bool { return strings.HasPrefix(name, ExpensesPrefix) }

// CategoryName is the account a typed new category is declared as: kept as typed when it starts with an
// account type, else under the top level, IncomePrefix when money comes from it and ExpensesPrefix when it
// goes to it; insurance:x is a subcategory, not a top level of its own.
func CategoryName(typed, topLevel string) string {
	typed = strings.TrimSpace(typed)
	if IsCategory(typed) || HoldsMoney(typed) || typed == "" {
		return typed
	}

	return topLevel + typed
}

// CheckLeaf refuses a new name whose last part one of the names already ends in (hledger's uniqueleafnames).
func CheckLeaf(name string, names []string) error {
	for _, existing := range names {
		if leaf(existing) == leaf(name) {
			return RefusalError{Reason: "the name " + leaf(name) + " is already used by " + existing}
		}
	}

	return nil
}

// leaf is the last part of an account name.
func leaf(name string) string { return name[strings.LastIndex(name, ":")+1:] }
