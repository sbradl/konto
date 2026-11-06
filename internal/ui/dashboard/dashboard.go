// Package dashboard is the Dashboard view: the Accounts panel with its cursor over Upcoming, and beside them
// the stat cards, Recent and the monthly spend chart as far as the terminal has room for them. Tab moves the
// focus between Accounts, Upcoming and Recent; the cursor keys move the Accounts cursor or scroll the others.
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
	upcomingHeight      = 10 // the least Upcoming takes under Accounts; it gets all the room Accounts leaves
	minAccountsHeight   = 5  // Upcoming gives way rather than squeeze Accounts below this
	spendHeight         = 10 // the spend chart under Recent
	minRecentHeight     = 5  // the chart gives way rather than squeeze Recent below this
)

// Model is the Dashboard's view-local state; the ledger is replaced wholesale on refresh.
type Model struct {
	accounts []domain.Account
	recent   []domain.RecentEntry
	upcoming []domain.RecentEntry
	spend    []domain.MonthSpend
	figures  figures
	cursor   layout.Cursor
	focus    panel
	scrolls  [panelCount]layout.Cursor // the first row Upcoming and Recent show; Accounts uses cursor
	width    int
	height   int
}

// panel is one of the panels the focus moves between, in tab order.
type panel int

const (
	accountsPanel panel = iota
	upcomingPanel
	recentPanel
	panelCount
)

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
	return []layout.Binding{
		{Key: layout.KeyTab, Action: "focus"}, {Key: keyClose, Action: "close"}, {Key: keyReopen, Action: "reopen"},
	}
}

// Key applies a key: c and o ask for the selected account to be closed or reopened, tab and shift+tab move
// the focus, the cursor keys move or scroll the focused panel.
func (m Model) Key(key string) (Model, Action) {
	switch key {
	case keyClose:
		return m, CloseSelected{}
	case keyReopen:
		return m, ReopenSelected{}
	case layout.KeyTab:
		return m.cycle(1), nil
	case layout.KeyBackTab:
		return m.cycle(-1), nil
	}

	return m.Move(key), nil
}

// cycle moves the focus by step through the panels shown, wrapping around.
func (m Model) cycle(step int) Model {
	next := m
	shown := m.shownPanels()
	at := slices.Index(shown, m.focused())
	next.focus = shown[(at+step+len(shown))%len(shown)]

	return next
}

// shownPanels are the focusable panels the terminal has room for, in tab order.
func (m Model) shownPanels() []panel {
	fit := m.fit()

	shown := []panel{accountsPanel}
	if fit.upcoming {
		shown = append(shown, upcomingPanel)
	}

	if fit.right {
		shown = append(shown, recentPanel)
	}

	return shown
}

// focused is the panel with the focus; Accounts when the focused panel no longer fits.
func (m Model) focused() panel {
	if slices.Contains(m.shownPanels(), m.focus) {
		return m.focus
	}

	return accountsPanel
}

// Move applies a cursor key: up, down, home, end, pgup or pgdown. It moves the Accounts cursor, stopping at
// both ends, or scrolls the focused Upcoming or Recent panel until its last entry shows.
func (m Model) Move(key string) Model {
	next := m

	focus := m.focused()
	if focus == accountsPanel {
		next.cursor = m.cursor.Move(key, len(m.accounts), m.pageSize())

		return next
	}
	entries, capacity := m.entries(focus), m.capacity(focus)
	next.scrolls[focus] = m.scrolls[focus].Scroll(key, lastOffset(len(entries), capacity), capacity)

	return next
}

// entries are what the Upcoming or Recent panel lists.
func (m Model) entries(focus panel) []domain.RecentEntry {
	if focus == upcomingPanel {
		return m.upcoming
	}

	return m.recent
}

// capacity is how many rows fit inside the Upcoming or Recent panel.
func (m Model) capacity(focus panel) int {
	return max(0, m.panelHeight(focus)-layout.PanelChrome)
}

// panelHeight is the Upcoming or Recent panel's height: Upcoming takes what Accounts leaves, Recent what
// the cards and the chart leave.
func (m Model) panelHeight(focus panel) int {
	if focus == upcomingPanel {
		return m.height - m.accountsHeight()
	}

	if m.fit().chart {
		return m.height - cardHeight - spendHeight
	}

	return m.height - cardHeight
}

// lastOffset is the furthest a panel of capacity rows scrolls into count entries: its last page.
func lastOffset(count, capacity int) int { return max(0, count-capacity) }

// moveTo puts the cursor on row i, clamped, and scrolls to keep it visible.
func (m Model) moveTo(i int) Model {
	next := m
	next.cursor = m.cursor.To(i, len(m.accounts), m.pageSize())

	return next
}

// pageSize is how many account rows the panel shows at once.
func (m Model) pageSize() int { return max(1, m.accountsHeight()-layout.PanelChrome) }

// accountsHeight is just tall enough for every account when Upcoming fits under it, leaving Upcoming at
// least upcomingHeight; the whole height when it does not.
func (m Model) accountsHeight() int {
	if !m.fit().upcoming {
		return m.height
	}

	return min(len(m.accounts)+layout.PanelChrome, m.height-upcomingHeight)
}

// Position is the focused panel's 1-based cursor row, or first row shown, and its row count; (0, 0) when the
// panel is empty.
func (m Model) Position() (row, total int) {
	focus := m.focused()
	if focus == accountsPanel {
		if len(m.accounts) == 0 {
			return 0, 0
		}

		return m.cursor.Row + 1, len(m.accounts)
	}

	entries := m.entries(focus)
	if len(entries) == 0 {
		return 0, 0
	}

	return m.offset(focus) + 1, len(entries)
}

// offset is the first entry the Upcoming or Recent panel shows, kept within its last page as the ledger and
// the terminal change.
func (m Model) offset(focus panel) int {
	return min(m.scrolls[focus].Offset, lastOffset(len(m.entries(focus)), m.capacity(focus)))
}

// title is the panel's title, marked when the panel has the focus.
func (m Model) title(focus panel, title string) string {
	if m.focused() == focus {
		return title + " ◂"
	}

	return title
}

// Selected is the account under the cursor; false when the panel is empty.
func (m Model) Selected() (domain.Account, bool) {
	if len(m.accounts) == 0 {
		return domain.Account{}, false
	}

	return m.accounts[m.cursor.Row], true
}

// View draws Accounts over Upcoming in the left column and the stat cards over Recent and the spend chart
// in the right.
func (m Model) View() string {
	fit := m.fit()

	left := m.accountsPanel()
	if fit.upcoming {
		left += "\n" + m.entriesPanel(upcomingPanel, "UPCOMING", m.accountsWidth())
	}

	if !fit.right {
		return left
	}
	right := m.width - m.accountsWidth()
	column := lipgloss.JoinHorizontal(lipgloss.Top, m.cards(right)...)

	column += "\n" + m.entriesPanel(recentPanel, "RECENT", right)
	if fit.chart {
		column += "\n" + spendPanel(m.spend, right, spendHeight)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, left, column)
}

// fitting is which panels besides Accounts the terminal has room for: Upcoming under Accounts, the right
// column (the stat cards over Recent) and, under Recent, the spend chart.
type fitting struct{ upcoming, right, chart bool }

// fit decides which panels besides Accounts are shown; View draws them and HiddenPanels counts the others.
func (m Model) fit() fitting {
	var fit fitting
	fit.upcoming = m.height >= minAccountsHeight+upcomingHeight
	fit.right = m.width-m.accountsWidth() >= cardCount*minCardWidth
	fit.chart = fit.right && m.height-cardHeight >= spendHeight+minRecentHeight

	return fit
}

// HiddenPanels is how many panels besides Accounts the terminal is too small to show; the chart sits in the
// right column, so it hides with it.
func (m Model) HiddenPanels() int {
	fit, hidden := m.fit(), 0
	if !fit.upcoming {
		hidden++
	}

	if !fit.right {
		hidden += cardCount + 1 // the cards and Recent
	}

	if !fit.chart {
		hidden++
	}

	return hidden
}

// entriesPanel is the Upcoming or Recent panel from its scroll offset on.
func (m Model) entriesPanel(focus panel, title string, width int) string {
	return entriesPanel(m.title(focus, title), m.entries(focus)[m.offset(focus):], width, m.panelHeight(focus))
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

	return []string{m.netWorthCard(half), m.netIncomeCard(width - half)}
}

// netWorthCard is the net worth, bold.
func (m Model) netWorthCard(width int) string {
	worth := theme.Bold.Render(m.figures.netWorth.Format())

	return card("NET WORTH", width, worth, theme.Dim.Render("assets + liabilities"))
}

// netIncomeCard is the month's net income, red when negative and green when not, with its income and expenses.
func (m Model) netIncomeCard(width int) string {
	net := m.figures.net

	style := theme.Ok
	if len(net) > 0 && net[0].IsNegative() {
		style = theme.Negative
	}
	title := strings.ToUpper(m.figures.month.String()) + " NET INCOME"
	note := number(m.figures.income) + " in / " + number(m.figures.expenses) + " out"

	return card(title, width, style.Render(net.Format()), theme.Dim.Render(note))
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

// accountsPanel is the Accounts panel, a third of the screen wide and as tall as accountsHeight.
func (m Model) accountsPanel() string {
	width := m.accountsWidth()
	inner := max(0, width-layout.PanelChrome)
	last := min(len(m.accounts), m.cursor.Offset+m.pageSize())
	visible := m.accounts[m.cursor.Offset:last]
	rows := make([]string, len(visible))
	for i, account := range visible {
		rows[i] = accountLine(account, inner, m.cursor.Offset+i == m.cursor.Row)
	}

	return layout.Panel(m.title(accountsPanel, "ACCOUNTS"), width, m.accountsHeight(), strings.Join(rows, "\n"))
}

// accountLine is one account: source glyph, name, right-aligned balance.
func accountLine(account domain.Account, width int, selected bool) string {
	glyph, glyphStyle := sourceMark(account.Source)
	balance, balanceStyle := balanceOf(account)
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

// balanceOf is the account's balance and its style: red when negative, or the closing date instead.
func balanceOf(account domain.Account) (string, lipgloss.Style) {
	switch {
	case account.IsClosed():
		return "closed " + account.Closed.Format(time.DateOnly), lipgloss.NewStyle()
	case account.Balance.IsAllNegative():
		return account.Balance.Format(), theme.Negative
	}

	return account.Balance.Format(), lipgloss.NewStyle()
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
