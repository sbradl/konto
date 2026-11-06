package layout

import (
	"testing"

	"github.com/sbradl/konto/internal/ui/uitest"
)

func TestFitPadsOrCutsToTheWidthAndLeavesTextWholeWithNoRoom(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width int
		want  string
	}{
		{"abc", 5, "abc  "}, {"abcdef", 4, "abcd"}, {"abc", 3, "abc"}, {"abc", 1, "a"}, {"abc", 0, "abc"}, {"abc", -2, "abc"},
	} {
		if got := Fit(tc.text, tc.width); got != tc.want {
			t.Errorf("Fit(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
		}
	}
}

func TestAlignRightPadsOnTheLeftAndNeverCuts(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width int
		want  string
	}{
		{"12", 5, "   12"}, {"12345", 3, "12345"}, {"12", -1, "12"},
	} {
		if got := AlignRight(tc.text, tc.width); got != tc.want {
			t.Errorf("AlignRight(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
		}
	}
}

func TestTitleRuleInsetsTheTitleAndRunsToTheWidth(t *testing.T) {
	got := uitest.Plain(TitleRule("keys", 12))
	if got != "─ keys ─────" {
		t.Errorf("rule = %q", got)
	}

	if got := uitest.Plain(TitleRule("überschrift", 3)); got != "─ überschrift " {
		t.Errorf("rule wider than the width = %q, want the title alone", got)
	}
}

func TestMoreSaysHowManyRowsAreLeftOut(t *testing.T) {
	if got := uitest.Plain(More(3)); got != " … 3 more" {
		t.Errorf("More(3) = %q", got)
	}
}

func TestPluralCountsTheNoun(t *testing.T) {
	for n, want := range map[int]string{0: "0 rows", 1: "1 row", 2: "2 rows"} {
		if got := Plural(n, "row"); got != want {
			t.Errorf("Plural(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestEditAppliesAKeyAtTheCaret(t *testing.T) {
	for _, c := range []struct {
		text     string
		back     int
		key      string
		want     string
		wantBack int
		ok       bool
	}{
		// typing and deleting, at the end and inside
		{"ab", 0, "c", "abc", 0, true},
		{"ab", 0, "ä", "abä", 0, true},
		{"ab", 0, " ", "ab ", 0, true},
		{"ab", 1, "x", "axb", 1, true},
		{"abä", 0, "backspace", "ab", 0, true},
		{"", 0, "backspace", "", 0, true},
		{"abc", 1, "backspace", "ac", 1, true},
		{"abc", 3, "backspace", "abc", 3, true},
		{"abc", 1, "delete", "ab", 0, true},
		{"abc", 2, "delete", "ac", 1, true},
		{"abc", 0, "delete", "abc", 0, true},
		{"ab", 1, "ctrl+u", "", 0, true},
		// moving the caret
		{"abc", 0, "left", "abc", 1, true},
		{"abc", 3, "left", "abc", 3, true},
		{"abc", 2, "right", "abc", 1, true},
		{"abc", 0, "right", "abc", 0, true},
		{"abc", 0, "home", "abc", 3, true},
		{"abc", 2, "end", "abc", 0, true},
		{"abc", 0, "ctrl+a", "abc", 3, true},
		{"abc", 2, "ctrl+e", "abc", 0, true},
		// words: letters and digits, whatever stands between them
		{"REWE Markt 12", 0, "ctrl+left", "REWE Markt 12", 2, true},
		{"REWE Markt 12", 2, "ctrl+left", "REWE Markt 12", 8, true},
		{"REWE Markt 12", 2, "alt+left", "REWE Markt 12", 8, true},
		{"REWE Markt 12", 13, "ctrl+left", "REWE Markt 12", 13, true},
		{"REWE Markt 12", 13, "ctrl+right", "REWE Markt 12", 9, true},
		{"REWE Markt 12", 9, "alt+right", "REWE Markt 12", 3, true},
		{"REWE Markt 12", 0, "ctrl+right", "REWE Markt 12", 0, true},
		{"expenses:food", 0, "ctrl+h", "expenses:", 0, true},
		{"expenses: ", 0, "ctrl+backspace", "", 0, true},
		{"REWE Markt", 0, "ctrl+w", "REWE ", 0, true},
		{"REWE Markt", 0, "alt+backspace", "REWE ", 0, true},
		{"REWE Markt", 3, "ctrl+h", "REWE rkt", 3, true},
		{"REWE Markt", 10, "ctrl+h", "REWE Markt", 10, true},
		{"REWE Markt", 5, "ctrl+delete", "REWE ", 0, true},
		{"REWE Markt", 10, "alt+delete", " Markt", 6, true},
		// a caret past the start of a shorter text stands at the start
		{"ab", 9, "x", "xab", 2, true},
		// not for the input
		{"ab", 1, "enter", "ab", 1, false},
		{"ab", 1, "tab", "ab", 1, false},
	} {
		got, back, ok := Edit(c.text, c.back, c.key)
		if got != c.want || back != c.wantBack || ok != c.ok {
			t.Errorf("Edit(%q, %d, %q) = %q, %d, %v; want %q, %d, %v", c.text, c.back, c.key, got, back, ok, c.want, c.wantBack, c.ok)
		}
	}
}

func TestWithCaretDrawsTheCaretWhereItStands(t *testing.T) {
	for _, c := range []struct {
		text string
		back int
		want string
	}{
		{"", 0, Caret},
		{"abä", 0, "abä" + Caret},
		{"abä", 1, "ab" + Caret + "ä"},
		{"ab", 2, Caret + "ab"},
		{"ab", 9, Caret + "ab"},
	} {
		if got := WithCaret(c.text, c.back); got != c.want {
			t.Errorf("WithCaret(%q, %d) = %q, want %q", c.text, c.back, got, c.want)
		}
	}
}
