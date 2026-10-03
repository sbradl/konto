package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

// issues are the open issues of the latest check run.
func (m Model) issues() []domain.CheckIssue {
	if m.run == nil {
		return nil
	}
	return m.run.Issues
}

// checkPage is how many issues the panel shows at once: what fits under its title in half the screen.
func (m Model) checkPage() int {
	return max(1, layout.PopupHeight(max(1, len(m.issues()))+1, m.height)-1)
}

// onChecksKey moves the panel's cursor, or jumps to the selected issue in the editor.
func (m Model) onChecksKey(key string) (Model, tea.Cmd) {
	issues := m.issues()
	switch key {
	case "up":
		m.checkCursor = max(0, m.checkCursor-1)
	case "down":
		m.checkCursor = max(0, min(m.checkCursor+1, len(issues)-1))
	case "enter":
		if len(issues) > 0 && issues[m.checkCursor].File != "" {
			return m, m.openEditor(issues[m.checkCursor])
		}
	}
	return m.scrollChecks(), nil
}

// scrollChecks keeps the selected issue inside the visible window.
func (m Model) scrollChecks() Model {
	m.checkOffset = max(0, min(m.checkOffset, m.checkCursor))
	if m.checkCursor >= m.checkOffset+m.checkPage() {
		m.checkOffset = m.checkCursor - m.checkPage() + 1
	}
	return m
}

// checksBody is the panel's rows: what went wrong with the last reload, if anything, then the
// issues, one clean line, or a note that nothing ran.
func (m Model) checksBody() []string {
	return append(m.failureRows(), m.runRows()...)
}

// failureRows are the lines of the error that made the last reload fail, one row per line.
func (m Model) failureRows() []string {
	if m.refreshErr == nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(m.refreshErr.Error()), "\n")
	lines[0] = "✗ " + lines[0]
	for i, line := range lines {
		lines[i] = " " + theme.Negative.Render(line)
	}
	return lines
}

// runRows are the rows of the latest check run.
func (m Model) runRows() []string {
	if m.run == nil {
		return []string{"no checks have run yet"}
	}
	if m.run.IsClean() {
		return []string{" " + theme.Ok.Render(fmt.Sprintf("✓ all %d checks pass", m.run.Total))}
	}
	last := min(len(m.run.Issues), m.checkOffset+m.checkPage())
	visible := m.run.Issues[m.checkOffset:last]
	rows := make([]string, len(visible))
	for i, issue := range visible {
		rows[i] = m.issueLine(issue)
		if m.checkOffset+i == m.checkCursor {
			rows[i] = theme.Selected.Render(fit(" "+m.issueText(issue), m.width))
		}
	}
	return rows
}

// issueText is an issue without styling: severity, check, location when there is one, message.
func (m Model) issueText(issue domain.CheckIssue) string {
	text := issue.Severity.String() + "  " + issue.Check + "  "
	if issue.File != "" {
		text += fmt.Sprintf("%s:%d  ", m.shortPath(issue.File), issue.Line)
	}
	return text + issue.Message
}

// issueLine is an unselected issue, its severity coloured.
func (m Model) issueLine(issue domain.CheckIssue) string {
	style := theme.Warn
	if issue.Severity == domain.SeverityErr {
		style = theme.Negative
	}
	severity := issue.Severity.String()
	return " " + style.Render(severity) + strings.TrimPrefix(m.issueText(issue), severity)
}

// checkStatus is the check indicator for the status bar, with the style it is drawn in.
// Nothing shows until the first run lands.
func (m Model) checkStatus() (string, lipgloss.Style) {
	if m.run == nil {
		return "", theme.StatusBar
	}
	if m.run.IsClean() {
		return "● clean   ", theme.StatusOk
	}
	var parts []string
	if n := m.run.Count(domain.SeverityErr); n > 0 {
		parts = append(parts, fmt.Sprintf("%d err", n))
	}
	if n := m.run.Count(domain.SeverityWarn); n > 0 {
		parts = append(parts, fmt.Sprintf("%d warn", n))
	}
	text := "● " + strings.Join(parts, " · ") + "   "
	if m.run.Count(domain.SeverityErr) > 0 {
		return text, theme.StatusErr
	}
	return text, theme.StatusWarn
}

// shortPath is file relative to the journal's directory when it lies inside it, else as it is.
func (m Model) shortPath(file string) string {
	if rel, err := filepath.Rel(filepath.Dir(m.journalPath), file); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return file
}
