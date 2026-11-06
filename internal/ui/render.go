package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/theme"
)

// chromeRows is the height of the tab bar, the rule below it and the status bar.
const chromeRows = 3

// View renders the current screen.
func (m Model) View() tea.View {
	view := tea.NewView(m.content())
	view.AltScreen = true

	return view
}

// tooSmall reports a terminal below the floor; before the first size report
// there is nothing to judge.
func (m Model) tooSmall() bool {
	return m.width > 0 && (m.width < minWidth || m.height < minHeight)
}

// screenKind is what fills the terminal; the first that applies wins, in the order screen tries them.
type screenKind int

const (
	screenViews      screenKind = iota // the tabs over a loaded journal
	screenTooSmall                     // the terminal is below the floor
	screenFirstRun                     // there is no journal yet
	screenParseIssue                   // the journal has a located error
	screenLoading                      // the first load has not landed
	screenLoadFailed                   // the tabs, the load's error in place of a view; no view keys
)

// screen is what fills the terminal: the content drawn and who gets the keys both follow it.
func (m Model) screen() screenKind {
	if m.tooSmall() {
		return screenTooSmall
	}

	if m.firstRun() {
		return screenFirstRun
	}

	if m.parseIssue() != nil {
		return screenParseIssue
	}

	if m.loadErr != nil {
		return screenLoadFailed
	}

	if m.ledger == nil {
		return screenLoading
	}

	return screenViews
}

func (m Model) content() string {
	switch m.screen() {
	case screenTooSmall:
		return m.centred(fmt.Sprintf(
			"terminal too small — need ≥%d×%d, have %d×%d", minWidth, minHeight, m.width, m.height))
	case screenFirstRun:
		return m.centred(m.firstRunText())
	case screenParseIssue:
		return m.centred(parseErrorText(*m.parseIssue()))
	case screenLoading:
		return m.centred("loading journal…")
	case screenViews, screenLoadFailed:
	}

	return m.withPreview(m.withPopup(m.views()))
}

// views is the normal screen: tab bar, rule, the active view, status bar.
func (m Model) views() string {
	body := m.body()
	bodyHeight := max(0, m.height-chromeRows)
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)
	rule := theme.Dim.Render(strings.Repeat("─", m.width))

	return strings.Join([]string{m.tabBar(), rule, body, m.statusBar()}, "\n")
}

// body is what the active view shows between the rule and the status bar.
func (m Model) body() string {
	if m.loadErr != nil {
		return m.loadErr.Error()
	}

	return m.activeTab().view(m)
}

// hint is the active view's keys as the status bar shows them.
func (m Model) hint() string {
	keys := m.activeTab().keys
	parts := make([]string, 0, len(keys))
	for _, binding := range keys {
		parts = append(parts, binding.Label()+" "+binding.Action)
	}

	return strings.Join(parts, statusGap)
}

// The status bar's spacing: its segments are joined by statusGap, and statusEdge keeps them off the edges.
const (
	statusGap  = "   "
	statusEdge = " "
)

// statusBar shows the active view's name and key hint, with "? keys" at the right edge.
func (m Model) statusBar() string {
	left := m.statusLeft()
	status := m.statusChecks()
	right := m.indicator() + "? keys" + statusEdge
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(status)-lipgloss.Width(right))

	return theme.StatusBar.Render(left+strings.Repeat(" ", gap)) + status + theme.StatusBar.Render(right)
}

// statusChecks is the check indicator in its style with the gap after it, or nothing before the first run.
func (m Model) statusChecks() string {
	status, style := m.checkStatus()
	if status == "" {
		return ""
	}

	return style.Render(status + statusGap)
}

func (m Model) firstRunText() string {
	text := "No journal found at " + m.journalPath + "\n\ny to create it · n to quit"
	if m.createErr != nil {
		text += "\n\n" + m.createErr.Error()
	}

	return text
}

func (m Model) centred(text string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, text)
}

// indicator is the in-flight spinner, which sits one cell before "? keys", or the stale-snapshot error flag
// with the gap after it; nothing when there is neither.
func (m Model) indicator() string {
	if m.busy() {
		return "⣾ "
	}

	if m.failure != nil {
		return "✗ error" + statusGap
	}

	return ""
}

// tabBar puts the tabs on the left and the journal path, with the
// transaction count once loaded, at the right edge.
func (m Model) tabBar() string {
	info := m.journalPath
	if m.ledger != nil {
		info += fmt.Sprintf(" · %d txns", m.ledger.TransactionCount())
	}
	info += " "
	tabs := m.renderTabs()
	gap := max(0, m.width-lipgloss.Width(tabs)-lipgloss.Width(info))

	return tabs + strings.Repeat(" ", gap) + info
}

// renderTabs draws the tab labels, the active one selected.
func (m Model) renderTabs() string {
	labels := make([]string, tabCount)
	for i, tab := range tabs() {
		label := fmt.Sprintf(" %d %s ", i+1, tab.name)
		if i == m.active {
			labels[i] = theme.Selected.Render(label)
		} else {
			labels[i] = theme.Dim.Render(label)
		}
	}

	return strings.Join(labels, theme.Dim.Render("│"))
}

// issueOf is the located journal issue behind err, if there is one.
func issueOf(err error) *domain.CheckIssue {
	var journalErr domain.JournalError
	if !errors.As(err, &journalErr) {
		return nil
	}

	return &journalErr.Issue
}

func parseErrorText(issue domain.CheckIssue) string {
	return issue.Severity.String() + " " + issue.Check + "  " + issue.Location() + "\n\n" +
		issue.Message + "\n\nEnter to jump to the editor · q to quit"
}

// statusLeft is the left of the status bar: a pending question or a notice, else the view's name,
// its key hint and its cursor position.
func (m Model) statusLeft() string {
	if m.prompt != nil {
		return statusEdge + m.prompt.question + " (y/n)"
	}

	if m.notice != "" {
		return statusEdge + m.notice
	}

	segments := []string{m.activeTab().name}
	if hint := m.hint(); hint != "" {
		segments = append(segments, hint)
	}

	if row, total := m.position(); total > 0 {
		segments = append(segments, fmt.Sprintf("%d/%d", row, total))
	}

	if hidden := m.hiddenPanels(); hidden > 0 {
		segments = append(segments, hiddenNotice(hidden))
	}

	return statusEdge + strings.Join(segments, statusGap)
}

// hiddenNotice says how many panels of the active view did not fit.
func hiddenNotice(hidden int) string {
	if hidden == 1 {
		return "1 panel hidden · widen terminal"
	}

	return fmt.Sprintf("%d panels hidden · widen terminal", hidden)
}
