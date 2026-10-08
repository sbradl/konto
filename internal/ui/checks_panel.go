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

// checksPanel is the latest check run and the checks panel's cursor over its issues.
type checksPanel struct {
	run    *domain.CheckRun // nil until the first run lands
	cursor layout.Cursor    // the selected issue, and the first one shown
}

// issues are the open issues of the run.
func (p checksPanel) issues() []domain.CheckIssue {
	if p.run == nil {
		return nil
	}
	return p.run.Issues
}

// page is how many issues the panel shows at once on a screen height rows tall: what fits under its title in
// half the screen.
func (p checksPanel) page(height int) int {
	return max(1, layout.PopupHeight(max(1, len(p.issues()))+1, height)-1)
}

// withRun shows a new run, the cursor kept on its row, or on the last issue when the run has fewer.
func (p checksPanel) withRun(run domain.CheckRun, height int) checksPanel {
	next := checksPanel{run: &run, cursor: p.cursor}
	next.cursor = p.cursor.To(p.cursor.Row, len(run.Issues), next.page(height))
	return next
}

// moved applies up or down; any other key only keeps the selected issue in view.
func (p checksPanel) moved(key string, height int) checksPanel {
	next, count, page := p, len(p.issues()), p.page(height)
	switch key {
	case "up":
		next.cursor = p.cursor.To(p.cursor.Row-1, count, page)
	case "down":
		next.cursor = p.cursor.To(p.cursor.Row+1, count, page)
	default:
		next.cursor = p.cursor.Scrolled(page)
	}
	return next
}

// selected is the issue under the cursor; false when there are none.
func (p checksPanel) selected() (domain.CheckIssue, bool) {
	issues := p.issues()
	if len(issues) == 0 {
		return domain.CheckIssue{}, false
	}
	return issues[p.cursor.Row], true
}

// onChecksKey moves the panel's cursor, or jumps to the selected issue in the editor.
func (m Model) onChecksKey(key string) (Model, tea.Cmd) {
	if issue, ok := m.checks.selected(); key == "enter" && ok && issue.File != "" {
		return m, m.openEditor(issue)
	}
	m.checks = m.checks.moved(key, m.height)
	return m, nil
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
	if m.checks.run == nil {
		return []string{"no checks have run yet"}
	}
	if m.checks.run.IsClean() {
		return []string{" " + theme.Ok.Render(fmt.Sprintf("✓ all %d checks pass", m.checks.run.Total))}
	}
	last := min(len(m.checks.run.Issues), m.checks.cursor.Offset+m.checks.page(m.height))
	visible := m.checks.run.Issues[m.checks.cursor.Offset:last]
	rows := make([]string, len(visible))
	for i, issue := range visible {
		rows[i] = m.issueLine(issue)
		if m.checks.cursor.Offset+i == m.checks.cursor.Row {
			rows[i] = theme.Selected.Render(layout.Fit(" "+m.issueText(issue), m.width))
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
	if m.checks.run == nil {
		return "", theme.StatusBar
	}
	if m.checks.run.IsClean() {
		return "● clean   ", theme.StatusOk
	}
	var parts []string
	if n := m.checks.run.Count(domain.SeverityErr); n > 0 {
		parts = append(parts, fmt.Sprintf("%d err", n))
	}
	if n := m.checks.run.Count(domain.SeverityWarn); n > 0 {
		parts = append(parts, fmt.Sprintf("%d warn", n))
	}
	text := "● " + strings.Join(parts, " · ") + "   "
	if m.checks.run.Count(domain.SeverityErr) > 0 {
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
