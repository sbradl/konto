package scheduled

import (
	"strings"

	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	caret      = "▏"
	focusMark  = "▸"
	labelWidth = 20
	pickerRows = 5
	noValue    = "—"
	formHint   = "Tab next field · ←/→ choose · Enter save · Esc cancel"
)

// withForm draws the open form over the view, bottom-anchored like the other popups.
func (m Model) withForm(base string) string {
	if m.form == nil {
		return base
	}
	title := "new periodic rule"
	if m.form.editing != nil {
		title = "edit periodic rule"
	}
	rows := append([]string{layout.TitleRule(title, m.width)}, m.form.lines()...)
	rows = rows[:min(len(rows), m.height)]
	for i, row := range rows[1:] {
		rows[i+1] = layout.Fit(row, m.width)
	}
	return layout.OverlayFlush(base, strings.Join(rows, "\n"), m.height)
}

// lines are the form's fields, the picker's matches under the focused account field, the refusal and the hint.
func (f *form) lines() []string {
	var lines []string
	for field := range fieldCount {
		if !f.shown(field) {
			continue
		}
		lines = append(lines, f.line(field))
		if field == f.focus && f.isAccount() {
			lines = append(lines, f.pickerLines()...)
		}
	}
	if f.message != "" {
		lines = append(lines, " "+theme.Negative.Render(f.message))
	}
	return append(lines, " "+theme.Dim.Render(formHint))
}

// line is one field: its mark and label, then what it holds.
func (f *form) line(field int) string {
	mark := " "
	if field == f.focus {
		mark = theme.Warn.Render(focusMark)
	}
	return mark + theme.Dim.Render(padRight(fieldLabel(field), labelWidth)) + f.shownValue(field)
}

// shownValue is the field's value: a text with the caret when it has the focus, the choices of a dropdown with
// the chosen one in brackets, an account, or the picker's filter while one is being typed.
func (f *form) shownValue(field int) string {
	focused := field == f.focus
	switch field {
	case fieldFrequency:
		names := make([]string, 0, len(periodic.Frequencies()))
		for _, frequency := range periodic.Frequencies() {
			names = append(names, choice(frequency.String(), frequency == f.input.Frequency))
		}
		return strings.Join(names, "  ")
	case fieldCurrency:
		names := make([]string, 0, len(f.currencies))
		for _, currency := range f.currencies {
			names = append(names, choice(currency, currency == f.input.Currency))
		}
		return strings.Join(names, "  ")
	case fieldFrom, fieldTo:
		text := [2]string{f.input.From, f.input.To}[field-fieldFrom]
		if focused && f.filter != "" {
			return f.filter + caret
		}
		if text == "" {
			text = theme.Dim.Render(noValue)
		}
		return text + map[bool]string{true: caret, false: ""}[focused]
	}
	text := [fieldCount]string{
		f.input.Date, "", f.input.Remaining, f.input.Description, f.input.Amount, "", "", "",
	}[field]
	if focused {
		text += caret
	}
	return text
}

// choice is a dropdown entry, in brackets and highlighted when it is the one the field holds.
func choice(name string, held bool) string { //nolint:revive // the held mark is the entry's own state, not a mode
	if held {
		return theme.Selected.Render("[" + name + "]")
	}
	return name
}

// pickerLines are the accounts the typed filter leaves, a few around the cursor.
func (f *form) pickerLines() []string {
	matches := f.matches()
	if len(matches) == 0 {
		return []string{"   " + theme.Dim.Render("no account matches")}
	}
	first := max(0, f.cursor-pickerRows+1)
	last := min(len(matches), first+pickerRows)
	lines := make([]string, 0, pickerRows)
	for i := first; i < last; i++ {
		text := "   " + matches[i]
		if i == f.cursor {
			text = "  " + theme.Selected.Render(" "+matches[i])
		}
		lines = append(lines, text)
	}
	return lines
}

func padRight(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-len([]rune(text))))
}
