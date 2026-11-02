package layout

import (
	"slices"
	"testing"
)

func TestCursorKeysMoveAndStopAtBothEnds(t *testing.T) {
	const count, page = 10, 3
	at := func(row, offset int) Cursor { return Cursor{Row: row, Offset: offset} }
	for _, tc := range []struct {
		from Cursor
		key  string
		want Cursor
	}{
		{at(4, 3), "up", at(3, 3)},
		{at(4, 3), "down", at(5, 3)},
		{at(0, 0), "up", at(0, 0)},
		{at(9, 7), "down", at(9, 7)},
		{at(4, 3), "home", at(0, 0)},
		{at(4, 3), "end", at(9, 7)},
		{at(4, 3), "pgup", at(1, 1)},
		{at(4, 3), "pgdown", at(7, 5)},
		{at(4, 3), "x", at(4, 3)},
	} {
		if got := tc.from.Move(tc.key, count, page); got != tc.want {
			t.Errorf("%+v %s = %+v, want %+v", tc.from, tc.key, got, tc.want)
		}
	}
}

func TestCursorInAnEmptyListStaysAtTheTop(t *testing.T) {
	if got := (Cursor{Row: 3, Offset: 2}).Move("down", 0, 5); got != (Cursor{}) {
		t.Errorf("cursor = %+v, want the top", got)
	}
}

func TestCursorScrollsJustEnoughToStayInView(t *testing.T) {
	if got := (Cursor{Row: 6, Offset: 0}).Scrolled(4); got != (Cursor{Row: 6, Offset: 3}) {
		t.Errorf("below the window: %+v", got)
	}
	if got := (Cursor{Row: 2, Offset: 5}).Scrolled(4); got != (Cursor{Row: 2, Offset: 2}) {
		t.Errorf("above the window: %+v", got)
	}
	if got := (Cursor{Row: 4, Offset: 0}).Scrolled(4); got != (Cursor{Row: 4, Offset: 1}) {
		t.Errorf("one row past the window: %+v", got)
	}
	if got := (Cursor{Row: 4, Offset: 2}).Scrolled(4); got != (Cursor{Row: 4, Offset: 2}) {
		t.Errorf("inside the window: %+v", got)
	}
}

func TestScrollMovesTheFirstRowShownBetweenTheTopAndTheLast(t *testing.T) {
	const last, page = 7, 3
	for _, tc := range []struct {
		from int
		key  string
		want int
	}{
		{4, "up", 3},
		{4, "down", 5},
		{0, "up", 0},
		{7, "down", 7},
		{4, "home", 0},
		{4, "end", 7},
		{4, "pgup", 1},
		{4, "pgdown", 7},
		{4, "x", 4},
	} {
		if got := (Cursor{Offset: tc.from}).Scroll(tc.key, last, page); got != (Cursor{Offset: tc.want}) {
			t.Errorf("%d %s = %+v, want offset %d", tc.from, tc.key, got, tc.want)
		}
	}
}

func TestMovesKnowsTheCursorKeys(t *testing.T) {
	for _, key := range []string{"up", "down", "home", "end", "pgup", "pgdown"} {
		if !Moves(key) {
			t.Errorf("%s is a cursor key", key)
		}
	}
	if Moves("a") || Moves("left") {
		t.Error("a and left are no cursor keys")
	}
}

func TestVisibleIsThePageFromTheFirstRowShown(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	for _, tc := range []struct {
		offset, page int
		want         []int
	}{
		{0, 2, []int{0, 1}}, {3, 3, []int{3, 4}}, {9, 2, []int{}},
	} {
		if got := Visible(Cursor{Offset: tc.offset}, items, tc.page); !slices.Equal(got, tc.want) {
			t.Errorf("offset %d page %d = %v, want %v", tc.offset, tc.page, got, tc.want)
		}
	}
}

func TestABindingLabelCapitalisesANamedKey(t *testing.T) {
	for key, want := range map[string]string{"enter": "Enter", "esc": "Esc", "f1": "F1", "n": "n", "?": "?", "1–5": "1–5"} {
		if got := (Binding{Key: key}).Label(); got != want {
			t.Errorf("Label(%q) = %q, want %q", key, got, want)
		}
	}
}
