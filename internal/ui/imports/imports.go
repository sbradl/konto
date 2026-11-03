// Package imports is the Import view: pick a bank export, see what it holds against the journal.
// It does no I/O; the root runs the actions it asks for and hands the results back.
package imports

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

const (
	infoLines    = 6
	infoHeight   = infoLines + layout.PanelChrome
	purposeLines = 3
	detailHeight = 7 // type, payee, the purpose lines and the frame
	// detailMinHeight is the screen the row panel needs, the info block and two minimal panels besides it:
	// with less, the row table keeps the room.
	detailMinHeight = 23
	cutMark         = "…"
	minPanel        = 4
	rowsShare       = 60 // percent of the room below the info block the row table takes
	columnGap       = 2
	ruleIDWidth     = 4
	targetWidth     = 24
	amountWidth     = 18
	dateWidth       = len("2026-09-30")
	parentEntry     = ".."
	needsARule      = "needs a rule"
	pickerTitle     = "OPEN A CSV FILE"
	emptyHint       = "press n for a new file"
	nothingNew      = "nothing new"
	createHint      = "create account"
	noWatermark     = "no watermark"
	noItems         = "—"
	transferMark    = " ⇄"
)

// Action is what the view asks the root to do: Browse, Open or Write; nil asks for nothing.
type Action interface{ isAction() }

// Browse asks for the folder Dir to be listed in the file picker.
type Browse struct{ Dir string }

// Open asks for the export at Path to be read against the journal.
type Open struct{ Path string }

// Write asks for the staged import to be written.
type Write struct{}

func (Browse) isAction() {}
func (Open) isAction()   {}
func (Write) isAction()  {}

// The Import view's own keys.
const (
	keyNewFile = "n"
	keyWrite   = "w"
	keyNext    = "c"
)

// Bindings are the keys Key acts on besides the cursor keys, as the root hints them.
func Bindings() []layout.Binding {
	return []layout.Binding{
		{Key: keyNewFile, Action: "new file"},
		{Key: layout.KeyEnter, Action: "categorise"},
		{Key: keyNext, Action: "next to categorise"},
		{Key: keyWrite, Action: "write"},
	}
}

// Model is the Import view's state: nothing, the file picker, or a staged import with a dialog over it.
type Model struct {
	downloadDir string
	mode        mode
	dir         string
	entries     []domain.FileEntry
	listErr     error
	browse      layout.Cursor // the file picker's selected entry
	staged      *csvimport.Staged
	dlg         dialog        // what the open dialog holds
	waiting     *dialog       // the rule form waiting while its category picker is open; nil when none is
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
	next.staged, next.message = &staged, ""
	next.table = layout.Cursor{}
	if staged.NewAccount && staged.Named == "" {
		return next.show(modeAccount, dialog{name: csvimport.SuggestAccountName(staged.IBAN)})
	}
	return next.show(modeRows, dialog{})
}

// WithRefreshed swaps the staged import for the same export read again against a changed journal. The
// cursor stays where it was, as near as the new rows allow; a dialog that points at a row is closed.
func (m Model) WithRefreshed(staged csvimport.Staged) Model {
	next := m
	next.staged = &staged
	if next.mode != modeAccount {
		next = next.closeDialog()
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
	next.staged, next.table = nil, layout.Cursor{}
	return next.closeDialog()
}

// WithMessage tells the user why nothing happened and closes the picker; the next key clears it.
func (m Model) WithMessage(text string) Model {
	next := m
	next.message = text
	if next.mode == modeFiles {
		return next.show(modeRows, dialog{})
	}
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
	if m.mode == modeFiles || m.staged == nil || len(m.staged.Rows) == 0 {
		return 0, 0
	}
	return m.table.Row + 1, len(m.staged.Rows)
}

// Key applies a key to the view; n, the cursor keys, Enter in the picker and Esc are its own.
func (m Model) Key(key string) (Model, Action) {
	next := m
	next.message = ""
	if next.mode.dialog() {
		return next.dialogKey(key), nil
	}
	if next.mode == modeFiles {
		return next.pickerKey(key)
	}
	switch key {
	case keyNewFile:
		next = next.show(modeFiles, dialog{})
		next.dir, next.entries, next.listErr = m.downloadDir, nil, nil
		next.browse = layout.Cursor{}
		return next, Browse{Dir: m.downloadDir}
	case layout.KeyEnter:
		return next.openChoice(), nil
	case keyWrite:
		if next.staged == nil {
			return next, nil
		}
		return next, Write{}
	case keyNext:
		return next.jumpToCategorise(), nil
	default:
		return next.moveRows(key), nil
	}
}

// openChoice opens the categorise popup on the row under the cursor.
func (m Model) openChoice() Model {
	if m.rowCount() > 0 {
		return m.show(modeChoice, dialog{})
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

// jumpToCategorise moves the cursor to the next row that needs a category, or says that none does.
func (m Model) jumpToCategorise() Model {
	next := m.rowsTo(m.nextToCategorise())
	if next.staged != nil && next.staged.NeedCategory() == 0 {
		next.message = "no row needs a category"
	}
	return next
}

// nextToCategorise is the first row after the cursor that needs a category, from the top again after the last;
// the cursor's own row when no other does.
func (m Model) nextToCategorise() int {
	count := m.rowCount()
	for step := range count {
		if at := (m.table.Row + 1 + step) % count; m.staged.Rows[at].NeedsCategory() {
			return at
		}
	}
	return m.table.Row
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
	room := m.height - infoHeight - m.bannerRows() - m.detailRows()
	rows = max(minPanel, room*rowsShare/100)
	return rows, max(0, room-rows)
}

// innerWidth is the width of a panel's body.
func (m Model) innerWidth() int { return m.width - layout.PanelChrome }

func (m Model) rowPage() int { rows, _ := m.layoutHeights(); return max(1, rows-layout.PanelChrome) }

// detailRows is the height of the row panel on this screen.
func (m Model) detailRows() int {
	if m.height-m.bannerRows() < detailMinHeight {
		return 0
	}
	return detailHeight
}

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
	if m.mode == modeFiles { // ifs, not a switch: mutation testing sees no coverage on switch cases
		return m.picker(height)
	}
	if m.staged == nil {
		return layout.Panel("IMPORT", m.width, height, theme.Dim.Render(" "+emptyHint))
	}
	return m.overlay(m.stagedView())
}

// stagedView is the info block above the row table and the skipped list.
func (m Model) stagedView() string {
	rowsHeight, skippedHeight := m.layoutHeights()
	rowsPanel := layout.Panel("ROWS", m.width, rowsHeight, m.rowLines())
	skipped := layout.Panel("SKIPPED", m.width, skippedHeight, m.skippedLines(max(0, skippedHeight-layout.PanelChrome)))
	panels := []string{m.info(), rowsPanel, skipped}
	if m.detailRows() > 0 { // above the rows, where the bottom-anchored dialogs leave it readable
		panels = []string{m.info(), layout.Panel("ROW", m.width, detailHeight, m.rowDetail()), rowsPanel, skipped}
	}
	return strings.Join(panels, "\n")
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
	lines := []string{
		label("file", staged.File), label("account", account), label("format", staged.Format),
		label("watermark", watermark), label("staged", m.stagedSummary()), label("", theme.Bold.Render(summary)),
	}
	return layout.Panel("IMPORT", m.width, infoHeight, strings.Join(lines, "\n"))
}

// label is a line of a panel: a dim name, then its value.
func label(name, value string) string {
	return " " + theme.Dim.Render(fmt.Sprintf("%-*s", labelWidth, name)) + value
}

// rowDetail is what a rule can match on the row under the cursor; the table has room for its description only.
func (m Model) rowDetail() string {
	if m.rowCount() == 0 {
		return ""
	}
	row := m.staged.Rows[m.table.Row]
	lines := []string{label("type", row.Type), label("payee", row.Payee)}
	for i, line := range wrapped(row.Purpose, max(1, m.innerWidth()-labelWidth-1), purposeLines) {
		name := ""
		if i == 0 {
			name = "purpose"
		}
		lines = append(lines, label(name, line))
	}
	return strings.Join(lines, "\n")
}

// wrapped is text broken to width, at most limit lines, the last marked when more was cut.
// shortcut: the end of a very long purpose stays unseen, add scrolling if a rule ever needs it.
func wrapped(text string, width, limit int) []string {
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(text), "\n")
	if len(lines) <= limit {
		return lines
	}
	lines = lines[:limit]
	last := strings.TrimRight(layout.Fit(lines[limit-1], max(1, width-1)), " ")
	lines[limit-1] = last + cutMark
	return lines
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
