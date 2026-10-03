// Package dashboard is the Dashboard view: the Accounts panel with its cursor.
package dashboard

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	minAccountsWidth    = 50 // the Accounts column is a third of the screen, but never narrower than this
	columnDivisor       = 3
	panelChrome         = 2 // border rows or columns around the panel body
	cardHeight          = 4 // a stat card: border, figure, note, border
	cardCount           = 2
	minCardWidth        = 24 // narrower than this the cards give way
	maxCategoryWidth    = 24 // the Recent panel's widest category column
	minDescriptionWidth = 8
	descriptionShare    = 45 // percent of the room beside the date and amount that description takes
	recentGap           = 2
	recentFixed         = 16 // a Recent row's edges, its date and the gaps around the middle columns
	upcomingHeight      = 7  // the Upcoming panel: border, four occurrences and a "more" row, border
	minRecentHeight     = 5  // Upcoming gives way rather than squeeze Recent below this
)

// Model is the Dashboard's view-local state; the ledger is replaced wholesale on refresh.
type Model struct {
	accounts []domain.Account
	recent   []domain.RecentEntry
	upcoming []domain.RecentEntry
	figures  figures
	cursor   int
	offset   int // the first visible row
	width    int
	height   int
}

// figures are the numbers the stat cards show.
type figures struct {
	netWorth domain.MultiAmount
	month    time.Month
	net      domain.MultiAmount
	income   domain.MultiAmount
	expenses domain.MultiAmount
}

// WithLedger shows a fresh snapshot: its accounts and the stat cards' figures.
func (m Model) WithLedger(ledger domain.Ledger) Model {
	next := m
	next.figures = figures{
		netWorth: ledger.NetWorth(), month: ledger.Month.Month,
		net: ledger.MonthNet(), income: ledger.MonthIncome(), expenses: ledger.MonthExpenses(),
	}
	next.recent = ledger.Recent()
	next.upcoming = ledger.UpcomingEntries()
	return next.WithAccounts(ledger.Accounts)
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

// Selected is the account under the cursor; false when the panel is empty.
func (m Model) Selected() (domain.Account, bool) {
	if len(m.accounts) == 0 {
		return domain.Account{}, false
	}
	return m.accounts[m.cursor], true
}

// View draws the Accounts panel in the left column and the stat cards above whatever fills the right.
func (m Model) View() string {
	left := m.accountsPanel()
	width := lipgloss.Width(left)
	if m.width-width < 2*minCardWidth {
		return left
	}
	right := m.width - width
	cards := lipgloss.JoinHorizontal(lipgloss.Top, m.cards(right)...)
	column := cards + "\n" + m.listPanels(right, m.height-cardHeight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, column)
}

// listPanels stacks Recent over Upcoming; Upcoming is left out when there is no room for both.
func (m Model) listPanels(width, height int) string {
	if height < upcomingHeight+minRecentHeight {
		return entriesPanel("RECENT", m.recent, width, height)
	}
	recent := entriesPanel("RECENT", m.recent, width, height-upcomingHeight)
	return recent + "\n" + entriesPanel("UPCOMING", m.upcoming, width, upcomingHeight)
}

// entriesPanel lists transactions one per row: date, description, category, amount. When they do not
// all fit, the last row says how many more there are. Rows are not selectable.
func entriesPanel(title string, entries []domain.RecentEntry, width, height int) string {
	inner, capacity := max(0, width-panelChrome), max(0, height-panelChrome)
	shown := entries
	if len(shown) > capacity {
		shown = shown[:max(0, capacity-1)]
	}
	columns := recentColumns(shown, inner)
	rows := make([]string, 0, capacity)
	for _, entry := range shown {
		rows = append(rows, columns.line(entry))
	}
	if hidden := len(entries) - len(shown); hidden > 0 {
		rows = append(rows, theme.Dim.Render(fmt.Sprintf(" … %d more", hidden)))
	}
	return layout.Panel(title, width, height, strings.Join(rows, "\n"))
}

// recent column widths for one panel width: what is left of the row after the date and the widest amount
// is shared between description and category.
type columns struct {
	width, amount, description, category int
}

func recentColumns(entries []domain.RecentEntry, width int) columns {
	widest := 0
	for _, entry := range entries {
		widest = max(widest, lipgloss.Width(entry.Amount.Format()))
	}
	spare := max(0, width-recentFixed-widest)
	description := max(minDescriptionWidth, spare*descriptionShare/100)
	return columns{
		width: width, amount: widest, description: min(description, spare),
		category: min(maxCategoryWidth, max(0, spare-description-recentGap)),
	}
}

// line is one transaction: the amount at the right edge, red when negative and green when not.
func (c columns) line(entry domain.RecentEntry) string {
	amount := entry.Amount.Format()
	style := theme.Ok
	if entry.Amount.IsAllNegative() {
		style = theme.Negative
	}
	left := " " + entry.Date.Format(time.DateOnly) + "  " + fit(entry.Description, c.description)
	left += "  " + fit(entry.Category, c.category)
	gap := strings.Repeat(" ", max(1, c.width-lipgloss.Width(left)-lipgloss.Width(amount)-1))
	return left + gap + style.Render(amount)
}

// fit pads text to width, or cuts it off there.
func fit(text string, width int) string {
	cut := lipgloss.NewStyle().MaxWidth(width).Render(text)
	return cut + strings.Repeat(" ", width-lipgloss.Width(cut))
}

// cards are the two stat cards sharing width between them.
func (m Model) cards(width int) []string {
	half := width / cardCount
	net := m.figures.net
	style := theme.Ok
	if len(net) > 0 && net[0].IsNegative() {
		style = theme.Negative
	}
	worthValue := theme.Bold.Render(m.figures.netWorth.Format())
	worthNote := theme.Dim.Render("assets + liabilities")
	incomeNote := number(m.figures.income) + " in / " + number(m.figures.expenses) + " out"
	incomeTitle := strings.ToUpper(m.figures.month.String()) + " NET INCOME"
	return []string{
		card("NET WORTH", half, worthValue, worthNote),
		card(incomeTitle, width-half, style.Render(net.Format()), theme.Dim.Render(incomeNote)),
	}
}

// card is a bordered figure with a note under it.
func card(title string, width int, value, note string) string {
	return layout.Panel(title, width, cardHeight, " "+value+"\n "+note)
}

// number is the amount without its commodity when there is just one, else as Format shows it.
func number(amounts domain.MultiAmount) string {
	if len(amounts) == 1 {
		return amounts[0].FormatNumber()
	}
	return amounts.Format()
}

// accountsPanel is the Accounts panel, a third of the screen wide.
func (m Model) accountsPanel() string {
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
