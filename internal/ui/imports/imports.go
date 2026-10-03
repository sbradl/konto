// Package imports is the Import view: pick a bank export, see what it holds against the journal.
// It does no I/O; the root runs the actions it asks for and hands the results back.
package imports

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	panelChrome  = 2
	infoLines    = 5
	infoHeight   = infoLines + panelChrome
	minPanel     = 4
	rowsShare    = 60 // percent of the room below the info block the row table takes
	columnGap    = 2
	ruleIDWidth  = 4
	targetWidth  = 24
	amountWidth  = 14
	dateWidth    = len("2026-09-30")
	parentEntry  = ".."
	needsARule   = "needs a rule"
	pickerTitle  = "OPEN A CSV FILE"
	emptyHint    = "press n for a new file"
	nothingNew   = "nothing new"
	createHint   = "create account"
	noWatermark  = "no watermark"
	moreTemplate = " … %d more"
)

// Action is what the view asks the root to do: list a directory or open a file. At most one is set.
type Action struct {
	Browse string
	Open   string
}

// Model is the Import view's state: nothing, the file picker, or a staged import.
type Model struct {
	downloadDir string
	picking     bool
	dir         string
	entries     []domain.FileEntry
	listErr     error
	pickCursor  int
	pickOffset  int
	staged      *csvimport.Staged
	cursor      int
	offset      int
	message     string
	width       int
	height      int
}

// New returns an empty Import view whose picker starts in downloadDir.
func New(downloadDir string) Model { return Model{downloadDir: downloadDir} }

// Resize gives the view the content area below the tab bar and above the status bar.
func (m Model) Resize(width, height int) Model {
	next := m
	next.width, next.height = width, height
	return next.scrollRows().scrollPicker()
}

// WithStaged shows an import and closes the picker.
func (m Model) WithStaged(staged csvimport.Staged) Model {
	next := m
	next.staged, next.picking, next.message = &staged, false, ""
	next.cursor, next.offset = 0, 0
	return next
}

// WithMessage tells the user why nothing happened and closes the picker; the next key clears it.
func (m Model) WithMessage(text string) Model {
	next := m
	next.message, next.picking = text, false
	return next
}

// WithListing shows a directory's entries in the picker, or why it cannot be listed.
func (m Model) WithListing(dir string, entries []domain.FileEntry, err error) Model {
	next := m
	next.dir, next.entries, next.listErr = dir, entries, err
	next.pickCursor, next.pickOffset = 0, 0
	return next
}

// Position is the 1-based row under the cursor and the number of rows; (0, 0) outside a staged import.
func (m Model) Position() (row, total int) {
	if m.picking || m.staged == nil || len(m.staged.Rows) == 0 {
		return 0, 0
	}
	return m.cursor + 1, len(m.staged.Rows)
}

// Key applies a key to the view; n, the cursor keys, Enter in the picker and Esc are its own.
func (m Model) Key(key string) (Model, Action) {
	next := m
	next.message = ""
	if next.picking {
		return next.pickerKey(key)
	}
	switch key {
	case "n":
		next.picking, next.dir, next.entries, next.listErr = true, m.downloadDir, nil, nil
		next.pickCursor, next.pickOffset = 0, 0
		return next, Action{Browse: m.downloadDir}
	default:
		return next.moveRows(key), Action{}
	}
}

// rowCount is the number of rows the cursor moves over.
func (m Model) rowCount() int {
	if m.staged == nil {
		return 0
	}
	return len(m.staged.Rows)
}

func (m Model) moveRows(key string) Model {
	switch key {
	case "up":
		return m.rowsTo(m.cursor - 1)
	case "down":
		return m.rowsTo(m.cursor + 1)
	case "home":
		return m.rowsTo(0)
	case "end":
		return m.rowsTo(m.rowCount())
	case "pgup":
		return m.rowsTo(m.cursor - m.rowPage())
	case "pgdown":
		return m.rowsTo(m.cursor + m.rowPage())
	}
	return m
}

func (m Model) rowsTo(row int) Model {
	next := m
	next.cursor = max(0, min(row, m.rowCount()-1))
	return next.scrollRows()
}

func (m Model) scrollRows() Model {
	next := m
	next.offset = max(0, min(m.offset, m.cursor))
	if m.cursor >= next.offset+m.rowPage() {
		next.offset = m.cursor - m.rowPage() + 1
	}
	return next
}

// layoutHeights are the heights of the row table and the skipped panel below the info block.
func (m Model) layoutHeights() (rows, skipped int) {
	room := max(2*minPanel, m.height-infoHeight-m.bannerRows())
	rows = max(minPanel, room*rowsShare/100)
	return rows, max(minPanel, room-rows)
}

func (m Model) rowPage() int { rows, _ := m.layoutHeights(); return max(1, rows-panelChrome) }

func (m Model) bannerRows() int {
	if m.message == "" {
		return 0
	}
	return 1
}

// pickerRows are the picker's entries: the parent folder first, unless the folder is the root.
func (m Model) pickerRows() []domain.FileEntry {
	if filepath.Dir(m.dir) == m.dir {
		return m.entries
	}
	return append([]domain.FileEntry{{Name: parentEntry, IsDir: true}}, m.entries...)
}

func (m Model) pickerPage() int { return max(1, m.height-panelChrome-1) }

func (m Model) pickerKey(key string) (Model, Action) {
	rows := m.pickerRows()
	switch key {
	case "esc":
		m.picking = false
	case "up":
		m = m.pickerTo(m.pickCursor - 1)
	case "down":
		m = m.pickerTo(m.pickCursor + 1)
	case "home":
		m = m.pickerTo(0)
	case "end":
		m = m.pickerTo(len(rows))
	case "pgup":
		m = m.pickerTo(m.pickCursor - m.pickerPage())
	case "pgdown":
		m = m.pickerTo(m.pickCursor + m.pickerPage())
	case "enter":
		if len(rows) > 0 {
			return m, m.activate(rows[m.pickCursor])
		}
	}
	return m, Action{}
}

// activate is what Enter does on an entry: go into a folder, or open a file.
func (m Model) activate(entry domain.FileEntry) Action {
	switch {
	case entry.Name == parentEntry:
		return Action{Browse: filepath.Dir(m.dir)}
	case entry.IsDir:
		return Action{Browse: filepath.Join(m.dir, entry.Name)}
	}
	return Action{Open: filepath.Join(m.dir, entry.Name)}
}

func (m Model) pickerTo(row int) Model {
	next := m
	next.pickCursor = max(0, min(row, len(m.pickerRows())-1))
	return next.scrollPicker()
}

func (m Model) scrollPicker() Model {
	next := m
	next.pickOffset = max(0, min(m.pickOffset, m.pickCursor))
	if m.pickCursor >= next.pickOffset+m.pickerPage() {
		next.pickOffset = m.pickCursor - m.pickerPage() + 1
	}
	return next
}

// View draws the picker, or the staged import, or the empty start state; a message sits above them.
func (m Model) View() string {
	body := m.body()
	if m.message == "" {
		return body
	}
	return " " + theme.Warn.Render(m.message) + "\n" + body
}

func (m Model) body() string {
	height := m.height - m.bannerRows()
	switch {
	case m.picking:
		return m.picker(height)
	case m.staged == nil:
		return layout.Panel("IMPORT", m.width, height, theme.Dim.Render(" "+emptyHint))
	}
	return m.stagedView(height)
}

// picker is the file picker: the directory, then its folders and CSV files with the cursor.
func (m Model) picker(height int) string {
	lines := []string{" " + theme.Bold.Render(m.dir)}
	if m.listErr != nil {
		lines = append(lines, " "+theme.Negative.Render(m.listErr.Error()))
		return layout.Panel(pickerTitle, m.width, height, strings.Join(lines, "\n"))
	}
	rows := m.pickerRows()
	for i := m.pickOffset; i < min(len(rows), m.pickOffset+m.pickerPage()); i++ {
		name := rows[i].Name
		if rows[i].IsDir && name != parentEntry {
			name += "/"
		}
		line := " " + name
		if i == m.pickCursor {
			line = theme.Selected.Render(fit(line, m.width-panelChrome))
		}
		lines = append(lines, line)
	}
	return layout.Panel(pickerTitle, m.width, height, strings.Join(lines, "\n"))
}

// stagedView is the info block above the row table and the skipped list.
func (m Model) stagedView(height int) string {
	rowsHeight, skippedHeight := m.layoutHeights()
	rowsPanel := layout.Panel("ROWS", m.width, rowsHeight, m.rowLines())
	skippedPanelHeight := max(0, height-infoHeight-rowsHeight)
	skipped := layout.Panel("SKIPPED", m.width, skippedPanelHeight, m.skippedLines(skippedHeight-panelChrome))
	return strings.Join([]string{m.info(), rowsPanel, skipped}, "\n")
}

// info is the file, the account it goes to, the bank format, the watermark and what is left to do.
func (m Model) info() string {
	staged := m.staged
	account := staged.Account.Name
	if staged.NewAccount {
		account = theme.Warn.Render(createHint) + theme.Dim.Render(" for "+staged.IBAN)
	}
	watermark := noWatermark
	if !staged.Watermark.IsZero() {
		watermark = staged.Watermark.Format(time.DateOnly)
	}
	summary := nothingNew
	if len(staged.Rows) > 0 {
		summary = fmt.Sprintf("%d need a category", staged.NeedCategory())
	}
	label := func(name, value string) string { return " " + theme.Dim.Render(fmt.Sprintf("%-10s", name)) + value }
	lines := []string{
		label("file", staged.File), label("account", account), label("format", staged.Format),
		label("watermark", watermark), label("", theme.Bold.Render(summary)),
	}
	return layout.Panel("IMPORT", m.width, infoHeight, strings.Join(lines, "\n"))
}

// rowLines are the visible rows of the table, the one under the cursor highlighted.
func (m Model) rowLines() string {
	inner := m.width - panelChrome
	last := min(len(m.staged.Rows), m.offset+m.rowPage())
	lines := make([]string, 0, last-m.offset)
	for i := m.offset; i < last; i++ {
		cells := cellsOf(m.staged.Rows[i], inner)
		if i == m.cursor {
			lines = append(lines, cells.highlighted(inner))
		} else {
			lines = append(lines, cells.styled())
		}
	}
	return strings.Join(lines, "\n")
}

// rowCells are the cells of one row of the table, as text.
type rowCells struct {
	mark, rule, date, description, target, amount string
	needsCategory                                 bool
}

func cellsOf(row csvimport.Row, width int) rowCells {
	cells := rowCells{
		mark: "?", date: row.Date.Format(time.DateOnly), target: needsARule, needsCategory: row.NeedsCategory(),
		amount: alignRight(row.Amount.Format(), amountWidth),
	}
	if !row.NeedsCategory() {
		cells.mark, cells.target = "✓", row.Target
		cells.rule = "#" + row.RuleID
	}
	cells.rule = fit(cells.rule, ruleIDWidth)
	cells.target = fit(cells.target, targetWidth)
	room := width - 3 - ruleIDWidth - dateWidth - columnGap - columnGap - targetWidth - amountWidth
	cells.description = fit(row.Description, max(0, room))
	return cells
}

// highlighted is the row for the cursor: the match mark keeps its colour outside the highlight, as the
// Accounts glyph does, so the state stays readable; the rest is plain on the selection background.
func (c rowCells) highlighted(width int) string {
	markStyle := theme.Ok
	if c.needsCategory {
		markStyle = theme.Warn
	}
	return " " + markStyle.Render(c.mark) + theme.Selected.Render(fit(c.rest(), max(0, width-2)))
}

// rest is everything after the match mark.
func (c rowCells) rest() string {
	return " " + c.rule + c.date + strings.Repeat(" ", columnGap) + c.description +
		strings.Repeat(" ", columnGap) + c.target + c.amount
}

// styled is the row with its match mark green or amber, its category dim and "needs a rule" amber.
func (c rowCells) styled() string {
	markStyle, targetStyle := theme.Ok, theme.Dim
	if c.needsCategory {
		markStyle, targetStyle = theme.Warn, theme.Warn
	}
	return " " + markStyle.Render(c.mark) + " " + c.rule + c.date + strings.Repeat(" ", columnGap) + c.description +
		strings.Repeat(" ", columnGap) + targetStyle.Render(c.target) + c.amount
}

// skippedLines list the rows the journal already has, dim; when they do not fit the last row counts the rest.
func (m Model) skippedLines(capacity int) string {
	skipped := m.staged.Skipped
	shown := skipped
	if len(shown) > capacity {
		shown = shown[:max(0, capacity-1)]
	}
	lines := make([]string, 0, capacity)
	for _, row := range shown {
		description := fit(row.Description, m.width-panelChrome-1-dateWidth-columnGap-amountWidth-1)
		line := " " + row.Date.Format(time.DateOnly) + "  " + description + alignRight(row.Amount.Format(), amountWidth)
		lines = append(lines, theme.Dim.Render(fit(line, m.width-panelChrome)))
	}
	if hidden := len(skipped) - len(shown); hidden > 0 {
		lines = append(lines, theme.Dim.Render(fmt.Sprintf(moreTemplate, hidden)))
	}
	return strings.Join(lines, "\n")
}

func fit(text string, width int) string {
	cut := lipgloss.NewStyle().MaxWidth(max(0, width)).Render(text)
	return cut + strings.Repeat(" ", max(0, width-lipgloss.Width(cut)))
}

func alignRight(text string, width int) string {
	return strings.Repeat(" ", max(0, width-lipgloss.Width(text))) + text
}
