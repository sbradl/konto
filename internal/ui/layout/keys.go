package layout

import (
	"strings"
	"unicode/utf8"
)

// Key names as the views get them (tea.KeyPressMsg.String()), for the keys more than one view or dialog reacts
// to; a view's own letters are its own.
const (
	KeyEsc     = "esc"
	KeyEnter   = "enter"
	KeyTab     = "tab"
	KeyBackTab = "shift+tab"
	KeyUp      = "up"
	KeyDown    = "down"
	KeyLeft    = "left"
	KeyRight   = "right"
	KeyHome    = "home"
	KeyEnd     = "end"
	KeyPgUp    = "pgup"
	KeyPgDown  = "pgdown"
)

// Binding is a key and what it does, as the status bar and the keys popup hint it.
type Binding struct{ Key, Action string }

// Label is the key as a hint shows it: a named key capitalised ("Enter"), a single character as it is.
func (b Binding) Label() string {
	if utf8.RuneCountInString(b.Key) < 2 {
		return b.Key
	}

	return strings.ToUpper(b.Key[:1]) + b.Key[1:]
}
