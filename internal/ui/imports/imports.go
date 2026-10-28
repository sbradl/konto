// Package imports is the Import view: pick a bank export, see what it holds against the journal.
// It does no I/O; the root runs the actions it asks for and hands the results back.
package imports

import (
	"fmt"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	infoLines    = 6
	infoHeight   = infoLines + layout.PanelChrome
	minPanel     = 4
	rowsShare    = 60 // percent of the room below the info block the row table takes
	columnGap    = 2
	ruleIDWidth  = 4
	targetWidth  = 24
	amountWidth  = 18
	dateWidth    = len("2026-09-30")
	parentEntry  = ".."
	needsARule   = "needs a rule"
	pickerTitle  = "OPEN A CSV FILE"
	emptyHint    = "press n for a new file"
	nothingNew   = "nothing new"
	createHint   = "create account"
	noWatermark  = "no watermark"
	noItems      = "—"
	transferMark = " ⇄"
)

// Action is what the view asks the root to do: list a directory, open a file or write the import. At most
// one is set.
type Action struct {
	Browse string
	Open   string
	Write  bool
}

// Model is the Import view's state: nothing, the file picker, or a staged import.
type Model struct {
	downloadDir string
	picking     bool
	dir         string
	entries     []domain.FileEntry
	listErr     error
	browse      layout.Cursor // the file picker's selected entry
	staged      *csvimport.Staged
	dlg         dialog        // the categorising dialog over the rows
	parent      dialog        // the form waiting while its category picker is open
	table       layout.Cursor // the row table's selected row
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
	next.dlg, next.parent = dialog{}, dialog{}
	next.table = layout.Cursor{}
	if staged.NewAccount && staged.Named == "" {
		next.dlg = dialog{kind: dialogAccount, filter: csvimport.SuggestAccountName(staged.IBAN)}
	}
	return next
}

// WithRefreshed swaps the staged import for the same export read again against a changed journal. The
// cursor stays where it was, as near as the new rows allow; a dialog that points at a row is closed.
func (m Model) WithRefreshed(staged csvimport.Staged) Model {
	next := m
	next.staged = &staged
	if next.dlg.kind != dialogAccount {
		next.dlg, next.parent = dialog{}, dialog{}
	}
	return next.rowsTo(m.table.Row)
}

// Staged is the import being worked on; false when there is none.
func (m Model) Staged() (csvimport.Staged, bool) {
	if m.staged == nil {
		return csvimport.Staged{}, false
	}
	return *m.staged, true
}

// Cleared drops the staged import and shows the empty start state.
func (m Model) Cleared() Model {
	next := m
	next.staged, next.dlg, next.parent = nil, dialog{}, dialog{}
	next.table = layout.Cursor{}
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
	next.browse = layout.Cursor{}
	return next
}

// Position is the 1-based row under the cursor and the number of rows; (0, 0) outside a staged import.
func (m Model) Position() (row, total int) {
	if m.picking || m.staged == nil || len(m.staged.Rows) == 0 {
		return 0, 0
	}
	return m.table.Row + 1, len(m.staged.Rows)
}

// Key applies a key to the view; n, the cursor keys, Enter in the picker and Esc are its own.
func (m Model) Key(key string) (Model, Action) {
	next := m
	next.message = ""
	if next.dlg.kind != dialogNone {
		return next.dialogKey(key), Action{}
	}
	if next.picking {
		return next.pickerKey(key)
	}
	switch key {
	case "n":
		next.picking, next.dir, next.entries, next.listErr = true, m.downloadDir, nil, nil
		next.browse = layout.Cursor{}
		return next, Action{Browse: m.downloadDir}
	case keyEnter:
		return next.openChoice(), Action{}
	case "w":
		return next, Action{Write: next.staged != nil}
	default:
		return next.moveRows(key), Action{}
	}
}

// openChoice opens the categorise popup on the row under the cursor.
func (m Model) openChoice() Model {
	if m.rowCount() > 0 {
		m.dlg = dialog{kind: dialogChoice}
	}
	return m
}

// rowCount is the number of rows the cursor moves over.
func (m Model) rowCount() int {
	if m.staged == nil {
		return 0
	}
	return len(m.staged.Rows)
}

func (m Model) moveRows(key string) Model {
	next := m
	next.table = m.table.Move(key, m.rowCount(), m.rowPage())
	return next
}

func (m Model) rowsTo(row int) Model {
	next := m
	next.table = m.table.To(row, m.rowCount(), m.rowPage())
	return next
}

func (m Model) scrollRows() Model {
	next := m
	next.table = m.table.Scrolled(m.rowPage())
	return next
}

// layoutHeights are the heights of the row table and the skipped panel below the info block; the skipped
// panel takes what the row table leaves.
func (m Model) layoutHeights() (rows, skipped int) {
	room := m.height - infoHeight - m.bannerRows()
	rows = max(minPanel, room*rowsShare/100)
	return rows, max(0, room-rows)
}

// innerWidth is the width of a panel's body.
func (m Model) innerWidth() int { return m.width - layout.PanelChrome }

func (m Model) rowPage() int { rows, _ := m.layoutHeights(); return max(1, rows-layout.PanelChrome) }

func (m Model) bannerRows() int {
	if m.message == "" {
		return 0
	}
	return 1
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
	return m.overlay(m.stagedView())
}

// stagedView is the info block above the row table and the skipped list.
func (m Model) stagedView() string {
	rowsHeight, skippedHeight := m.layoutHeights()
	rowsPanel := layout.Panel("ROWS", m.width, rowsHeight, m.rowLines())
	skipped := layout.Panel("SKIPPED", m.width, skippedHeight, m.skippedLines(max(0, skippedHeight-layout.PanelChrome)))
	return strings.Join([]string{m.info(), rowsPanel, skipped}, "\n")
}

// info is the file, the account it goes to, the bank format, the watermark and what is left to do.
func (m Model) info() string {
	staged := m.staged
	account := staged.Account.Name
	if staged.NewAccount {
		account = theme.Warn.Render(createHint) + theme.Dim.Render(" for "+staged.IBAN)
		if staged.Named != "" {
			account = staged.Named + theme.Dim.Render(" (new)")
		}
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
		label("watermark", watermark), label("staged", m.stagedSummary()), label("", theme.Bold.Render(summary)),
	}
	return layout.Panel("IMPORT", m.width, infoHeight, strings.Join(lines, "\n"))
}

// stagedSummary lists what the import would add to the journal besides the transactions: the rules and the
// new accounts, and what the categories do to net income.
func (m Model) stagedSummary() string {
	rules := m.staged.StagedRules()
	items := make([]string, 0, len(rules))
	if m.staged.Named != "" {
		items = append(items, "account "+m.staged.Named)
	}
	for _, rule := range rules {
		items = append(items, "rule "+rule.ID)
	}
	for _, name := range m.staged.NewAccounts {
		items = append(items, "account "+name)
	}
	list := noItems
	if len(items) > 0 {
		list = strings.Join(items, ", ")
	}
	return list + theme.Dim.Render(" · net income ") + m.staged.NetIncomeChange().Format()
}

// rowLines are the visible rows of the table, the one under the cursor highlighted.
func (m Model) rowLines() string {
	inner := m.innerWidth()
	last := min(len(m.staged.Rows), m.table.Offset+m.rowPage())
	lines := make([]string, 0, m.rowPage())
	for i := m.table.Offset; i < last; i++ {
		cells := cellsOf(m.staged.Rows[i], inner)
		if i == m.table.Row {
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
		amount: layout.AlignRight(row.Amount.Format(), amountWidth),
	}
	if !row.NeedsCategory() {
		cells.mark, cells.target = "✓", row.Target
		if row.IsTransfer() {
			cells.target += transferMark
		}
		if row.RuleID != "" {
			cells.rule = "#" + row.RuleID
		}
	}
	cells.rule = layout.Fit(cells.rule, ruleIDWidth)
	cells.target = layout.Fit(cells.target, targetWidth)
	room := width - 3 - ruleIDWidth - dateWidth - columnGap - columnGap - targetWidth - amountWidth
	cells.description = layout.Fit(row.Description, max(0, room))
	return cells
}

// highlighted is the row for the cursor: the match mark keeps its colour outside the highlight, as the
// Accounts glyph does, so the state stays readable; the rest is plain on the selection background.
func (c rowCells) highlighted(width int) string {
	markStyle := theme.Ok
	if c.needsCategory {
		markStyle = theme.Warn
	}
	return " " + markStyle.Render(c.mark) + theme.Selected.Render(layout.Fit(c.rest(), max(0, width-2)))
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
	for i := range shown {
		lines = append(lines, skippedLine(&shown[i], m.innerWidth()))
	}
	if hidden := len(skipped) - len(shown); hidden > 0 {
		lines = append(lines, layout.More(hidden))
	}
	return strings.Join(lines, "\n")
}

// skippedLine is a row the journal already has: its date, description and amount, dim, across the width.
func skippedLine(row *csvimport.Row, width int) string {
	description := layout.Fit(row.Description, width-1-dateWidth-columnGap-amountWidth-1)
	amount := layout.AlignRight(row.Amount.Format(), amountWidth)
	line := " " + row.Date.Format(time.DateOnly) + "  " + description + amount
	return theme.Dim.Render(layout.Fit(line, width))
}
