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

func (m Model) content() string {
	if m.tooSmall() {
		return m.centred(fmt.Sprintf(
			"terminal too small — need ≥%d×%d, have %d×%d", minWidth, minHeight, m.width, m.height))
	}
	if m.firstRun {
		return m.centred(m.firstRunText())
	}
	if m.parseIssue != nil {
		return m.centred(parseErrorText(*m.parseIssue))
	}
	if m.ledger == nil {
		return m.centred("loading journal…")
	}
	return m.views()
}

// views is the normal screen: tab bar, rule, the active view, status bar.
func (m Model) views() string {
	body := ""
	if m.loadErr != nil {
		body = m.loadErr.Error()
	}
	bodyHeight := max(0, m.height-chromeRows)
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)
	rule := theme.Dim.Render(strings.Repeat("─", m.width))
	return strings.Join([]string{m.tabBar(), rule, body, m.statusBar()}, "\n")
}

// statusBar shows the active view's name.
func (m Model) statusBar() string {
	return " " + tabNames()[m.active]
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
	names := tabNames()
	labels := make([]string, len(names))
	for i, name := range names {
		label := fmt.Sprintf(" %d %s ", i+1, name)
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
