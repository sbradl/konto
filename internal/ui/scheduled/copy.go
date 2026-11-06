package scheduled

import (
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	copyTitle = "copy a transaction"
	copyHint  = "type to filter · ↑/↓ choose · Enter copies · Esc back"
	noMatch   = "no transaction matches"
)

// copying is the open list of the journal's transactions to copy into a new rule.
type copying struct {
	filter string
	back   int // how far the filter's caret stands before its end
	cursor layout.Cursor
}

// OpenCopy opens the list of transactions a new rule can be copied from.
func (m Model) OpenCopy() Model {
	m.copying = &copying{}

	return m
}

// copyMatches are the transactions whose description holds the filter, case aside, newest first.
func (m Model) copyMatches() []domain.Transaction {
	needle := strings.ToLower(m.copying.filter)
	matches := slices.DeleteFunc(slices.Clone(m.ledger.Transactions), func(transaction domain.Transaction) bool {
		return !strings.Contains(strings.ToLower(transaction.Description), needle)
	})
	slices.SortStableFunc(matches, func(a, b domain.Transaction) int { return b.Date.Compare(a.Date) })

	return matches
}

// copyPage is how many transactions the list shows at once below its filter line.
func (m Model) copyPage() int { return layout.PopupCapacity(len(m.copyMatches())+1, m.height) - 1 }

// copyKey applies a key to the list: Enter opens the new-rule form filled from the transaction under the
// cursor, Esc closes the list, the cursor keys move, anything else edits the filter.
func (m Model) copyKey(key string) Model {
	if key == layout.KeyEsc { // ifs, not a switch: mutation testing sees no coverage on switch cases
		m.copying = nil

		return m
	}

	if key == layout.KeyEnter {
		return m.copyChosen()
	}

	return m.copyBrowse(key)
}

// copyChosen opens the new-rule form filled from the transaction under the cursor, if any matches.
func (m Model) copyChosen() Model {
	matches := m.copyMatches()
	if len(matches) == 0 {
		return m
	}
	chosen := matches[m.copying.cursor.Row]
	m.copying = nil

	return m.open(periodic.InputFrom(chosen), nil)
}

// copyBrowse moves the cursor on a cursor key and edits the filter on any other, back at the top on a change.
func (m Model) copyBrowse(key string) Model {
	next := *m.copying
	if layout.Moves(key) {
		next.cursor = next.cursor.Move(key, len(m.copyMatches()), max(1, m.copyPage()))
	} else {
		text, back, _ := layout.Edit(next.filter, next.back, key)
		if text != next.filter {
			next.cursor = layout.Cursor{}
		}
		next.filter, next.back = text, back
	}
	m.copying = &next

	return m
}

// withCopy draws the open list over the view, bottom-anchored like the form.
func (m Model) withCopy(base string) string {
	if m.copying == nil {
		return base
	}
	lines := []string{" " + layout.WithCaret(m.copying.filter, m.copying.back) + "  " + theme.Dim.Render(copyHint)}

	matches := m.copyMatches()
	if len(matches) == 0 {
		lines = append(lines, " "+theme.Dim.Render(noMatch))
	}
	page := max(1, m.copyPage())
	for i, transaction := range layout.Visible(m.copying.cursor, matches, page) {
		line := copyLine(transaction)
		if m.copying.cursor.Offset+i == m.copying.cursor.Row {
			line = theme.Selected.Render(layout.Fit(line, m.width))
		}
		lines = append(lines, line)
	}

	return layout.OverlayFlush(base, layout.Popup(copyTitle, lines, m.width), m.height)
}

// copyLine is a transaction as the rule copied from it would hold it: date, description, amount and accounts.
func copyLine(transaction domain.Transaction) string {
	input := periodic.InputFrom(transaction)

	return " " + transaction.Date.Format(time.DateOnly) + "  " + transaction.Description + "  " +
		input.Amount + " " + input.Currency + "  " + input.From + " → " + input.To
}
