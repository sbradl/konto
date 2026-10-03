// Package dashboard is the Dashboard view: the Accounts panel with its cursor.
package dashboard

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	minAccountsWidth = 50 // the Accounts column is a third of the screen, but never narrower than this
	columnDivisor    = 3
	panelChrome      = 2 // border rows or columns around the panel body
)

// Model is the Dashboard's view-local state; the ledger is replaced wholesale on refresh.
type Model struct {
	accounts []domain.Account
	cursor   int
	offset   int // the first visible row
	width    int
	height   int
}

// New returns an empty Dashboard.
func New() Model { return Model{} }

// WithAccounts shows the asset and liability accounts of a fresh snapshot: open
// ones first, closed ones after, each group by name. The cursor stays on its row
// number, or on the last row if the list shrank.
func (m Model) WithAccounts(accounts []domain.Account) Model {
	listed := make([]domain.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.IsClassifiable() {
			listed = append(listed, account)
		}
	}
	slices.SortStableFunc(listed, compareAccounts)
	next := m
	next.accounts = listed
	return next.moveTo(next.cursor)
}

// compareAccounts orders open accounts before closed ones, each group by name.
func compareAccounts(a, b domain.Account) int {
	switch {
	case a.IsClosed() && !b.IsClosed():
		return 1
	case !a.IsClosed() && b.IsClosed():
		return -1
	}
	return strings.Compare(a.Name, b.Name)
}

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	next := m
	next.width, next.height = width, height
	return next.moveTo(next.cursor)
}

// Move applies a cursor key: up, down, home, end, pgup or pgdown. The cursor stops at both ends.
func (m Model) Move(key string) Model {
	switch key {
	case "up":
		return m.moveTo(m.cursor - 1)
	case "down":
		return m.moveTo(m.cursor + 1)
	case "home":
		return m.moveTo(0)
	case "end":
		return m.moveTo(len(m.accounts)) // moveTo clamps to the last row
	case "pgup":
		return m.moveTo(m.cursor - m.pageSize())
	case "pgdown":
		return m.moveTo(m.cursor + m.pageSize())
	}
	return m
}

// moveTo puts the cursor on row i, clamped, and scrolls to keep it visible.
func (m Model) moveTo(i int) Model {
	next := m
	next.cursor = max(0, min(i, len(m.accounts)-1))
	next.offset = max(0, min(m.offset, next.cursor))
	if next.cursor >= next.offset+m.pageSize() {
		next.offset = next.cursor - m.pageSize() + 1
	}
	return next
}

// pageSize is how many account rows the panel shows at once.
func (m Model) pageSize() int { return max(1, m.height-panelChrome) }

// Position is the 1-based cursor row and the row count; (0, 0) when there are no accounts.
func (m Model) Position() (row, total int) {
	if len(m.accounts) == 0 {
		return 0, 0
	}
	return m.cursor + 1, len(m.accounts)
}

// View draws the Accounts panel in the left column.
func (m Model) View() string {
	width := min(m.width, max(minAccountsWidth, m.width/columnDivisor))
	inner := max(0, width-panelChrome)
	last := min(len(m.accounts), m.offset+m.pageSize())
	visible := m.accounts[m.offset:last]
	rows := make([]string, len(visible))
	for i, account := range visible {
		rows[i] = accountLine(account, inner, m.offset+i == m.cursor)
	}
	return layout.Panel("ACCOUNTS", width, m.height, strings.Join(rows, "\n"))
}

// accountLine is one account: source glyph, name, right-aligned balance.
func accountLine(account domain.Account, width int, selected bool) string {
	glyph, glyphStyle := sourceMark(account.Source)
	balance, balanceStyle := account.Balance.Format(), lipgloss.NewStyle()
	if account.IsClosed() {
		balance = "closed " + account.Closed.Format(time.DateOnly)
	} else if account.Balance.IsAllNegative() {
		balanceStyle = theme.Negative
	}
	left := " " + glyph + " " + account.Name
	gap := strings.Repeat(" ", max(1, width-lipgloss.Width(left)-lipgloss.Width(balance)-1))
	switch {
	case selected: // the glyph stays outside the highlight so its dim colour remains readable
		rest := " " + account.Name + gap + balance + " "
		return " " + glyphStyle.Render(glyph) + theme.Selected.Render(padTo(rest, width-lipgloss.Width(" "+glyph)))
	case account.IsClosed():
		return theme.Dim.Render(left + gap + balance)
	}
	return " " + glyphStyle.Render(glyph) + " " + account.Name + gap + balanceStyle.Render(balance)
}

func padTo(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-lipgloss.Width(text)))
}

// sourceMark is the glyph and colour of how an account is fed (spec §6.4a).
func sourceMark(source domain.Source) (string, lipgloss.Style) {
	switch source {
	case domain.SourceImport:
		return "↓", theme.Dim
	case domain.SourceManual:
		return "✎", theme.Warn
	case domain.SourceClearing:
		return "⇄", theme.Dim
	case domain.SourceNone:
	}
	return "?", theme.Warn
}
