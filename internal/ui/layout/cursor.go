package layout

import "slices"

// Cursor is the selected row of a scrolling list and the first row the list shows. A view scrolled without a
// selected row (Scroll) keeps only the first row shown.
type Cursor struct {
	Row, Offset int
}

// Move applies a cursor key to a list of count rows that shows page rows at once: up, down, home, end, pgup
// or pgdown. The cursor stops at both ends; any other key leaves it where it is.
func (c Cursor) Move(key string, count, page int) Cursor {
	switch key {
	case KeyUp:
		return c.To(c.Row-1, count, page)
	case KeyDown:
		return c.To(c.Row+1, count, page)
	case KeyHome:
		return c.To(0, count, page)
	case KeyEnd:
		return c.To(count, count, page) // To clamps to the last row
	case KeyPgUp:
		return c.To(c.Row-page, count, page)
	case KeyPgDown:
		return c.To(c.Row+page, count, page)
	}
	return c
}

// Moves reports whether key is one of the cursor keys Move and Scroll act on.
func Moves(key string) bool {
	return slices.Contains([]string{KeyUp, KeyDown, KeyHome, KeyEnd, KeyPgUp, KeyPgDown}, key)
}

// Scroll applies a cursor key to a view with no selected row, the keys moving its first row shown as Move
// moves a selected one: between the top and last, page rows a page.
func (c Cursor) Scroll(key string, last, page int) Cursor {
	return Cursor{Offset: Cursor{Row: c.Offset}.Move(key, last+1, page).Row}
}

// To puts the cursor on row, clamped to the list's count rows, and scrolls to keep it among the page shown.
func (c Cursor) To(row, count, page int) Cursor {
	return Cursor{Row: max(0, min(row, count-1)), Offset: c.Offset}.Scrolled(page)
}

// Scrolled moves the window of page rows just enough to show the cursor's row, as after a resize.
func (c Cursor) Scrolled(page int) Cursor {
	offset := max(0, min(c.Offset, c.Row))
	if c.Row >= offset+page {
		offset = c.Row - page + 1
	}
	return Cursor{Row: c.Row, Offset: offset}
}

// Visible is the page of items the cursor's window shows, from its first row shown on.
func Visible[T any](c Cursor, items []T, page int) []T {
	first := min(c.Offset, len(items))
	return items[first:min(len(items), first+page)]
}
