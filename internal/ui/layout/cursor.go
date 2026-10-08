package layout

// Cursor is the selected row of a scrolling list and the first row the list shows.
type Cursor struct {
	Row, Offset int
}

// Move applies a cursor key to a list of count rows that shows page rows at once: up, down, home, end, pgup
// or pgdown. The cursor stops at both ends; any other key leaves it where it is.
func (c Cursor) Move(key string, count, page int) Cursor {
	switch key {
	case "up":
		return c.To(c.Row-1, count, page)
	case "down":
		return c.To(c.Row+1, count, page)
	case "home":
		return c.To(0, count, page)
	case "end":
		return c.To(count, count, page) // To clamps to the last row
	case "pgup":
		return c.To(c.Row-page, count, page)
	case "pgdown":
		return c.To(c.Row+page, count, page)
	}
	return c
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
