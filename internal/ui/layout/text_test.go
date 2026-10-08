package layout

import (
	"regexp"
	"testing"
)

var escapes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

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
	got := plain(TitleRule("keys", 12))
	if got != "─ keys ─────" {
		t.Errorf("rule = %q", got)
	}
	if got := plain(TitleRule("überschrift", 3)); got != "─ überschrift " {
		t.Errorf("rule wider than the width = %q, want the title alone", got)
	}
}

func TestPluralCountsTheNoun(t *testing.T) {
	for n, want := range map[int]string{0: "0 rows", 1: "1 row", 2: "2 rows"} {
		if got := Plural(n, "row"); got != want {
			t.Errorf("Plural(%d) = %q, want %q", n, got, want)
		}
	}
}
