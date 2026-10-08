package layout

import "testing"

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
