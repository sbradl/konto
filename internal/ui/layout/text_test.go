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

func TestEditHandlesTextBackspaceClearAndOtherKeys(t *testing.T) {
	for _, c := range []struct {
		text, key, want string
		ok              bool
	}{
		{"ab", "c", "abc", true},
		{"ab", "ä", "abä", true},
		{"abä", "backspace", "ab", true},
		{"", "backspace", "", true},
		{"ab", "ctrl+u", "", true},
		{"ab", " ", "ab ", true},
		{"ab", "enter", "ab", false},
		{"ab", "tab", "ab", false},
	} {
		if got, ok := Edit(c.text, c.key); got != c.want || ok != c.ok {
			t.Errorf("Edit(%q, %q) = %q, %v", c.text, c.key, got, ok)
		}
	}
}
