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
