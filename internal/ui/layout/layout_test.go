package layout

import (
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/ui/theme"
	"github.com/sbradl/konto/internal/ui/uitest"
)

func screen(_, height int) string {
	row := strings.Repeat(".", 10)
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
	if !strings.Contains(got, theme.PanelTitle.Render("ACCOUNTS")) {
		t.Errorf("the title should be cyan, got %q", got)
	}
	got = uitest.Plain(got)

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

func TestOverlayFlushReachesTheLastRow(t *testing.T) {
	got := strings.Split(OverlayFlush(screen(10, 6), "AAAAAAAAAA\nBBBBBBBBBB", 6), "\n")

	want := []string{"..........", "..........", "..........", "..........", "AAAAAAAAAA", "BBBBBBBBBB"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPopupCapacityIsTheBodyUnderTheTitleInHalfTheScreenAtLeastOneLine(t *testing.T) {
	for _, tc := range []struct{ lines, screen, want int }{
		{3, 40, 3}, {30, 40, 19}, {19, 40, 19}, {0, 40, 1}, {5, 3, 1},
	} {
		if got := PopupCapacity(tc.lines, tc.screen); got != tc.want {
			t.Errorf("PopupCapacity(%d, %d) = %d, want %d", tc.lines, tc.screen, got, tc.want)
		}
	}
}

func TestPopupIsTheTitleRuleOverTheBodyFittedToTheWidth(t *testing.T) {
	got := strings.Split(uitest.Plain(Popup("keys", []string{"short", "a line too long"}, 10)), "\n")

	want := []string{"─ keys ───", "short     ", "a line too"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("popup = %q, want %q", got, want)
	}
}
