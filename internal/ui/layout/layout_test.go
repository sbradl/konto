package layout

import (
	"regexp"
	"strings"
	"testing"
)

func screen(width, height int) string {
	row := strings.Repeat(".", width)
	rows := make([]string, height)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

func TestOverlayAnchorsThePopupAboveTheLastRow(t *testing.T) {
	got := strings.Split(Overlay(screen(10, 6), "AAAAAAAAAA\nBBBBBBBBBB", 6), "\n")

	want := []string{"..........", "..........", "..........", "AAAAAAAAAA", "BBBBBBBBBB", ".........."}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOverlayKeepsTheBaseSize(t *testing.T) {
	got := strings.Split(Overlay(screen(10, 6), "AAAAAAAAAA", 6), "\n")

	if len(got) != 6 {
		t.Errorf("got %d rows, want 6", len(got))
	}
}

func TestOverlayClampsAPopupTallerThanTheBase(t *testing.T) {
	popup := strings.Repeat("AAAAAAAAAA\n", 9) + "AAAAAAAAAA"

	got := strings.Split(Overlay(screen(10, 4), popup, 4), "\n")

	if len(got) != 4 || got[0] != "AAAAAAAAAA" {
		t.Errorf("got %q, want 4 rows with the popup from the top", got)
	}
}

func TestPopupHeightIsContentCappedAtHalfTheScreen(t *testing.T) {
	for _, tc := range []struct{ content, screen, want int }{
		{3, 40, 3}, {30, 40, 20}, {20, 40, 20}, {5, 3, 1},
	} {
		if got := PopupHeight(tc.content, tc.screen); got != tc.want {
			t.Errorf("PopupHeight(%d, %d) = %d, want %d", tc.content, tc.screen, got, tc.want)
		}
	}
}

func TestColumnsFitTheWidthUpToTheMaximum(t *testing.T) {
	for _, tc := range []struct{ width, minWidth, maxCols, want int }{
		{120, 25, 4, 4}, {100, 25, 4, 4}, {60, 25, 4, 2}, {10, 25, 4, 1}, {120, 25, 2, 2},
	} {
		if got := Columns(tc.width, tc.minWidth, tc.maxCols); got != tc.want {
			t.Errorf("Columns(%d, %d, %d) = %d, want %d", tc.width, tc.minWidth, tc.maxCols, got, tc.want)
		}
	}
}

func TestPanelDrawsARoundedBorderWithTheTitleInsetTwoCells(t *testing.T) {
	got := Panel("ACCOUNTS", 16, 4, "one\nthis line is far too long")
	if !strings.Contains(got, "\x1b[36mACCOUNTS") {
		t.Errorf("the title should be cyan, got %q", got)
	}
	got = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(got, "")

	want := "╭─ ACCOUNTS ───╮\n│one           │\n│this line is f│\n╰──────────────╯"
	if got != want {
		t.Errorf("panel =\n%s\nwant\n%s", got, want)
	}
}

func TestPanelPadsMissingBodyLines(t *testing.T) {
	got := strings.Split(Panel("X", 6, 5, "a"), "\n")

	if len(got) != 5 || got[2] != "│    │" || got[3] != "│    │" {
		t.Errorf("panel = %q, want 5 rows with blank body rows", got)
	}
}
