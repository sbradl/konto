package imports

import (
	"fmt"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

// info is the file, the account it goes to, the bank format, the watermark and what is left to do.
func (m Model) info() string {
	staged := m.staged
	lines := []string{
		label("file", staged.File), label("account", accountText(staged)), label("format", staged.Format),
		label("watermark", watermarkText(staged)), label("staged", m.stagedSummary()),
		label("", theme.Bold.Render(needCategoryText(staged))),
	}

	return layout.Panel("IMPORT", m.width, infoHeight, strings.Join(lines, "\n"))
}

// accountText is the account the import goes to, or the new account it would create.
func accountText(staged *csvimport.Staged) string {
	if !staged.NewAccount {
		return staged.Account.Name
	}

	if staged.Named != "" {
		return staged.Named + theme.Dim.Render(" (new)")
	}

	return theme.Warn.Render(createHint) + theme.Dim.Render(" for "+staged.IBAN)
}

// watermarkText is the account's watermark: the date of its latest transaction in the journal.
func watermarkText(staged *csvimport.Staged) string {
	if staged.Watermark.IsZero() {
		return noWatermark
	}

	return staged.Watermark.Format(time.DateOnly)
}

// needCategoryText is how many rows still need a category, or that the export holds nothing new.
func needCategoryText(staged *csvimport.Staged) string {
	if len(staged.Rows) == 0 {
		return nothingNew
	}

	return fmt.Sprintf("%d need a category", staged.NeedCategory())
}

// label is a line of a panel: a dim name, then its value.
func label(name, value string) string {
	return " " + theme.Dim.Render(fmt.Sprintf("%-*s", labelWidth, name)) + value
}

// stagedSummary lists what the import would add to the journal besides the transactions: the rules and the
// new accounts, and what the categories do to net income.
func (m Model) stagedSummary() string {
	rules := m.staged.StagedRules()

	items := make([]string, 0, len(rules))
	if m.staged.Named != "" {
		items = append(items, "account "+m.staged.Named)
	}
	for _, rule := range rules {
		items = append(items, "rule "+rule.ID)
	}
	for _, name := range m.staged.NewAccounts {
		items = append(items, "account "+name)
	}

	list := noItems
	if len(items) > 0 {
		list = strings.Join(items, ", ")
	}

	return list + theme.Dim.Render(" · net income ") + m.staged.NetIncomeChange().Format()
}
