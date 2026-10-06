// Package scheduled is the Scheduled view: the periodic rules of the journal with the raw entry,
// the next occurrences and the annual impact of the selected one. Rules never post (ADR 0006).
package scheduled

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	panelChrome    = 2
	minListHeight  = 5
	nextWidth      = 10
	statusWidth    = 26 // "none in the next 12 months"
	columnGap      = 2
	descShare      = 40 // percent of the room between date and status that description takes
	occurrenceRows = 6
	occurrenceBox  = occurrenceRows + panelChrome
	noneUpcoming   = "none in the next 12 months"
)

// Model is the Scheduled view's state: the rules with their projections and the cursor.
type Model struct {
	ledger    domain.Ledger
	form      *form // the open new-rule or edit-rule form
	schedules []domain.RuleSchedule
	total     domain.MultiAmount
	cursor    int
	offset    int // the first visible rule
	width     int
	height    int
}

// New returns an empty Scheduled view.
func New() Model { return Model{} }

// WithLedger shows a fresh snapshot's rules; the cursor stays on its row number, or on the last row.
func (m Model) WithLedger(ledger domain.Ledger) Model {
	next := m
	next.ledger = ledger
	next.schedules = ledger.Schedules()
	next.total = ledger.AnnualImpact()
	return next.moveTo(next.cursor)
}

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	next := m
	next.width, next.height = width, height
	return next.moveTo(next.cursor)
}

// Position is the 1-based cursor row and the row count; (0, 0) when there are no rules.
func (m Model) Position() (row, total int) {
	if len(m.schedules) == 0 {
		return 0, 0
	}
	return m.cursor + 1, len(m.schedules)
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
		return m.moveTo(len(m.schedules))
	case "pgup":
		return m.moveTo(m.cursor - m.pageSize())
	case "pgdown":
		return m.moveTo(m.cursor + m.pageSize())
	}
	return m
}

func (m Model) moveTo(row int) Model {
	next := m
	next.cursor = max(0, min(row, len(m.schedules)-1))
	next.offset = max(0, min(m.offset, next.cursor))
	if next.cursor >= next.offset+m.pageSize() {
		next.offset = next.cursor - m.pageSize() + 1
	}
	return next
}

// listHeight is the RULES panel's height: its rules, but never more than half the view.
func (m Model) listHeight() int {
	return min(max(len(m.schedules)+panelChrome, minListHeight), max(minListHeight, m.height/panelChrome))
}

// pageSize is how many rules the list shows at once.
func (m Model) pageSize() int { return max(1, m.listHeight()-panelChrome) }

// View draws the list on top, then the entry panel beside the occurrences and the annual impact.
func (m Model) View() string {
	listHeight := m.listHeight()
	bottom := max(0, m.height-listHeight)
	entryWidth := m.width / panelChrome
	right := m.width - entryWidth
	occurrencesHeight := min(occurrenceBox, bottom/panelChrome)
	impactHeight := bottom - occurrencesHeight
	column := layout.Panel("OCCURRENCES", right, occurrencesHeight, m.occurrences()) + "\n" +
		layout.Panel("ANNUAL IMPACT", right, impactHeight, m.impact(right-panelChrome, impactHeight-panelChrome))
	lower := lipgloss.JoinHorizontal(lipgloss.Top, layout.Panel("ENTRY", entryWidth, bottom, m.entry()), column)
	return m.withForm(m.list(listHeight) + "\n" + lower)
}

// list is the RULES panel: next date, description, raw cadence and status of each rule.
func (m Model) list(height int) string {
	inner := max(0, m.width-panelChrome)
	if len(m.schedules) == 0 {
		return layout.Panel("RULES", m.width, height, theme.Dim.Render(" no rules — press n to add a rule"))
	}
	last := min(len(m.schedules), m.offset+m.pageSize())
	rows := make([]string, 0, m.pageSize())
	for i := m.offset; i < last; i++ {
		line := ruleLine(m.schedules[i], inner)
		if i == m.cursor {
			rows = append(rows, theme.Selected.Render(fit(line.text(), inner)))
		} else {
			rows = append(rows, line.styled())
		}
	}
	return layout.Panel("RULES", m.width, height, strings.Join(rows, "\n"))
}

// listRow is one row of the list: everything before the status, and the status in its own style.
type listRow struct {
	prefix, status string
	statusStyle    lipgloss.Style
}

func (r listRow) text() string   { return r.prefix + r.status }
func (r listRow) styled() string { return r.prefix + r.statusStyle.Render(fit(r.status, statusWidth)) }

func ruleLine(schedule domain.RuleSchedule, width int) listRow {
	spare := max(0, width-1-nextWidth-3*columnGap-statusWidth)
	description := spare * descShare / 100
	next, status, style := "", noneUpcoming, theme.Dim
	if date, ok := schedule.Next(); ok {
		next, status, style = date.Format(time.DateOnly), "upcoming", theme.Ok
	}
	gap := strings.Repeat(" ", columnGap)
	prefix := " " + fit(next, nextWidth) + gap + fit(schedule.Rule.Description, description) + gap +
		fit(schedule.Rule.Period, spare-description) + gap
	return listRow{prefix: prefix, status: status, statusStyle: style}
}

// Selected is the rule under the cursor; false when there are none.
func (m Model) Selected() (domain.PeriodicRule, bool) {
	schedule, ok := m.current()
	return schedule.Rule, ok
}

// current is the selected rule's schedule; false when there are no rules.
func (m Model) current() (domain.RuleSchedule, bool) {
	if len(m.schedules) == 0 {
		return domain.RuleSchedule{}, false
	}
	return m.schedules[m.cursor], true
}

// entry is the selected rule as written: the ~ line magenta, comments dim.
func (m Model) entry() string {
	schedule, ok := m.current()
	if !ok {
		return ""
	}
	lines := make([]string, len(schedule.Rule.Lines))
	for i, line := range schedule.Rule.Lines {
		lines[i] = " " + styleEntryLine(line)
	}
	return strings.Join(lines, "\n")
}

// styleEntryLine colours one line of a rule block: a rule line magenta up to its comment, a comment dim.
func styleEntryLine(line string) string {
	code, comment, hasComment := strings.Cut(line, ";")
	switch {
	case strings.HasPrefix(line, "~") && hasComment:
		return theme.Directive.Render(strings.TrimRight(code, " ")) + "  " + theme.Dim.Render(";"+comment)
	case strings.HasPrefix(line, "~"):
		return theme.Directive.Render(line)
	case strings.HasPrefix(strings.TrimSpace(line), ";"):
		return theme.Dim.Render(line)
	}
	return line
}

// occurrences lists the next six dates of the selected rule.
func (m Model) occurrences() string {
	schedule, ok := m.current()
	if !ok {
		return ""
	}
	if len(schedule.Occurrences) == 0 {
		return theme.Dim.Render(" " + noneUpcoming)
	}
	rows := make([]string, 0, occurrenceRows)
	for _, occurrence := range schedule.Occurrences[:min(occurrenceRows, len(schedule.Occurrences))] {
		rows = append(rows, " "+occurrence.Date.Format(time.DateOnly))
	}
	return strings.Join(rows, "\n")
}

// impact is the signed twelve-month net-worth effect of every rule, and their total.
func (m Model) impact(width, capacity int) string {
	rows := make([]string, 0, len(m.schedules)+1)
	for _, schedule := range m.schedules {
		rows = append(rows, impactLine(schedule.Rule.Description, schedule.Impact(), width, lipgloss.NewStyle()))
	}
	if len(m.schedules) > 0 {
		rows = append(rows, impactLine("Total", m.total, width, theme.Bold))
	}
	if len(rows) > capacity && capacity > 0 {
		hidden := len(rows) - (capacity - 1)
		rows = append(rows[:capacity-1], theme.Dim.Render(fmt.Sprintf(" … %d more", hidden)))
	}
	return strings.Join(rows, "\n")
}

// impactLine is a label with its amount at the right edge: green when positive, red when negative, dim at zero.
func impactLine(label string, amount domain.MultiAmount, width int, labelStyle lipgloss.Style) string {
	text := amount.Format()
	style := theme.Ok
	switch {
	case amount.IsZero():
		style = theme.Dim
	case amount.IsAllNegative():
		style = theme.Negative
	}
	room := max(0, width-lipgloss.Width(text)-columnGap-1)
	return " " + labelStyle.Render(fit(label, room)) + strings.Repeat(" ", columnGap) + style.Render(text)
}

func fit(text string, width int) string {
	cut := lipgloss.NewStyle().MaxWidth(max(0, width)).Render(text)
	return cut + strings.Repeat(" ", max(0, width-lipgloss.Width(cut)))
}
