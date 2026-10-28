package imports

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

var euro = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

func eur(mantissa int64) domain.MultiAmount {
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}}
}

var escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

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

func lines(m Model) []string { return strings.Split(plain(m.View()), "\n") }

func TestAnEmptyImportHintsAtTheNewFileKey(t *testing.T) {
	view := plain(New("/dl").Resize(180, 40).View())

	if !strings.Contains(view, "press n for a new file") || strings.Contains(view, "●") {
		t.Errorf("empty Import:\n%s", view)
	}
	if row, total := New("/dl").Position(); row != 0 || total != 0 {
		t.Errorf("an empty Import has no position")
	}
}

func TestNOpensThePickerAtTheDownloadDirectoryAndAsksForItsListing(t *testing.T) {
	m, action := New("/home/me/Downloads").Resize(180, 40).Key("n")

	if action.Browse != "/home/me/Downloads" || action.Open != "" {
		t.Errorf("action = %+v, want a listing of the download directory", action)
	}
	if view := plain(m.View()); !strings.Contains(view, "/home/me/Downloads") {
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
	view := plain(picker(t).View())

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
	if action.Browse != "/dl/subdir" || action.Open != "" {
		t.Errorf("enter on a folder: %+v, want to browse /dl/subdir", action)
	}

	file, _ := m.Key("down")
	file, _ = file.Key("down")
	_, action = file.Key("enter")
	if action.Open != "/dl/sept.csv" || action.Browse != "" {
		t.Errorf("enter on a file: %+v, want to open /dl/sept.csv", action)
	}
}

func TestEnterOnDotDotGoesUpAndTheRootHasNoDotDot(t *testing.T) {
	_, action := picker(t).Key("enter")
	if action.Browse != "/" {
		t.Errorf("enter on .. from /dl: %+v, want to browse /", action)
	}
	root := New("/").Resize(180, 40)
	root, _ = root.Key("n")
	root = root.WithListing("/", listing(), nil)
	if strings.Contains(plain(root.View()), "..") {
		t.Errorf("the root has no parent")
	}
}

func TestEscLeavesThePickerWithNothingStaged(t *testing.T) {
	m, action := picker(t).Key("esc")

	if action != (Action{}) || !strings.Contains(plain(m.View()), "press n for a new file") {
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

	if action.Browse != "/dl" {
		t.Errorf("the picker reopens at %q, want the download directory", action.Browse)
	}
}

func TestAMissingDirectoryIsShownNotACrash(t *testing.T) {
	m, _ := New("/nope").Resize(180, 40).Key("n")
	m = m.WithListing("/nope", nil, errors.New("listing /nope: no such file or directory"))

	if view := plain(m.View()); !strings.Contains(view, "no such file or directory") {
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
	end := plain(m.View())
	if !strings.Contains(end, "f59.csv") || strings.Contains(end, "f00.csv") {
		t.Errorf("the cursor at the end scrolls the list:\n%s", end)
	}
	for range 70 {
		m, _ = m.Key("up")
	}
	if !strings.Contains(plain(m.View()), "..") {
		t.Errorf("the cursor stops at the top")
	}
}

func TestOpeningAFileStagesItAndClosesThePicker(t *testing.T) {
	view := plain(picker(t).WithStaged(staged()).View())

	if strings.Contains(view, "OPEN A CSV FILE") || !strings.Contains(view, "REWE Markt") {
		t.Errorf("the staged import replaces the picker:\n%s", view)
	}
}

func TestTheInfoBlockNamesFileAccountFormatAndWatermark(t *testing.T) {
	view := plain(opened().View())

	for _, want := range []string{"file", "sept.csv", "account", "assets:checking", "format", "postbank", "watermark", "2026-09-26"} {
		if !strings.Contains(view, want) {
			t.Errorf("info block lacks %q:\n%s", want, view)
		}
	}
}

func TestAFreshAccountShowsNoWatermark(t *testing.T) {
	fresh := staged()
	fresh.Watermark = time.Time{}

	if view := plain(New("/dl").Resize(180, 40).WithStaged(fresh).View()); !strings.Contains(view, "no watermark") {
		t.Errorf("a fresh account has no watermark:\n%s", view)
	}
}

func TestANewAccountOffersToCreateIt(t *testing.T) {
	fresh := staged()
	fresh.NewAccount, fresh.Account = true, domain.Account{}

	view := plain(New("/dl").Resize(180, 40).WithStaged(fresh).View())

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
	text := plain(view)

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

	view := plain(New("/dl").Resize(180, 40).WithStaged(known).View())
	if !strings.Contains(view, "nothing new") {
		t.Errorf("view:\n%s", view)
	}
	nothingSkipped := staged()
	nothingSkipped.Skipped = nil
	if view := plain(New("/dl").Resize(180, 40).WithStaged(nothingSkipped).View()); !strings.Contains(view, "SKIPPED") {
		t.Errorf("the skipped panel is empty, not absent:\n%s", view)
	}
}

func TestMessagesReplaceTheStateUntilTheNextKey(t *testing.T) {
	m := opened().WithMessage("the file is not a recognised Postbank export")

	if view := plain(m.View()); !strings.Contains(view, "not a recognised Postbank export") {
		t.Errorf("message missing:\n%s", view)
	}
	cleared, _ := m.Key("down")
	if strings.Contains(plain(cleared.View()), "not a recognised") {
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
	view := plain(m.View())
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

	if action != (Action{}) || m.table.Row != 0 {
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

	if _, action := m.Key("enter"); action != (Action{}) {
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
