package imports

import (
	"fmt"
	"strings"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	focusMark     = "▸"
	createLabel   = "+ create"
	transferLabel = "(transfer)"
	noCategory    = "—"
	labelWidth    = 10
	pickerHint    = "type to find or create · Enter picks · Esc back"
)

// overlay draws the open dialog over the staged import, bottom-anchored like the keys popup.
func (m Model) overlay(base string) string {
	if !m.mode.dialog() {
		return base
	}
	title, body := m.dialogBody()
	capacity := layout.PopupCapacity(len(body), m.height)
	shown := layout.Cursor{Row: m.selectedLine()}.Scrolled(capacity) // the line the cursor marks stays in view

	return layout.OverlayFlush(base, layout.Popup(title, layout.Visible(shown, body, capacity), m.width), m.height)
}

// selectedLine is the index in the dialog body of the entry under the list cursor.
func (m Model) selectedLine() int {
	if m.mode == modeCategory || m.mode == modeRuleList {
		return listHeader + m.dlg.cursor.Row
	}

	return 0
}

// listHeader is the filter line above the entries of a list.
const listHeader = 1

// dialogBody is the title and lines of the open dialog.
func (m Model) dialogBody() (title string, body []string) {
	switch m.mode { // exhaustive: a new dialog without its body fails the lint
	case modeChoice:
		return "categorise", m.choiceLines()
	case modeRuleForm:
		if m.dlg.fallback {
			return "fallback rule", m.formLines()
		}

		return "new rule", m.formLines()
	case modeRuleList:
		return "add to rule", m.ruleListLines()
	case modeAddForm:
		return "widen rule " + m.dlg.ruleID, m.addFormLines()
	case modeCategory:
		return "category", m.pickerLines()
	case modeAccount:
		return "create account", m.accountLines()
	case modeRows, modeFiles:
	}

	return "", nil
}

// listLine is one entry of a list, highlighted when it is the one under the cursor.
func listLine(text string, index, cursor, width int) string {
	line := " " + text
	if index == cursor {
		return theme.Selected.Render(layout.Fit(line, width))
	}

	return line
}

func (m Model) choiceLines() []string {
	options := m.choices()
	lines := make([]string, 0, len(options))
	for i, option := range options {
		lines = append(lines, listLine(option, i, m.dlg.cursor.Row, m.width))
	}

	return lines
}

// field is a labelled text input line; the one whose number is the focus shows the caret.
func (d dialog) field(label, text string, own int) string {
	mark := " "
	if d.focus == own {
		mark, text = theme.Warn.Render(focusMark), layout.WithCaret(text, d.back)
	}

	return mark + theme.Dim.Render(fmt.Sprintf("%-*s", labelWidth, label)) + text
}

func (m Model) formLines() []string {
	category := m.dlg.category.Name
	if category == "" {
		category = noCategory
	}
	rule := m.currentRule()
	lines := []string{
		m.dlg.field("pattern", m.dlg.draft.Pattern, fieldPattern),
		m.dlg.field("purpose", m.dlg.draft.Purpose, fieldPurpose),
		m.dlg.field("category", category, -1),
		" " + m.impactLine(rule),
	}
	lines = append(lines, ruleTextLines(rule)...)

	return append(lines, m.refusalLines()...)
}

// ruleTextLines are the rule as the journal would hold it, dim and indented.
func ruleTextLines(rule csvimport.Rule) []string {
	var lines []string
	for line := range strings.SplitSeq(rule.Text(), "\n") {
		lines = append(lines, "   "+theme.Dim.Render(line))
	}

	return lines
}

// refusalLines is the refusal of the last confirm, if any.
func (m Model) refusalLines() []string {
	if m.dlg.refusal == "" {
		return nil
	}

	return []string{" " + theme.Negative.Render("refused: "+m.dlg.refusal)}
}

// impactLine says how many rows of this file the rule matches, and whether it would take rows from others.
func (m Model) impactLine(rule csvimport.Rule) string {
	if m.dlg.draft.Pattern == "" && m.dlg.draft.Purpose == "" {
		return theme.Dim.Render("enter a pattern")
	}
	impact := m.staged.Impact(rule)

	line := "matches " + layout.Plural(impact.Matches, "row") + " in this file"
	if impact.Takes == 0 {
		return line
	}
	taken := "would take " + layout.Plural(impact.Takes, "row") + " from rule " + strings.Join(impact.From, ", ")

	return line + " · " + theme.Negative.Render(taken+" — saving is blocked")
}

func (m Model) ruleListLines() []string {
	rules := m.ruleChoices()
	lines := append(make([]string, 0, 1+len(rules)), m.dlg.field("filter", m.dlg.filter, 0))
	for i, rule := range rules {
		text := fmt.Sprintf("#%-3s %-24s %s", rule.ID, rule.Category, ruleSummary(rule))
		lines = append(lines, listLine(text, i, m.dlg.cursor.Row, m.width))
	}

	return lines
}

// ruleSummary is what a rule matches, in one line.
func ruleSummary(rule csvimport.Rule) string {
	parts := make([]string, 0, len(rule.Alternatives))
	for _, alternative := range rule.Alternatives {
		conditions := make([]string, 0, len(alternative))
		for _, condition := range alternative {
			conditions = append(conditions, condition.Pattern)
		}
		parts = append(parts, strings.Join(conditions, " & "))
	}

	return strings.Join(parts, " | ")
}

func (m Model) addFormLines() []string {
	widened, _ := m.staged.Widened(m.dlg.ruleID, m.dlg.draft.Alternative())
	lines := []string{
		m.dlg.field(m.dlg.draft.Field, m.dlg.draft.Pattern, fieldPattern),
		m.dlg.field("purpose", m.dlg.draft.Purpose, fieldPurpose),
		" " + m.impactLine(widened),
	}
	lines = append(lines, ruleTextLines(widened)...)

	return append(lines, m.refusalLines()...)
}

func (m Model) pickerLines() []string {
	lines := []string{m.dlg.field("category", m.dlg.filter, 0) + "  " + theme.Dim.Render(pickerHint)}
	candidates, create := m.pickerEntries()
	for i, candidate := range candidates {
		text := candidate.Name
		if candidate.Transfer {
			text += "  " + transferLabel
		}
		lines = append(lines, listLine(text, i, m.dlg.cursor.Row, m.width))
	}

	if create {
		row := m.staged.Rows[m.table.Row]
		text := createLabel + " " + csvimport.SuggestName(row, m.dlg.filter)
		lines = append(lines, listLine(strings.TrimSpace(text), len(candidates), m.dlg.cursor.Row, m.width))
	}

	return append(lines, m.refusalLines()...)
}

// accountLines is the name input for the account a new IBAN stands for.
func (m Model) accountLines() []string {
	return append([]string{
		m.dlg.field("name", m.dlg.name, 0),
		" " + theme.Dim.Render("for "+m.staged.IBAN+" · Enter creates · Esc abandons"),
	}, m.refusalLines()...)
}
