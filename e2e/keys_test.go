//go:build e2e

package e2e

import (
	"fmt"
	"strings"
)

// namedKeys maps the key names used in the features to the bytes a legacy
// xterm sends. Legacy encoding keeps the harness independent of the keyboard
// protocol konto negotiates.
var namedKeys = map[string]string{
	"Enter":          "\r",
	"Esc":            "\x1b",
	"Tab":            "\t",
	"Shift+Tab":      "\x1b[Z",
	"Backspace":      "\x7f",
	"Ctrl+Backspace": "\x08",
	"Space":          " ",
	"↑":              "\x1b[A",
	"↓":              "\x1b[B",
	"→":              "\x1b[C",
	"←":              "\x1b[D",
	"Up":             "\x1b[A",
	"Down":           "\x1b[B",
	"Right":          "\x1b[C",
	"Left":           "\x1b[D",
	"Home":           "\x1b[H",
	"End":            "\x1b[F",
	"PgUp":           "\x1b[5~",
	"PgDn":           "\x1b[6~",
	"Insert":         "\x1b[2~",
	"Delete":         "\x1b[3~",
	"F1":             "\x1bOP",
	"F2":             "\x1bOQ",
	"F3":             "\x1bOR",
	"F4":             "\x1bOS",
	"F5":             "\x1b[15~",
	"F6":             "\x1b[17~",
	"F7":             "\x1b[18~",
	"F8":             "\x1b[19~",
	"F9":             "\x1b[20~",
	"F10":            "\x1b[21~",
	"F11":            "\x1b[23~",
	"F12":            "\x1b[24~",
}

// keyBytes encodes one key name from a feature: a named key, a single
// character, or a Ctrl+/Alt+/Shift+ chord on a single character.
func keyBytes(name string) (string, error) {
	if b, ok := namedKeys[name]; ok {
		return b, nil
	}

	if len([]rune(name)) == 1 {
		return name, nil
	}

	modifier, key, ok := strings.Cut(name, "+")
	if !ok || len([]rune(key)) != 1 {
		return "", fmt.Errorf("unknown key %q", name)
	}

	return chordBytes(modifier, key, name)
}

// chordBytes encodes modifier+key, a single-character key; name is the chord as the feature wrote it.
func chordBytes(modifier, key, name string) (string, error) {
	switch modifier {
	case "Ctrl":
		c := strings.ToLower(key)[0]
		if c < 'a' || c > 'z' {
			return "", fmt.Errorf("unsupported Ctrl chord %q", name)
		}

		return string(rune(c & 0x1f)), nil
	case "Alt":
		return "\x1b" + key, nil
	case "Shift":
		return strings.ToUpper(key), nil
	}

	return "", fmt.Errorf("unknown modifier in %q", name)
}
