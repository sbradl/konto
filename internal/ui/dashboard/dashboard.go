// Package dashboard is the Dashboard view: the Accounts panel and beside it the stat cards, and under them
// the Activity panel, the upcoming occurrences over a today line over the recent transactions, as far as the
// terminal has room for them. Tab moves the focus between Accounts and Activity; the cursor keys move the
// Accounts cursor or scroll Activity.
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
	maxCategoryWidth    = 24 // the Activity panel's widest category column
	minDescriptionWidth = 8
	descriptionShare    = 60 // percent of the room beside the date and amount that description takes
	recentGap           = 2
	recentFixed         = 16 // an Activity row's edges, its date and the gaps around the middle columns
	minFigureWidth      = 15 // an amount or balance column holds at least -100.000,00 EUR
	activityHeight      = 10 // the least Activity takes under Accounts; it gets all the room Accounts leaves
	minAccountsHeight   = 5  // Activity gives way rather than squeeze Accounts below this
)

// Model is the Dashboard's view-local state; the ledger is replaced wholesale on refresh.
type Model struct {
	accounts []domain.Account
	recent   []domain.RecentEntry // newest first
	upcoming []domain.RecentEntry // soonest first
	rows     []activityRow        // Activity's rows, top to bottom
	todayRow int                  // the today line's index in rows
	today    time.Time
	figures  figures
	cursor   layout.Cursor
	focus    panel
	scroll   layout.Cursor // the first Activity row shown, once scrolled
	scrolled bool          // until Activity is scrolled it keeps the today line in its middle
	width    int
	height   int
}

// panel is one of the panels the focus moves between, in tab order.
type panel int

const (
	accountsPanel panel = iota
	activityPanel
)

// figures are the numbers the stat cards show.
type figures struct {
	netWorth domain.MultiAmount
	month    time.Month
	net      domain.MultiAmount
	income   domain.MultiAmount
	expenses domain.MultiAmount
}

// WithLedger shows a fresh snapshot: its accounts, the stat cards' figures and the activity around today.
func (m Model) WithLedger(ledger domain.Ledger) Model {
	next := m
	next.figures = figures{
		netWorth: ledger.NetWorth(), month: ledger.Month.Month,
		net: ledger.MonthNet(), income: ledger.MonthIncome(), expenses: ledger.MonthExpenses(),
	}
	next.recent = ledger.Recent()
	next.upcoming = ledger.UpcomingEntries()
	next.today = ledger.Today
	next.rows, next.todayRow = timeline(next.upcoming, next.recent, ledger.MonthTotals())

	return next.WithAccounts(ledger.Accounts)
}

// New returns an empty Dashboard.
func New() Model { return Model{rows: []activityRow{{}}} } // the today line alone

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
	keyToday  = "t"
)

// Bindings are the keys Key acts on besides the cursor keys, as the root hints them.
func Bindings() []layout.Binding {
	return []layout.Binding{
		{Key: layout.KeyTab, Action: "focus"},
		{Key: keyClose, Action: "close"},
		{Key: keyReopen, Action: "reopen"},
		{Key: keyToday, Action: "today"},
	}
}

// Key applies a key: c and o ask for the selected account to be closed or reopened, tab and shift+tab move
// the focus, t puts Activity's today line back in its middle, the cursor keys move or scroll the focused panel.
func (m Model) Key(key string) (Model, Action) {
	switch key {
	case keyClose:
		return m, CloseSelected{}
	case keyReopen:
		return m, ReopenSelected{}
	case layout.KeyTab, layout.KeyBackTab:
		return m.toggle(), nil
	case keyToday:
		next := m
		next.scrolled = false

		return next, nil
	}

	return m.Move(key), nil
}

// toggle moves the focus to the other panel; it stays on Accounts while Activity does not fit.
func (m Model) toggle() Model {
	next := m
	next.focus = accountsPanel

	if m.focused() == accountsPanel && m.fit().activity {
		next.focus = activityPanel
	}

	return next
}

// focused is the panel with the focus; Accounts when Activity no longer fits.
func (m Model) focused() panel {
	if m.focus == activityPanel && m.fit().activity {
		return activityPanel
	}

	return accountsPanel
}

// Move applies a cursor key: up, down, home, end, pgup or pgdown. It moves the Accounts cursor, stopping at
// both ends, or scrolls Activity between its first and last page.
func (m Model) Move(key string) Model {
	next := m
	if m.focused() == accountsPanel {
		next.cursor = m.cursor.Move(key, len(m.accounts), m.pageSize())

		return next
	}
	capacity := m.capacity()
	next.scroll = layout.Cursor{Offset: m.offset()}.Scroll(key, lastOffset(m.rowCount(), capacity), capacity)
	next.scrolled = true

	return next
}

// rowCount is how many rows Activity lists: the upcoming occurrences, the today line and the recent ones.
func (m Model) rowCount() int { return len(m.rows) }

// capacity is how many rows fit inside Activity.
func (m Model) capacity() int { return max(0, m.activityHeight()-layout.PanelChrome) }

// activityHeight is what Accounts and the cards leave of the height.
func (m Model) activityHeight() int { return m.height - m.accountsHeight() }

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

// accountsHeight is just tall enough for every account, and for the cards beside it, when Activity fits under
// it, leaving Activity at least activityHeight; the whole height when it does not.
func (m Model) accountsHeight() int {
	if !m.fit().activity {
		return m.height
	}

	return max(cardHeight, min(len(m.accounts)+layout.PanelChrome, m.height-activityHeight))
}

// Position is the focused panel's 1-based cursor row, or first row shown, and its row count; (0, 0) when the
// panel is empty.
func (m Model) Position() (row, total int) {
	if m.focused() == activityPanel {
		return m.offset() + 1, m.rowCount()
	}

	if len(m.accounts) == 0 {
		return 0, 0
	}

	return m.cursor.Row + 1, len(m.accounts)
}

// offset is the first row Activity shows: the today line in the middle until it is scrolled, and always
// within its last page as the ledger and the terminal change.
func (m Model) offset() int {
	offset := m.scroll.Offset
	if !m.scrolled {
		offset = max(0, m.todayRow-m.capacity()/2)
	}

	return min(offset, lastOffset(m.rowCount(), m.capacity()))
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

// View draws Accounts with the stat cards beside it, and Activity across the full width under them.
func (m Model) View() string {
	fit := m.fit()

	top := m.accountsPanel()
	if fit.cards {
		top = lipgloss.JoinHorizontal(lipgloss.Top, append([]string{top}, m.cards(m.width-m.accountsWidth())...)...)
	}

	if !fit.activity {
		return top
	}

	return top + "\n" + m.activityPanel()
}

// fitting is which panels besides Accounts the terminal has room for: the stat cards beside it and Activity
// under it.
type fitting struct{ cards, activity bool }

// fit decides which panels besides Accounts are shown; View draws them and HiddenPanels counts the others.
func (m Model) fit() fitting {
	return fitting{
		cards:    m.width-m.accountsWidth() >= cardCount*minCardWidth,
		activity: m.height >= minAccountsHeight+activityHeight,
	}
}

// HiddenPanels is how many panels besides Accounts the terminal is too small to show.
func (m Model) HiddenPanels() int {
	fit, hidden := m.fit(), 0
	if !fit.cards {
		hidden += cardCount
	}

	if !fit.activity {
		hidden++
	}

	return hidden
}

// activityPanel lists, newest at the top, the upcoming occurrences, the today line and the recent
// transactions, from its scroll offset on. A first or last row that has more beyond it says how many; it
// always has room for several rows when it fits at all.
// Rows are not selectable.
func (m Model) activityPanel() string {
	offset := m.offset()
	end := min(m.rowCount(), offset+m.capacity())
	rows := m.activityLines(offset, end, max(0, m.width-layout.PanelChrome))

	if hidden := m.rowCount() - end; hidden > 0 {
		rows[len(rows)-1] = layout.More(hidden + 1)
	}

	if offset > 0 {
		rows[0] = layout.More(offset + 1)
	}

	return layout.Panel(m.title(activityPanel, "ACTIVITY"), m.width, m.activityHeight(), strings.Join(rows, "\n"))
}

// activityLines draws Activity's rows first up to end, the columns sized to every entry so they keep their
// place while it scrolls.
func (m Model) activityLines(first, end, width int) []string {
	columns := recentColumns(slices.Concat(m.upcoming, m.recent), width)
	var lines []string
	for i := first; i < end; i++ {
		row := m.rows[i]
		switch {
		case row.isMonth:
			lines = append(lines, monthLine(row.month, row.totals, width))
		case row.isEntry:
			lines = append(lines, columns.line(row.entry))
		default:
			lines = append(lines, todayLine(m.today, width))
		}
	}

	return lines
}

// activityRow is one row of Activity: a transaction, a month's totals, or else the today line.
type activityRow struct {
	isEntry, isMonth bool
	entry            domain.RecentEntry
	month            time.Time // the first of the month the totals are for
	totals           domain.MonthSummary
}

// timeline is the upcoming occurrences furthest first, the today line and the recent transactions newest
// first, each month opened by its totals line above its latest row, and the today line's index.
func timeline(
	upcoming, recent []domain.RecentEntry, totals map[time.Time]domain.MonthSummary,
) (rows []activityRow, today int) {
	var month time.Time
	add := func(entry domain.RecentEntry) {
		if first := domain.FirstOfMonth(entry.Date); !first.Equal(month) {
			month = first
			rows = append(rows, activityRow{isMonth: true, month: first, totals: totals[first]})
		}
		rows = append(rows, activityRow{isEntry: true, entry: entry})
	}
	for _, entry := range slices.Backward(upcoming) {
		add(entry)
	}
	today = len(rows)
	rows = append(rows, activityRow{})
	for _, entry := range recent {
		add(entry)
	}

	return rows, today
}

// monthLine is a yellow rule like the today line, but thin, carrying the month, its net change green or red,
// then in brackets its income in green and its expenses in red, each signed.
func monthLine(month time.Time, totals domain.MonthSummary, width int) string {
	rule := func(text string) string { return theme.Warn.Render(text) }
	net := totals.Net()
	label := rule(" ── ") + theme.Bold.Render(month.Format("January 2006")) + rule(" ── ") +
		signStyle(net).Render(layout.Signed(net)) + " (" + theme.Ok.Render(layout.Signed(totals.Income)) + " / " +
		theme.Negative.Render(totals.Expenses.Negated().Format()) + ") "

	return label + rule(strings.Repeat("─", max(0, width-lipgloss.Width(label)-1)))
}

// todayLine is the rule between the upcoming and the recent rows, carrying today's date.
func todayLine(today time.Time, width int) string {
	label := " ━━ today " + today.Format(time.DateOnly) + " "

	return theme.Warn.Render(label + strings.Repeat("━", max(0, width-lipgloss.Width(label)-1)))
}

// recent column widths for one panel width: what is left of the row after the date and the widest amount
// and balance is shared between description and category.
type columns struct {
	width, amount, balance, description, category int
}

func recentColumns(entries []domain.RecentEntry, width int) columns {
	amount, balance := minFigureWidth, minFigureWidth
	for _, entry := range entries {
		amount = max(amount, lipgloss.Width(entry.Amount.Format()))
		balance = max(balance, lipgloss.Width(entry.Balance.Format()))
	}
	spare := max(0, width-recentFixed-amount-recentGap-balance)
	description := max(minDescriptionWidth, spare*descriptionShare/100)

	return columns{
		width: width, amount: amount, balance: balance, description: min(description, spare),
		category: min(maxCategoryWidth, max(0, spare-description-recentGap)),
	}
}

// line is one transaction, its date red when overdue: the amount, then the balance after it at the right edge.
func (c columns) line(entry domain.RecentEntry) string {
	date := entry.Date.Format(time.DateOnly)
	if entry.Overdue {
		date = theme.Negative.Render(date)
	}

	left := " " + date + "  " + layout.Fit(entry.Description, c.description)
	if c.category > 0 { // a narrow screen leaves the category out rather than squeeze the figures
		left += "  " + layout.Fit(entry.Category, c.category)
	}
	right := layout.AlignRight(signed(entry.Amount), c.amount) + strings.Repeat(" ", recentGap)
	right += layout.AlignRight(signed(entry.Balance), c.balance)
	gap := strings.Repeat(" ", max(1, c.width-lipgloss.Width(left)-lipgloss.Width(right)-1))

	return left + gap + right
}

// signed is the amount red when negative and green when not.
func signed(amount domain.MultiAmount) string { return signStyle(amount).Render(amount.Format()) }

// signStyle is red for a negative amount and green for any other.
func signStyle(amount domain.MultiAmount) lipgloss.Style {
	if amount.IsAllNegative() {
		return theme.Negative
	}

	return theme.Ok
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
