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
	formHint   = "Tab/↑/↓ field · ←/→ choose · Enter save · Esc cancel"
)

// withForm draws the open form over the view, bottom-anchored like the other popups but, unlike them, not
// capped at half the screen: every field stays in view.
func (m Model) withForm(base string) string {
	if m.form == nil {
		return base
	}
	title := "new periodic rule"
	if m.form.editing != nil {
		title = "edit periodic rule"
	}
	return layout.OverlayFlush(base, layout.Popup(title, m.form.lines(), m.width), m.height) // cut at the screen
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
	return mark + theme.Dim.Render(padRight(fields()[field].label, labelWidth)) + f.shownValue(field)
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
		text := f.text(field)
		if focused && f.filter != "" {
			return f.filter + caret
		}
		if text == "" {
			text = theme.Dim.Render(noValue)
		}
		return text + map[bool]string{true: caret, false: ""}[focused]
	}
	text := f.text(field)
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
	lines := make([]string, 0, pickerRows)
	for i, match := range layout.Visible(f.cursor, matches, pickerRows) {
		text := "   " + match
		if f.cursor.Offset+i == f.cursor.Row {
			text = "  " + theme.Selected.Render(" "+match)
		}
		lines = append(lines, text)
	}
	return lines
}

func padRight(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-len([]rune(text))))
}
