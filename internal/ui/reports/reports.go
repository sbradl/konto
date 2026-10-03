// Package reports is the Reports view: the balance sheet and the income statement side by side,
// scrolled together, for a period the view itself remembers.
package reports

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	panelChrome = 2 // border rows or columns around a panel body
	periodRows  = 1 // the period bar above the panels
	panelCount  = 2
	columnGap   = 2
	deltaTitle  = "Δ period"
)

// Model is the Reports view's local state: the period it shows and how far the tables are scrolled.
type Model struct {
	period domain.Period
	report domain.Report
	offset int
	width  int
	height int
}

// New returns a Reports view on month to date with nothing loaded yet.
func New() Model { return Model{} }

// Period is the period the view shows; the Dashboard's month to date is separate.
func (m Model) Period() domain.Period { return m.period }

// NextPeriod moves to the next period (month, quarter, year, all time, month).
func (m Model) NextPeriod() Model {
	m.period = m.period.Next()
	return m
}

// WithReport shows a freshly built report, keeping the scroll position when it still fits.
func (m Model) WithReport(report domain.Report) Model {
	m.report = report
	m.offset = min(m.offset, m.maxOffset())
	return m
}

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	m.width, m.height = width, height
	m.offset = min(m.offset, m.maxOffset())
	return m
}

// capacity is how many rows each panel shows.
func (m Model) capacity() int { return max(1, m.height-periodRows-panelChrome) }

func (m Model) longest() int {
	return max(len(m.report.BalanceSheet), len(m.report.IncomeStatement))
}

// maxOffset is the furthest the tables scroll: the longest table's last row at the bottom.
func (m Model) maxOffset() int { return max(0, m.longest()-m.capacity()) }

// Move scrolls both tables together: up, down, pgup, pgdown, home or end.
func (m Model) Move(key string) Model {
	target := m.offset
	switch key {
	case "up":
		target--
	case "down":
		target++
	case "pgup":
		target -= m.capacity()
	case "pgdown":
		target += m.capacity()
	case "home":
		target = 0
	case "end":
		target = m.maxOffset()
	}
	m.offset = max(0, min(target, m.maxOffset()))
	return m
}

// View draws the period bar above the two panels.
func (m Model) View() string {
	bar := " " + theme.Dim.Render("period") + "  " + theme.Bold.Render(m.period.String())
	sheetWidth := m.width / panelCount
	height := m.height - periodRows
	sheet := layout.Panel("BALANCE SHEET", sheetWidth, height, m.table(m.report.BalanceSheet, sheetWidth-panelChrome))
	statementWidth := m.width - sheetWidth
	statement := layout.Panel("INCOME STATEMENT", statementWidth, height,
		m.table(m.report.IncomeStatement, statementWidth-panelChrome))
	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, sheet, statement)
}

// table draws the rows from the scroll offset on, the last visible row giving way to "… N more". A
// table that fits does not scroll, so its last row stays in view while a longer one is scrolled.
func (m Model) table(rows []domain.ReportRow, width int) string {
	columns := columnsOf(rows)
	capacity := m.capacity()
	lines := make([]string, 0, capacity+1)
	if columns.delta > 0 {
		lines = append(lines, columns.titleRow(width))
		capacity--
	}
	first := min(m.offset, max(0, len(rows)-capacity))
	last := min(len(rows), first+capacity)
	if last < len(rows) {
		last--
	}
	for _, row := range rows[first:last] {
		lines = append(lines, columns.line(row, width))
	}
	if hidden := len(rows) - last; hidden > 0 {
		lines = append(lines, theme.Dim.Render(fmt.Sprintf(" … %d more", hidden)))
	}
	return strings.Join(lines, "\n")
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
			widths.delta = max(widths.delta, lipgloss.Width(deltaText(*row.Delta)), lipgloss.Width(deltaTitle))
		}
	}
	return widths
}

// deltaText is the change as a figure; a change of nothing is a plain 0.
func deltaText(delta domain.MultiAmount) string {
	if delta.IsZero() {
		return "0"
	}
	return delta.Format()
}

func (c columns) titleRow(width int) string {
	return strings.Repeat(" ", max(0, width-c.delta-1)) + theme.Dim.Render(padLeft(deltaTitle, c.delta)) + " "
}

// line is one row: label at the left, the amount and the change right-aligned.
func (c columns) line(row domain.ReportRow, width int) string {
	if row.Kind == domain.RowHeader {
		return " " + theme.PanelTitle.Bold(true).Render(row.Label)
	}
	amount, change := row.Amount.Format(), ""
	if c.delta > 0 {
		change = strings.Repeat(" ", columnGap+c.delta)
		if row.Delta != nil {
			change = strings.Repeat(" ", columnGap) + alignRight(deltaText(*row.Delta), c.delta, deltaStyle(*row.Delta))
		}
	}
	room := max(0, width-c.amount-columnGap-lipgloss.Width(change)-columnGap)
	label := " " + indent(row) + rowStyle(row).Render(fit(row.Label, room-lipgloss.Width(indent(row))))
	return label + strings.Repeat(" ", columnGap-1) + alignRight(amount, c.amount, amountStyle(row)) + change + " "
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

// alignRight pads text to width on the left, the padding outside the style.
func alignRight(text string, width int, style lipgloss.Style) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(text))) + style.Render(text)
}

func padLeft(text string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(text))) + text
}

func fit(text string, width int) string {
	cut := lipgloss.NewStyle().MaxWidth(width).Render(text)
	return cut + strings.Repeat(" ", max(0, width-lipgloss.Width(cut)))
}
