// Package reports is the Reports view: the balance sheet and the income statement side by side,
// scrolled together, for a period the view itself remembers.
package reports

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	periodRows = 1 // the period bar above the panels
	panelCount = 2
	columnGap  = 2
	deltaTitle = "Δ period"
)

// Model is the Reports view's local state: the period it shows and how far the tables are scrolled.
type Model struct {
	period  domain.ReportPeriod
	report  domain.Report
	scroll  layout.Cursor // how far both tables are scrolled
	width   int
	height  int
	loaded  bool  // the period's report has arrived
	failure error // why the period's report could not be built, until one is
}

// New returns a Reports view on month to date with nothing loaded yet.
func New() Model { return Model{} }

// Period is the period the view shows; the Dashboard's month to date is separate.
func (m Model) Period() domain.ReportPeriod { return m.period }

// Action is what a key asks the root model to do: LoadReport; nil asks for nothing.
type Action interface{ isAction() }

// LoadReport asks for the report of the period now shown to be built.
type LoadReport struct{}

func (LoadReport) isAction() {}

// keyPeriod moves to the next period.
const keyPeriod = "p"

// Bindings are the keys Key acts on besides the cursor keys, as the root hints them.
func Bindings() []layout.Binding { return []layout.Binding{{Key: keyPeriod, Action: "period"}} }

// Key applies a key: p moves to the next period and asks for its report, the other keys scroll.
func (m Model) Key(key string) (Model, Action) {
	if key == keyPeriod {
		return m.NextPeriod(), LoadReport{}
	}

	return m.Move(key), nil
}

// NextPeriod moves to the next period (month, quarter, year, all time, month), its tables empty until its
// report arrives.
func (m Model) NextPeriod() Model {
	m.period = m.period.Next()
	m.report, m.loaded, m.failure, m.scroll = domain.Report{}, false, nil, layout.Cursor{}

	return m
}

// WithReport shows a freshly built report, keeping the scroll position when it still fits.
func (m Model) WithReport(report domain.Report) Model {
	m.report, m.loaded, m.failure = report, true, nil
	m.scroll.Offset = min(m.scroll.Offset, m.maxOffset())

	return m
}

// WithFailure says why the period's report could not be built; a report already on screen stays, since the
// status bar flags the failure.
func (m Model) WithFailure(err error) Model {
	m.failure = err

	return m
}

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	m.width, m.height = width, height
	m.scroll.Offset = min(m.scroll.Offset, m.maxOffset())

	return m
}

// capacity is how many rows each panel shows.
func (m Model) capacity() int { return max(1, m.height-periodRows-layout.PanelChrome) }

// room is how many rows of the table fit below its title row, if it has one.
func (m Model) room(rows []domain.ReportRow) int {
	if columnsOf(rows).delta > 0 {
		return max(1, m.capacity()-1)
	}

	return m.capacity()
}

// maxOffset is the furthest the tables scroll: the last row of the table with the most to scroll at the bottom.
func (m Model) maxOffset() int {
	return max(0, len(m.report.BalanceSheet)-m.room(m.report.BalanceSheet),
		len(m.report.IncomeStatement)-m.room(m.report.IncomeStatement))
}

// Move scrolls both tables together: up, down, pgup, pgdown, home or end.
func (m Model) Move(key string) Model {
	m.scroll = m.scroll.Scroll(key, m.maxOffset(), m.capacity())

	return m
}

// View draws the period bar above the two panels.
func (m Model) View() string {
	bar := " " + theme.Dim.Render("period") + "  " + theme.Bold.Render(m.period.String()) + m.state()
	sheetWidth := m.width / panelCount
	sheet := m.panel("BALANCE SHEET", m.report.BalanceSheet, sheetWidth)
	statement := m.panel("INCOME STATEMENT", m.report.IncomeStatement, m.width-sheetWidth)

	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, sheet, statement)
}

// panel draws one table in a panel width cells wide, below the period bar.
func (m Model) panel(title string, rows []domain.ReportRow, width int) string {
	return layout.Panel(title, width, m.height-periodRows, m.table(rows, width-layout.PanelChrome))
}

// state is what the period bar says after the period until its report is on screen: that it is being built,
// or why it could not be.
func (m Model) state() string {
	if m.loaded {
		return ""
	}

	if m.failure != nil {
		return "  " + theme.Negative.Render("cannot build the reports: "+m.failure.Error())
	}

	return "  " + theme.Dim.Render("loading the reports…")
}

// table draws the rows from the scroll offset on, the last visible row giving way to "… N more". A
// table that fits does not scroll, so its last row stays in view while a longer one is scrolled.
func (m Model) table(rows []domain.ReportRow, width int) string {
	columns := columnsOf(rows)
	capacity := m.room(rows)

	lines := make([]string, 0, capacity)
	if columns.delta > 0 {
		lines = append(lines, columns.titleRow(width))
	}
	shown, hidden := m.shownRows(rows, capacity)
	for _, row := range shown {
		lines = append(lines, columns.line(row, width))
	}

	if hidden > 0 {
		lines = append(lines, layout.More(hidden))
	}

	return strings.Join(lines, "\n")
}

// shownRows are the rows that fit in capacity from the scroll offset on, and how many after them are hidden;
// a row gives way to the "… N more" line when any are.
func (m Model) shownRows(rows []domain.ReportRow, capacity int) (shown []domain.ReportRow, hidden int) {
	first := min(m.scroll.Offset, max(0, len(rows)-capacity))

	last := min(len(rows), first+capacity)
	if last < len(rows) {
		last--
	}

	return rows[first:last], len(rows) - last
}

// columns are the widths of a table's amount columns; delta is 0 when the table has none.
type columns struct{ amount, delta int }

func columnsOf(rows []domain.ReportRow) columns {
	var widths columns
	for _, row := range rows {
		if row.Kind != domain.RowHeader {
			widths.amount = max(widths.amount, lipgloss.Width(row.Amount.Format()))
		}

		if row.Delta != nil {
			widths.delta = max(widths.delta, lipgloss.Width(row.Delta.Format()), lipgloss.Width(deltaTitle))
		}
	}

	return widths
}

func (c columns) titleRow(width int) string {
	return strings.Repeat(" ", max(0, width-c.delta-1)) + theme.Dim.Render(layout.AlignRight(deltaTitle, c.delta)) + " "
}

// line is one row: label at the left, the amount and the change right-aligned.
func (c columns) line(row domain.ReportRow, width int) string {
	if row.Kind == domain.RowHeader {
		return " " + theme.PanelTitle.Bold(true).Render(row.Label)
	}

	change := ""
	if c.delta > 0 {
		change = c.change(row)
	}
	room := max(0, width-c.amount-columnGap-lipgloss.Width(change)-columnGap)
	label := " " + indent(row) + rowStyle(row).Render(layout.Fit(row.Label, room-lipgloss.Width(indent(row))))
	figure := layout.AlignRight(amountStyle(row).Render(row.Amount.Format()), c.amount)

	return label + strings.Repeat(" ", columnGap-1) + figure + change + " "
}

// change is the row's cell of the change column, right-aligned after a gap; blank for a row without a change.
func (c columns) change(row domain.ReportRow) string {
	change := strings.Repeat(" ", columnGap+c.delta)
	if row.Delta != nil {
		delta := deltaStyle(*row.Delta).Render(row.Delta.Format())
		change = strings.Repeat(" ", columnGap) + layout.AlignRight(delta, c.delta)
	}

	return change
}

func indent(row domain.ReportRow) string {
	if row.Kind == domain.RowLeaf {
		return " "
	}

	return ""
}

func rowStyle(row domain.ReportRow) lipgloss.Style {
	if row.Kind == domain.RowGrandTotal {
		return theme.Emphasis
	}

	return lipgloss.NewStyle()
}

func amountStyle(row domain.ReportRow) lipgloss.Style {
	switch {
	case row.Kind == domain.RowGrandTotal:
		return theme.Emphasis
	case row.Amount.IsAllNegative():
		return theme.Negative
	}

	return lipgloss.NewStyle()
}

func deltaStyle(delta domain.MultiAmount) lipgloss.Style {
	switch {
	case delta.IsZero():
		return theme.Dim
	case delta.IsAllNegative():
		return theme.Negative
	}

	return theme.Ok
}
