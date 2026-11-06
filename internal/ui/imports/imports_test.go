package imports

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
	"github.com/sbradl/konto/internal/ui/uitest"
)

var euro = domain.DefaultAmount("EUR").Style

func eur(mantissa int64) domain.MultiAmount {
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}}
}

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)

	return d
}

func staged() csvimport.Staged {
	return csvimport.Staged{
		File: "sept.csv", IBAN: "DE89370400440532013000", Format: "postbank",
		Account: domain.Account{Name: "assets:checking", Source: domain.SourceImport}, Watermark: day("2026-09-26"),
		Rows: []csvimport.Row{
			{Date: day("2026-09-30"), Description: "REWE Markt", Amount: eur(-4217), Target: "expenses:groceries", RuleID: "2"},
			{Date: day("2026-09-29"), Description: "Bäckerei Schmidt", Amount: eur(-650)},
			{Date: day("2026-09-26"), Description: "Kiosk", Amount: eur(-200)},
		},
		Skipped: []csvimport.Row{
			{Date: day("2026-09-26"), Description: "Sparen", Amount: eur(-50000)},
			{Date: day("2026-09-25"), Description: "Acme GmbH", Amount: eur(300000)},
		},
	}
}

func opened() Model { return New("/dl").Resize(180, 40).WithStaged(staged()) }

func lines(m Model) []string { return strings.Split(uitest.Plain(m.View()), "\n") }

func TestAnEmptyImportHintsAtTheNewFileKey(t *testing.T) {
	view := uitest.Plain(New("/dl").Resize(180, 40).View())

	if !strings.Contains(view, "press n for a new file") || strings.Contains(view, "●") {
		t.Errorf("empty Import:\n%s", view)
	}

	if row, total := New("/dl").Position(); row != 0 || total != 0 {
		t.Errorf("an empty Import has no position")
	}
}

func TestNOpensThePickerAtTheDownloadDirectoryAndAsksForItsListing(t *testing.T) {
	m, action := New("/home/me/Downloads").Resize(180, 40).Key("n")

	if action != (Browse{Dir: "/home/me/Downloads"}) {
		t.Errorf("action = %+v, want a listing of the download directory", action)
	}

	if view := uitest.Plain(m.View()); !strings.Contains(view, "/home/me/Downloads") {
		t.Errorf("the picker shows its directory:\n%s", view)
	}
}

func listing() []domain.FileEntry {
	return []domain.FileEntry{{Name: "subdir", IsDir: true}, {Name: "sept.csv"}, {Name: "other.csv"}}
}

func picker(t *testing.T) Model {
	t.Helper()
	m, _ := New("/dl").Resize(180, 40).Key("n")

	return m.WithListing("/dl", listing(), nil)
}

func TestThePickerListsTheFoldersAndFilesItIsGiven(t *testing.T) {
	view := uitest.Plain(picker(t).View())

	for _, want := range []string{"OPEN A CSV FILE", "/dl", "..", "subdir/", "sept.csv", "other.csv"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker lacks %q:\n%s", want, view)
		}
	}
}

func TestEnterOnAFolderBrowsesIntoItAndOnAFileOpensIt(t *testing.T) {
	m := picker(t)

	into, _ := m.Key("down") // past ..

	_, action := into.Key("enter")
	if action != (Browse{Dir: "/dl/subdir"}) {
		t.Errorf("enter on a folder: %+v, want to browse /dl/subdir", action)
	}

	file, _ := m.Key("down")
	file, _ = file.Key("down")

	_, action = file.Key("enter")
	if action != (Open{Path: "/dl/sept.csv"}) {
		t.Errorf("enter on a file: %+v, want to open /dl/sept.csv", action)
	}
}

func TestEnterOnDotDotGoesUpAndTheRootHasNoDotDot(t *testing.T) {
	_, action := picker(t).Key("enter")
	if action != (Browse{Dir: "/"}) {
		t.Errorf("enter on .. from /dl: %+v, want to browse /", action)
	}
	root := New("/").Resize(180, 40)
	root, _ = root.Key("n")

	root = root.WithListing("/", listing(), nil)
	if strings.Contains(uitest.Plain(root.View()), "..") {
		t.Errorf("the root has no parent")
	}
}

func TestAMessageClosesThePickerButNotADialog(t *testing.T) {
	if m := picker(t).WithMessage("no"); m.mode != modeRows {
		t.Errorf("the picker stays open under a message: mode %d", m.mode)
	}

	if m := opened().openChoice().WithMessage("no"); !m.Capturing() {
		t.Error("a message closed the open dialog")
	}
}

func TestEscLeavesThePickerWithNothingStaged(t *testing.T) {
	m, action := picker(t).Key("esc")

	if action != nil || !strings.Contains(uitest.Plain(m.View()), "press n for a new file") {
		t.Errorf("esc should return to the empty Import; action = %+v", action)
	}

	back, _ := opened().Key("n")
	if esc, _ := back.Key("esc"); esc.staged == nil {
		t.Errorf("esc from the picker keeps an import that was staged")
	}
}

func TestThePickerForgetsItsDirectoryEachTime(t *testing.T) {
	m := picker(t)
	m, _ = m.Key("down")
	m, _ = m.Key("enter")
	m, _ = m.Key("esc")

	_, action := m.Key("n")

	if action != (Browse{Dir: "/dl"}) {
		t.Errorf("the picker reopens at %+v, want the download directory", action)
	}
}

func TestAMissingDirectoryIsShownNotACrash(t *testing.T) {
	m, _ := New("/nope").Resize(180, 40).Key("n")
	m = m.WithListing("/nope", nil, errors.New("listing /nope: no such file or directory"))

	if view := uitest.Plain(m.View()); !strings.Contains(view, "no such file or directory") {
		t.Errorf("the failure should be shown:\n%s", view)
	}
}

func TestPickerCursorStopsAtBothEndsAndScrolls(t *testing.T) {
	entries := make([]domain.FileEntry, 0, 60)
	for i := range 60 {
		entries = append(entries, domain.FileEntry{Name: fmt.Sprintf("f%02d.csv", i)})
	}
	m, _ := New("/dl").Resize(100, 20).Key("n")
	m = m.WithListing("/dl", entries, nil)

	for range 70 {
		m, _ = m.Key("down")
	}

	end := uitest.Plain(m.View())
	if !strings.Contains(end, "f59.csv") || strings.Contains(end, "f00.csv") {
		t.Errorf("the cursor at the end scrolls the list:\n%s", end)
	}
	for range 70 {
		m, _ = m.Key("up")
	}

	if !strings.Contains(uitest.Plain(m.View()), "..") {
		t.Errorf("the cursor stops at the top")
	}
}

func TestOpeningAFileStagesItAndClosesThePicker(t *testing.T) {
	view := uitest.Plain(picker(t).WithStaged(staged()).View())

	if strings.Contains(view, "OPEN A CSV FILE") || !strings.Contains(view, "REWE Markt") {
		t.Errorf("the staged import replaces the picker:\n%s", view)
	}
}

func TestTheInfoBlockNamesFileAccountFormatAndWatermark(t *testing.T) {
	view := uitest.Plain(opened().View())

	for _, want := range []string{"file", "sept.csv", "account", "assets:checking", "format", "postbank", "watermark", "2026-09-26"} {
		if !strings.Contains(view, want) {
			t.Errorf("info block lacks %q:\n%s", want, view)
		}
	}
}

func TestAFreshAccountShowsNoWatermark(t *testing.T) {
	fresh := staged()
	fresh.Watermark = time.Time{}

	if view := uitest.Plain(New("/dl").Resize(180, 40).WithStaged(fresh).View()); !strings.Contains(view, "no watermark") {
		t.Errorf("a fresh account has no watermark:\n%s", view)
	}
}

func TestANewAccountOffersToCreateIt(t *testing.T) {
	fresh := staged()
	fresh.NewAccount, fresh.Account = true, domain.Account{}

	view := uitest.Plain(New("/dl").Resize(180, 40).WithStaged(fresh).View())

	if !strings.Contains(view, "create account") || strings.Contains(view, "attach") {
		t.Errorf("a new IBAN offers to create an account:\n%s", view)
	}
}

func TestRowsShowMatchStateRuleIdTargetAndAmount(t *testing.T) {
	got := lines(opened())
	view := strings.Join(got, "\n")

	rewe := rowLine(t, got, "REWE Markt")
	if !strings.Contains(rewe, "✓") || !strings.Contains(rewe, "2026-09-30") || !strings.Contains(rewe, "expenses:groceries") ||
		!strings.Contains(rewe, "-42,17 EUR") || !strings.Contains(rewe, "#2") {
		t.Errorf("matched row = %q", rewe)
	}

	if kiosk := rowLine(t, got, "Kiosk"); !strings.Contains(kiosk, "?") || !strings.Contains(kiosk, "needs a rule") || strings.Contains(kiosk, "#") {
		t.Errorf("unmatched row = %q", kiosk)
	}

	if !strings.Contains(view, "2 need a category") {
		t.Errorf("the summary counts rows needing a category:\n%s", view)
	}
}

func rowLine(t *testing.T, lines []string, text string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, text) {
			return line
		}
	}
	t.Fatalf("no line with %q", text)

	return ""
}

func TestMatchedRowsAreGreenAndTheirTargetDimUnmatchedAreAmber(t *testing.T) {
	selectedLast, _ := opened().Key("end")
	view := selectedLast.View()

	for name, want := range map[string]string{
		"check green": "\x1b[32m✓", "question amber": "\x1b[33m?", "target dim": "\x1b[90mexpenses:groceries", "needs a rule amber": "\x1b[33mneeds a rule",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("%s: want %q in the view", name, want)
		}
	}
}

func TestTheFirstRowIsSelectedAndTheCursorMovesWithinTheRows(t *testing.T) {
	m := opened()
	if row, total := m.Position(); row != 1 || total != 3 {
		t.Errorf("position = %d/%d, want 1/3", row, total)
	}

	if !strings.Contains(m.View(), "100m") {
		t.Errorf("the selected row is highlighted")
	}

	for _, step := range []struct {
		key  string
		want int
	}{{"down", 2}, {"end", 3}, {"down", 3}, {"up", 2}, {"home", 1}, {"up", 1}, {"pgdown", 3}, {"pgup", 1}, {"x", 1}} {
		m, _ = m.Key(step.key)
		if row, _ := m.Position(); row != step.want {
			t.Errorf("after %s: row %d, want %d", step.key, row, step.want)
		}
	}
}

func TestSkippedRowsAreAListNotACountAndDim(t *testing.T) {
	view := opened().View()
	text := uitest.Plain(view)

	_, skipped, found := strings.Cut(text, "SKIPPED")
	if !found {
		t.Fatalf("no SKIPPED panel:\n%s", text)
	}
	for _, want := range []string{"2026-09-26", "Sparen", "-500,00 EUR", "2026-09-25", "Acme GmbH", "3.000,00 EUR"} {
		if !strings.Contains(skipped, want) {
			t.Errorf("skipped list lacks %q:\n%s", want, skipped)
		}
	}

	if !strings.Contains(view, "\x1b[90m") || strings.Contains(strings.SplitN(text, "SKIPPED", 2)[0], "Acme GmbH") {
		t.Errorf("skipped rows are dim and not in the row table")
	}
}

func TestAFileWithNothingNewSaysSoAndEmptyPanelsStayEmpty(t *testing.T) {
	known := staged()
	known.Rows = nil

	view := uitest.Plain(New("/dl").Resize(180, 40).WithStaged(known).View())
	if !strings.Contains(view, "nothing new") {
		t.Errorf("view:\n%s", view)
	}
	nothingSkipped := staged()

	nothingSkipped.Skipped = nil
	if view := uitest.Plain(New("/dl").Resize(180, 40).WithStaged(nothingSkipped).View()); !strings.Contains(view, "SKIPPED") {
		t.Errorf("the skipped panel is empty, not absent:\n%s", view)
	}
}

func TestMessagesReplaceTheStateUntilTheNextKey(t *testing.T) {
	m := opened().WithMessage("the file is not a recognised Postbank export")

	if view := uitest.Plain(m.View()); !strings.Contains(view, "not a recognised Postbank export") {
		t.Errorf("message missing:\n%s", view)
	}

	cleared, _ := m.Key("down")
	if strings.Contains(uitest.Plain(cleared.View()), "not a recognised") {
		t.Errorf("a key clears the message")
	}
}

func TestLongListsOfRowsScrollWithTheCursorAndOverflowOnlyForSkipped(t *testing.T) {
	big := staged()
	for i := range 40 {
		big.Rows = append(big.Rows, csvimport.Row{Date: day("2026-09-20"), Description: fmt.Sprintf("Row %02d", i), Amount: eur(-100)})
		big.Skipped = append(big.Skipped, csvimport.Row{Date: day("2026-09-01"), Description: fmt.Sprintf("Old %02d", i), Amount: eur(-100)})
	}
	m := New("/dl").Resize(100, 30).WithStaged(big)

	for range 30 {
		m, _ = m.Key("down")
	}

	view := uitest.Plain(m.View())
	if !strings.Contains(view, "Row 25") || !strings.Contains(view, "… ") {
		t.Errorf("the selected row stays visible and skipped says how many more:\n%s", view)
	}
}

func TestViewNeverGetsWiderThanTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{100, 27}, {120, 37}, {180, 57}} {
		for _, line := range lines(New("/dl").Resize(size[0], size[1]).WithStaged(staged())) {
			if lipgloss.Width(line) > size[0] {
				t.Errorf("%v: line is %d wide: %q", size, lipgloss.Width(line), line)
			}
		}

		if got := len(lines(New("/dl").Resize(size[0], size[1]).WithStaged(staged()))); got != size[1] {
			t.Errorf("%v: %d rows, want %d", size, got, size[1])
		}
	}
}

func TestKeysWithNothingStagedMoveNothing(t *testing.T) {
	m, action := New("/dl").Resize(180, 40).Key("down")

	if action != nil || m.table.Row != 0 {
		t.Errorf("nothing to move over: %+v", m)
	}
}

func TestThePickerCursorJumpsToTheEndsAndByPages(t *testing.T) {
	entries := make([]domain.FileEntry, 0, 60)
	for i := range 60 {
		entries = append(entries, domain.FileEntry{Name: fmt.Sprintf("f%02d.csv", i)})
	}
	m, _ := New("/dl").Resize(100, 20).Key("n")
	m = m.WithListing("/dl", entries, nil)

	for _, step := range []struct {
		key  string
		want int
	}{{"end", 60}, {"home", 0}, {"pgdown", 17}, {"pgup", 0}} {
		m, _ = m.Key(step.key)
		if m.browse.Row != step.want {
			t.Errorf("after %s the picker cursor is %d, want %d", step.key, m.browse.Row, step.want)
		}
	}
}

func TestEnterInAnEmptyPickerDoesNothing(t *testing.T) {
	m, _ := New("/").Resize(180, 40).Key("n")
	m = m.WithListing("/", nil, nil)

	if _, action := m.Key("enter"); action != nil {
		t.Errorf("action = %+v, want none", action)
	}
}

// spans are the texts that follow each occurrence of the style's opening sequence, up to its reset.
func spans(raw string, style lipgloss.Style) []string {
	opening, _, _ := strings.Cut(style.Render("x"), "x")
	parts := strings.Split(raw, opening)[1:]
	found := make([]string, 0, len(parts))
	for _, part := range parts {
		text, _, _ := strings.Cut(part, "\x1b[m")
		found = append(found, text)
	}

	return found
}

func TestExactlyTheRowUnderTheCursorIsHighlightedAndAlwaysVisible(t *testing.T) {
	m := New("/dl").Resize(100, 30).WithStaged(bigImport(40, 0))

	for step := range 42 {
		m, _ = m.Key("down")

		highlighted := spans(m.View(), theme.Selected)

		want := min(step+1, 42)
		if len(highlighted) != 1 || !strings.Contains(highlighted[0], rowName(want)) ||
			lipgloss.Width(highlighted[0]) != 100-layout.PanelChrome-2 {
			t.Fatalf("after %d downs the highlighted spans are %q", step+1, highlighted)
		}
	}
}

// rowName is the description of the staged row at the index, after the three of the sample file.
func rowName(index int) string {
	if index < 3 {
		return []string{"REWE Markt", "Bäckerei Schmidt", "Kiosk"}[index]
	}

	return fmt.Sprintf("Row %02d", index-3)
}

func TestSkippedRowsAreCutToTheInnerWidth(t *testing.T) {
	m := New("/dl").Resize(100, 30).WithStaged(staged())

	var widths []int
	for row := range strings.SplitSeq(m.View(), "\n") {
		if strings.Contains(row, "Sparen") || strings.Contains(row, "Acme") {
			for _, span := range spans(row, theme.Dim) {
				widths = append(widths, lipgloss.Width(span))
			}
		}
	}

	if !slices.Contains(widths, 100-layout.PanelChrome) {
		t.Errorf("dim span widths = %v, want one of %d", widths, 100-layout.PanelChrome)
	}
}

func TestExactlyTheEntryUnderThePickersCursorIsHighlightedAndAlwaysVisible(t *testing.T) {
	entries := make([]domain.FileEntry, 0, 30)
	for i := range 30 {
		entries = append(entries, domain.FileEntry{Name: fmt.Sprintf("f%02d.csv", i)})
	}
	m, _ := New("/dl").Resize(100, 14).Key("n")
	m = m.WithListing("/dl", entries, nil)

	for step := range 30 {
		m, _ = m.Key("down")

		highlighted := spans(m.View(), theme.Selected)
		if len(highlighted) != 1 || !strings.Contains(highlighted[0], fmt.Sprintf("f%02d.csv", step)) ||
			lipgloss.Width(highlighted[0]) != 100-layout.PanelChrome {
			t.Fatalf("after %d downs the highlighted spans are %q", step+1, highlighted)
		}
	}
}

func TestHighlightedAndSkippedLinesFillExactlyTheWidthTheyAreGiven(t *testing.T) {
	row := staged().Rows[1]
	skipped := staged().Skipped[0]
	cells := cellsOf(row, 100)

	highlighted := spans(cells.highlighted(100), theme.Selected)
	dim := spans(skippedLine(&skipped, 100), theme.Dim)
	entry := spans(listLine("a.csv", 1, 1, 100), theme.Selected)

	if len(highlighted) != 1 || lipgloss.Width(highlighted[0]) != 98 {
		t.Errorf("highlighted row = %q", highlighted)
	}

	if len(dim) != 1 || lipgloss.Width(dim[0]) != 100 {
		t.Errorf("skipped row = %q", dim)
	}

	if len(entry) != 1 || lipgloss.Width(entry[0]) != 100 || listLine("a.csv", 0, 1, 100) != " a.csv" {
		t.Errorf("picker entry = %q", entry)
	}
}

func TestAPanelBodyIsTwoCellsNarrowerThanTheScreen(t *testing.T) {
	if got := New("/dl").Resize(100, 30).innerWidth(); got != 98 {
		t.Errorf("innerWidth = %d, want 98", got)
	}
}

func detailed() Model {
	import1 := staged()
	import1.Rows[0].Type, import1.Rows[0].Payee, import1.Rows[0].Purpose = "SEPA Lastschrift", "REWE Markt", "Einkauf"
	import1.Rows[1].Type, import1.Rows[1].Payee = "Kartenzahlung", "Bäckerei Schmidt"
	import1.Rows[1].Purpose = strings.Repeat("Brötchen ", 200)

	return New("/dl").Resize(60, 40).WithStaged(import1)
}

func TestTheRowPanelShowsWhatARuleCanMatchForTheRowUnderTheCursor(t *testing.T) {
	first := strings.Join(lines(detailed()), "\n")
	for _, want := range []string{"╭─ ROW ", "type      SEPA Lastschrift", "payee     REWE Markt", "purpose   Einkauf"} {
		if !strings.Contains(first, want) {
			t.Errorf("missing %q:\n%s", want, first)
		}
	}

	moved, _ := detailed().Key(layout.KeyDown)
	if view := strings.Join(lines(moved), "\n"); !strings.Contains(view, "type      Kartenzahlung") {
		t.Errorf("the panel follows the cursor:\n%s", view)
	}
}

func TestALongPurposeWrapsOverThePanelsLinesAndStaysInsideIt(t *testing.T) {
	moved, _ := detailed().Key(layout.KeyDown)
	got := lines(moved)

	purpose := slices.IndexFunc(got, func(line string) bool { return strings.Contains(line, "purpose   ") })
	if purpose < 0 || purpose+purposeLines >= len(got) {
		t.Fatalf("no purpose line:\n%s", strings.Join(got, "\n"))
	}
	for i := range purposeLines {
		if !strings.Contains(got[purpose+i], "Brötchen") {
			t.Errorf("purpose line %d = %q", i, got[purpose+i])
		}
	}

	if !strings.HasPrefix(got[purpose+purposeLines], "╰") {
		t.Errorf("the panel closes after the purpose: %q", got[purpose+purposeLines])
	}

	if !strings.Contains(got[purpose+purposeLines-1], "…") {
		t.Errorf("a cut purpose says so: %q", got[purpose+purposeLines-1])
	}
}

func TestAPurposeThatFitsIsNotMarkedCut(t *testing.T) {
	if view := strings.Join(lines(detailed()), "\n"); strings.Contains(view, "…") {
		t.Errorf("nothing was cut:\n%s", view)
	}
}

func TestTheRowPanelGivesWayOnAShortScreen(t *testing.T) {
	if view := strings.Join(lines(New("/dl").Resize(100, detailMinHeight-1).WithStaged(staged())), "\n"); strings.Contains(view, "╭─ ROW ") {
		t.Errorf("no room for the panel:\n%s", view)
	}

	if view := strings.Join(lines(New("/dl").Resize(100, detailMinHeight).WithStaged(staged())), "\n"); !strings.Contains(view, "╭─ ROW ") {
		t.Errorf("room for the panel:\n%s", view)
	}
}

func TestAnImportWithNothingNewHasAnEmptyRowPanel(t *testing.T) {
	empty := staged()
	empty.Rows = nil

	view := strings.Join(lines(New("/dl").Resize(100, 40).WithStaged(empty)), "\n")
	if !strings.Contains(view, "╭─ ROW ") || strings.Contains(view, "purpose") {
		t.Errorf("an empty panel:\n%s", view)
	}
}

// withPurpose is a 60-wide view of an import whose first row has this purpose; 47 cells fit on a line.
func withPurpose(purpose string) []string {
	import1 := staged()
	import1.Rows[0].Purpose = purpose

	return lines(New("/dl").Resize(60, 40).WithStaged(import1))
}

func purposeLine(t *testing.T, got []string, offset int) string {
	t.Helper()

	at := slices.IndexFunc(got, func(line string) bool { return strings.Contains(line, "purpose   ") })
	if at < 0 {
		t.Fatalf("no purpose line:\n%s", strings.Join(got, "\n"))
	}

	return got[at+offset]
}

func TestAPurposeWrapsAtTheEdgeOfThePanel(t *testing.T) {
	const fits = 47

	full := strings.Repeat("x", fits)
	if got := withPurpose(full); !strings.Contains(purposeLine(t, got, 0), full) || strings.Contains(purposeLine(t, got, 1), "x") {
		t.Errorf("a purpose as wide as the line stays on it:\n%s", strings.Join(got, "\n"))
	}

	got := withPurpose(strings.Repeat("x", fits-2) + " yy")
	if !strings.Contains(purposeLine(t, got, 1), "yy") {
		t.Errorf("one cell too wide wraps:\n%s", strings.Join(got, "\n"))
	}
}

func TestAPurposeOfExactlyThePanelsLinesIsNotMarkedCut(t *testing.T) {
	got := strings.Join(withPurpose(strings.Repeat("x", 47*purposeLines)), "\n")
	if strings.Contains(got, cutMark) {
		t.Errorf("nothing was cut:\n%s", got)
	}
}

func TestTheCutMarkFitsOnAFullLine(t *testing.T) {
	got := withPurpose(strings.Repeat("x", 47*purposeLines+1))
	if line := purposeLine(t, got, purposeLines-1); !strings.Contains(line, cutMark) {
		t.Errorf("the mark is on the last line: %q", line)
	}
}

func TestABannerCountsAgainstTheRoomForTheRowPanel(t *testing.T) {
	banner := New("/dl").Resize(100, detailMinHeight).WithStaged(staged()).WithMessage("note")
	if view := strings.Join(lines(banner), "\n"); strings.Contains(view, "╭─ ROW ") {
		t.Errorf("the banner takes the panel's room:\n%s", view)
	}
}

func TestRowAmountsAlwaysShowTwoDecimals(t *testing.T) {
	import1 := staged()
	whole := domain.MultiAmount{{Commodity: "EUR", Mantissa: -6, Style: euro}}
	import1.Rows[0].Amount, import1.Skipped[0].Amount = whole, whole

	got := lines(New("/dl").Resize(100, 30).WithStaged(import1))
	if row := rowLine(t, got, "REWE Markt"); !strings.Contains(row, "-6,00 EUR") {
		t.Errorf("row = %q", row)
	}

	if row := rowLine(t, got, "Sparen"); !strings.Contains(row, "-6,00 EUR") {
		t.Errorf("skipped row = %q", row)
	}
}

func TestCJumpsToTheNextRowThatNeedsACategoryAndWrapsAround(t *testing.T) {
	m, _ := opened().Key("c") // rows: matched, open, open
	if m.table.Row != 1 {
		t.Errorf("from the matched row: %d", m.table.Row)
	}

	if m, _ = m.Key("c"); m.table.Row != 2 {
		t.Errorf("to the next: %d", m.table.Row)
	}

	if m, _ = m.Key("c"); m.table.Row != 1 {
		t.Errorf("wrapping past the matched row: %d", m.table.Row)
	}
}

func TestCStaysPutWhenNoRowNeedsACategory(t *testing.T) {
	done := staged()
	done.Rows = done.Rows[:1]
	done.Rows = append(done.Rows, csvimport.Row{Date: day("2026-09-20"), Description: "B", Amount: eur(-100), Target: "expenses:b"})

	m, _ := New("/dl").Resize(100, 30).WithStaged(done).Key("down")
	if m, _ = m.Key("c"); m.table.Row != 1 || !strings.Contains(m.View(), "no row needs a category") {
		t.Errorf("row = %d:\n%s", m.table.Row, m.View())
	}

	if empty, _ := New("/dl").Resize(100, 30).Key("c"); empty.table.Row != 0 || strings.Contains(empty.View(), "no row needs") {
		t.Errorf("no import: %d\n%s", empty.table.Row, empty.View())
	}

	open, _ := New("/dl").Resize(100, 30).WithStaged(staged()).Key("c")
	if strings.Contains(open.View(), "no row needs") {
		t.Errorf("rows need one:\n%s", open.View())
	}
}

func TestCScrollsTheTargetRowIntoView(t *testing.T) {
	long := bigImport(40, 0)
	for i := range long.Rows {
		long.Rows[i].Target = "expenses:x"
	}
	last := len(long.Rows) - 1
	long.Rows[last].Target = ""

	m, _ := New("/dl").Resize(100, 30).WithStaged(long).Key("c")
	if view := strings.Join(lines(m), "\n"); m.table.Row != last || !strings.Contains(view, long.Rows[last].Description) {
		t.Errorf("row %d not in view:\n%s", m.table.Row, view)
	}
}
