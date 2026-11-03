// Package dashboard is the Dashboard view: the Accounts panel with its cursor, and beside it the stat cards,
// Recent, Upcoming and the monthly spend chart as far as the terminal has room for them.
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
	minAccountsWidth    = 50 // the Accounts column is a third of the screen, but never narrower than this
	columnDivisor       = 3
	cardHeight          = 4 // a stat card: border, figure, note, border
	cardCount           = 2
	minCardWidth        = 24 // narrower than this the cards give way
	maxCategoryWidth    = 24 // the Recent panel's widest category column
	minDescriptionWidth = 8
	descriptionShare    = 60 // percent of the room beside the date and amount that description takes
	recentGap           = 2
	recentFixed         = 16 // a Recent row's edges, its date and the gaps around the middle columns
	upcomingHeight      = 10 // the Upcoming panel and the spend chart beside it
	bandPanels          = 2  // Upcoming and the spend chart share the lower band
	minBandWidth        = 40 // narrower than this each, the chart gives way
	minRecentHeight     = 5  // Upcoming gives way rather than squeeze Recent below this
)

// Model is the Dashboard's view-local state; the ledger is replaced wholesale on refresh.
type Model struct {
	accounts []domain.Account
	recent   []domain.RecentEntry
	upcoming []domain.RecentEntry
	spend    []domain.MonthSpend
	figures  figures
	cursor   layout.Cursor
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
	next.spend = ledger.Spend
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
	return next.moveTo(next.cursor.Row)
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
	return next.moveTo(next.cursor.Row)
}

// Action is what a key asks the root model to do with the selected account: CloseSelected or ReopenSelected;
// nil asks for nothing.
type Action interface{ isAction() }

// CloseSelected asks for closing the selected account to be planned.
type CloseSelected struct{}

// ReopenSelected asks for reopening the selected account to be planned.
type ReopenSelected struct{}

func (CloseSelected) isAction()  {}
func (ReopenSelected) isAction() {}

// The Dashboard's own keys.
const (
	keyClose  = "c"
	keyReopen = "o"
)

// Bindings are the keys Key acts on besides the cursor keys, as the root hints them.
func Bindings() []layout.Binding {
	return []layout.Binding{{Key: keyClose, Action: "close"}, {Key: keyReopen, Action: "reopen"}}
}

// Key applies a key: c and o ask for the selected account to be closed or reopened, the cursor keys move.
func (m Model) Key(key string) (Model, Action) {
	switch key {
	case keyClose:
		return m, CloseSelected{}
	case keyReopen:
		return m, ReopenSelected{}
	}
	return m.Move(key), nil
}

// Move applies a cursor key: up, down, home, end, pgup or pgdown. The cursor stops at both ends.
func (m Model) Move(key string) Model {
	next := m
	next.cursor = m.cursor.Move(key, len(m.accounts), m.pageSize())
	return next
}

// moveTo puts the cursor on row i, clamped, and scrolls to keep it visible.
func (m Model) moveTo(i int) Model {
	next := m
	next.cursor = m.cursor.To(i, len(m.accounts), m.pageSize())
	return next
}

// pageSize is how many account rows the panel shows at once.
func (m Model) pageSize() int { return max(1, m.height-layout.PanelChrome) }

// Position is the 1-based cursor row and the row count; (0, 0) when there are no accounts.
func (m Model) Position() (row, total int) {
	if len(m.accounts) == 0 {
		return 0, 0
	}
	return m.cursor.Row + 1, len(m.accounts)
}

// Selected is the account under the cursor; false when the panel is empty.
func (m Model) Selected() (domain.Account, bool) {
	if len(m.accounts) == 0 {
		return domain.Account{}, false
	}
	return m.accounts[m.cursor.Row], true
}

// View draws the Accounts panel in the left column and the stat cards above whatever fills the right.
func (m Model) View() string {
	left := m.accountsPanel()
	fit := m.fit()
	if !fit.right {
		return left
	}
	right := m.width - m.accountsWidth()
	cards := lipgloss.JoinHorizontal(lipgloss.Top, m.cards(right)...)
	column := cards + "\n" + m.listPanels(fit, right, m.height-cardHeight)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, column)
}

// fitting is which of the panels beside Accounts the terminal has room for: the right column (the stat
// cards over Recent), the lower band below Recent (Upcoming) and, beside Upcoming, the spend chart.
type fitting struct{ right, band, chart bool }

// fit decides which panels beside Accounts are shown; View draws them and HiddenPanels counts the others.
func (m Model) fit() fitting {
	right := m.width - m.accountsWidth()
	var fit fitting
	fit.right = right >= cardCount*minCardWidth
	fit.band = fit.right && m.height-cardHeight >= upcomingHeight+minRecentHeight
	fit.chart = fit.band && right >= bandPanels*minBandWidth
	return fit
}

// HiddenPanels is how many of the panels beside Accounts the terminal is too small to show: each part of
// the fit left out hides its own panels and, since the parts nest, those of the parts within it.
func (m Model) HiddenPanels() int {
	fit, hidden := m.fit(), 0
	parts := []struct {
		shown  bool
		panels int
	}{{fit.right, cardCount + 1}, {fit.band, 1}, {fit.chart, 1}} // the cards and Recent, Upcoming, the chart
	for _, part := range parts {
		if !part.shown {
			hidden += part.panels
		}
	}
	return hidden
}

// listPanels stacks Recent over Upcoming beside the spend chart, leaving out what does not fit.
func (m Model) listPanels(fit fitting, width, height int) string {
	if !fit.band {
		return entriesPanel("RECENT", m.recent, width, height)
	}
	recent := entriesPanel("RECENT", m.recent, width, height-upcomingHeight)
	if !fit.chart {
		return recent + "\n" + entriesPanel("UPCOMING", m.upcoming, width, upcomingHeight)
	}
	half := width / bandPanels
	band := lipgloss.JoinHorizontal(lipgloss.Top,
		entriesPanel("UPCOMING", m.upcoming, half, upcomingHeight),
		spendPanel(m.spend, width-half, upcomingHeight))
	return recent + "\n" + band
}

// entriesPanel lists transactions one per row: date, description, category, amount. When they do not
// all fit, the last row says how many more there are. Rows are not selectable.
func entriesPanel(title string, entries []domain.RecentEntry, width, height int) string {
	inner, capacity := max(0, width-layout.PanelChrome), max(0, height-layout.PanelChrome)
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
		rows = append(rows, layout.More(hidden))
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
	left := " " + entry.Date.Format(time.DateOnly) + "  " + layout.Fit(entry.Description, c.description)
	left += "  " + layout.Fit(entry.Category, c.category)
	gap := strings.Repeat(" ", max(1, c.width-lipgloss.Width(left)-lipgloss.Width(amount)-1))
	return left + gap + style.Render(amount)
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
		plain := amounts[0]
		plain.Commodity = ""
		return plain.Format()
	}
	return amounts.Format()
}

// accountsWidth is a third of the screen, but never narrower than minAccountsWidth.
func (m Model) accountsWidth() int { return min(m.width, max(minAccountsWidth, m.width/columnDivisor)) }

// accountsPanel is the Accounts panel, a third of the screen wide.
func (m Model) accountsPanel() string {
	width := m.accountsWidth()
	inner := max(0, width-layout.PanelChrome)
	last := min(len(m.accounts), m.cursor.Offset+m.pageSize())
	visible := m.accounts[m.cursor.Offset:last]
	rows := make([]string, len(visible))
	for i, account := range visible {
		rows[i] = accountLine(account, inner, m.cursor.Offset+i == m.cursor.Row)
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
