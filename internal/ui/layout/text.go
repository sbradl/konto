package layout

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/ui/theme"
)

// Fit cuts text off at width or pads it with blanks to width. With no room at all the text is left whole,
// for the edge of the panel it is drawn in to cut: a label cut there still says what its figure is.
func Fit(text string, width int) string {
	if width < 1 { // lipgloss reads a max width of zero as no limit anyway; a negative one must not pad
		return text
	}
	cut := lipgloss.NewStyle().MaxWidth(width).Render(text)
	return cut + strings.Repeat(" ", width-lipgloss.Width(cut))
}

// AlignRight pads text on the left to width; text wider than that is left whole.
func AlignRight(text string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(text))) + text
}

// TitleRule is a dim top edge width cells wide with the title inset two cells, for a popup, dialog or form.
func TitleRule(title string, width int) string {
	rule := "─ " + title + " "
	return theme.Dim.Render(rule + strings.Repeat("─", max(0, width-lipgloss.Width(rule))))
}

// More is the dim last row of a list that does not fit: how many rows it leaves out.
func More(hidden int) string { return theme.Dim.Render(" … " + strconv.Itoa(hidden) + " more") }

// Plural is the count with the noun, an s added unless the count is one.
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// Caret is what a text input draws where its caret stands.
const Caret = "▏"

// Edit applies a key to a text input whose caret stands back runes before the end of the text, so that a field
// that has just got the focus (back 0) has it at the end. It returns the text, the caret and false when the key
// is not for the input.
func Edit(text string, back int, key string) (edited string, newBack int, forInput bool) {
	runes := []rune(text)
	at := max(0, len(runes)-back)
	if edit, ok := editKeys()[key]; ok {
		runes, at = edit(runes, at)
		return string(runes), len(runes) - at, true
	}
	if utf8.RuneCountInString(key) == 1 {
		runes = slices.Insert(runes, at, []rune(key)...)
		return string(runes), len(runes) - at - 1, true
	}
	return text, back, false
}

// WithCaret is the text with the caret drawn back runes before its end.
func WithCaret(text string, back int) string {
	runes := []rune(text)
	at := max(0, len(runes)-back)
	return string(runes[:at]) + Caret + string(runes[at:])
}

// editKeys are the keys a text input acts on besides typed characters. A terminal may send Ctrl+Backspace as
// Ctrl+H or Ctrl+W, which is why those delete a word too.
func editKeys() map[string]func(runes []rune, at int) ([]rune, int) {
	wordLeft := func(runes []rune, at int) ([]rune, int) { return runes, wordStart(runes, at) }
	wordRight := func(runes []rune, at int) ([]rune, int) { return runes, wordEnd(runes, at) }
	home := func(runes []rune, _ int) ([]rune, int) { return runes, 0 }
	end := func(runes []rune, _ int) ([]rune, int) { return runes, len(runes) }
	deleteWordBefore := func(runes []rune, at int) ([]rune, int) { return cut(runes, wordStart(runes, at), at) }
	deleteWordAfter := func(runes []rune, at int) ([]rune, int) { return cut(runes, at, wordEnd(runes, at)) }
	return map[string]func(runes []rune, at int) ([]rune, int){
		KeyLeft:  func(runes []rune, at int) ([]rune, int) { return runes, max(0, at-1) },
		KeyRight: func(runes []rune, at int) ([]rune, int) { return runes, min(len(runes), at+1) },
		KeyHome:  home, "ctrl+a": home,
		KeyEnd: end, "ctrl+e": end,
		"ctrl+left": wordLeft, "alt+left": wordLeft,
		"ctrl+right": wordRight, "alt+right": wordRight,
		"backspace":      func(runes []rune, at int) ([]rune, int) { return cut(runes, max(0, at-1), at) },
		"delete":         func(runes []rune, at int) ([]rune, int) { return cut(runes, at, min(len(runes), at+1)) },
		"ctrl+backspace": deleteWordBefore, "ctrl+h": deleteWordBefore, "ctrl+w": deleteWordBefore,
		"alt+backspace": deleteWordBefore,
		"ctrl+delete":   deleteWordAfter, "alt+delete": deleteWordAfter,
		"ctrl+u": func([]rune, int) ([]rune, int) { return nil, 0 },
	}
}

// cut removes the runes from up to to, and leaves the caret where they were.
func cut(runes []rune, from, to int) (left []rune, at int) {
	return slices.Delete(runes, from, to), from
}

// isWord reports a rune of a word: anything else, such as the colon of an account name, separates words.
func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// wordStart is where the word before at begins, past what separates them.
func wordStart(runes []rune, at int) int {
	for at > 0 && !isWord(runes[at-1]) {
		at--
	}
	for at > 0 && isWord(runes[at-1]) {
		at--
	}
	return at
}

// wordEnd is where the word after at ends, past what separates them.
func wordEnd(runes []rune, at int) int {
	for at < len(runes) && !isWord(runes[at]) {
		at++
	}
	for at < len(runes) && isWord(runes[at]) {
		at++
	}
	return at
}
