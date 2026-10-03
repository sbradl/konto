package dashboard

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
)

var euro = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

func eur(mantissa int64) domain.MultiAmount {
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}}
}

func accounts() []domain.Account {
	return []domain.Account{
		{Name: "equity:opening-balances", Balance: eur(-1000000)},
		{Name: "liabilities:creditcard", Source: domain.SourceImport, Balance: eur(-14000)},
		{Name: "assets:old-account", Source: domain.SourceImport, Closed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Balance: eur(0)},
		{Name: "assets:transfers", Source: domain.SourceClearing, Balance: eur(0)},
		{Name: "assets:savings", Source: domain.SourceImport, Balance: eur(1350000)},
		{Name: "assets:checking", Source: domain.SourceImport, Balance: eur(1324000)},
		{Name: "assets:cash", Source: domain.SourceManual, Balance: eur(5000)},
		{Name: "assets:odd", Balance: eur(100)},
		{Name: "expenses:rent", Balance: eur(810000)},
	}
}

func loaded(height int) Model {
	return New().Resize(180, height).WithAccounts(accounts())
}

var escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

func lines(m Model) []string { return strings.Split(plain(m.View()), "\n") }

func TestAccountsListsAssetsAndLiabilitiesOpenFirstThenClosedEachByName(t *testing.T) {
	got := lines(loaded(20))

	order := []string{"assets:cash", "assets:checking", "assets:odd", "assets:savings", "assets:transfers", "liabilities:creditcard", "assets:old-account"}
	for i, name := range order {
		if !strings.Contains(got[i+1], name) {
			t.Errorf("row %d = %q, want it to hold %q", i, got[i+1], name)
		}
	}
	if joined := strings.Join(got, "\n"); strings.Contains(joined, "expenses:rent") || strings.Contains(joined, "equity:") {
		t.Errorf("categories and equity must not be listed:\n%s", joined)
	}
}

func TestPanelHasTheAccountsTitleAndFillsTheHeight(t *testing.T) {
	got := lines(loaded(20))

	if !strings.HasPrefix(got[0], "╭─ ACCOUNTS ") || len(got) != 20 {
		t.Errorf("panel has %d rows and starts %q, want 20 rows from a titled border", len(got), got[0])
	}
}

func TestPanelIsAThirdOfTheScreenButNotNarrowerThanFifty(t *testing.T) {
	for width, want := range map[int]int{180: 60, 120: 50, 100: 50, 40: 40} {
		got := lines(New().Resize(width, 10).WithAccounts(accounts()))

		if w := lipgloss.Width(got[0]); w != want {
			t.Errorf("screen %d: panel is %d wide, want %d", width, w, want)
		}
	}
}

func TestBalancesAreRightAligned(t *testing.T) {
	got := lines(loaded(20))

	for _, row := range got[1:8] {
		if !strings.HasSuffix(strings.TrimRight(row, " │"), "EUR") && !strings.Contains(row, "closed") {
			t.Errorf("row %q does not end in a balance", row)
		}
	}
	checking := got[2]
	if !strings.Contains(checking, "13.240,00 EUR") || lipgloss.Width(checking) != 60 {
		t.Errorf("checking row = %q", checking)
	}
	if !strings.HasSuffix(checking, "13.240,00 EUR │") {
		t.Errorf("balance should end one cell before the border: %q", checking)
	}
}

func TestSourceGlyphs(t *testing.T) {
	got := lines(loaded(20))

	for row, glyph := range map[int]string{1: "✎", 2: "↓", 3: "?", 5: "⇄", 6: "↓"} {
		if !strings.HasPrefix(strings.TrimPrefix(got[row], "│"), " "+glyph+" ") {
			t.Errorf("row %d = %q, want glyph %s", row, got[row], glyph)
		}
	}
}

func TestClosedAccountsShowTheirClosedDate(t *testing.T) {
	if row := lines(loaded(20))[7]; !strings.Contains(row, "closed 2026-03-01") || strings.Contains(row, "0,00") {
		t.Errorf("closed row = %q", row)
	}
}

func TestStylesCarryTheAnsiColours(t *testing.T) {
	raw := strings.Split(loaded(20).View(), "\n")

	if !strings.Contains(raw[6], "\x1b[31m") { // creditcard is negative: ANSI red
		t.Errorf("negative balance is not red: %q", raw[6])
	}
	if strings.Contains(raw[3], "\x1b[31m") { // checking is positive
		t.Errorf("positive balance must not be red: %q", raw[3])
	}
	if !strings.Contains(raw[7], "\x1b[90m") { // closed rows are dim (ANSI 8 = bright black)
		t.Errorf("closed row is not dim: %q", raw[7])
	}
	if !strings.Contains(raw[1], ";100m") { // the first row is selected: ANSI 8 background
		t.Errorf("selected row has no selection background: %q", raw[1])
	}
}

func TestPositionStartsOnTheFirstRow(t *testing.T) {
	if row, total := loaded(20).Position(); row != 1 || total != 7 {
		t.Errorf("Position() = %d/%d, want 1/7", row, total)
	}
	if row, total := New().Position(); row != 0 || total != 0 {
		t.Errorf("empty Position() = %d/%d, want 0/0", row, total)
	}
}

func TestMoveKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		keys []string
		want int
	}{
		"down":                 {[]string{"down"}, 2},
		"up stops at the top":  {[]string{"up"}, 1},
		"end":                  {[]string{"end"}, 7},
		"down stops at end":    {[]string{"end", "down"}, 7},
		"home":                 {[]string{"end", "home"}, 1},
		"pgdown":               {[]string{"pgdown"}, 7},
		"pgup":                 {[]string{"end", "pgup"}, 1},
		"vim keys are unbound": {[]string{"j", "k"}, 1},
	} {
		m := loaded(20)
		for _, key := range tc.keys {
			m = m.Move(key)
		}
		if row, _ := m.Position(); row != tc.want {
			t.Errorf("%s: row = %d, want %d", name, row, tc.want)
		}
	}
}

func TestPagingMovesByThePanelsBodyHeight(t *testing.T) {
	m := loaded(5).Move("pgdown") // body is 3 rows

	if row, _ := m.Position(); row != 4 {
		t.Errorf("row = %d, want 4", row)
	}
	if row, _ := m.Move("pgup").Position(); row != 1 {
		t.Errorf("row after pgup = %d, want 1", row)
	}
}

func TestTheCursorRowStaysVisibleWhenTheListScrolls(t *testing.T) {
	m := loaded(5) // three visible rows of seven

	for range 5 {
		m = m.Move("down")
	}

	got := lines(m)
	if !strings.Contains(got[1], "assets:savings") || !strings.Contains(got[3], "liabilities:creditcard") {
		t.Errorf("scrolled panel:\n%s", strings.Join(got, "\n"))
	}
	if raw := strings.Split(m.View(), "\n"); !strings.Contains(raw[3], ";100m") {
		t.Errorf("the cursor row should be highlighted after scrolling: %q", raw[3])
	}
	if m = m.Move("home"); !strings.Contains(lines(m)[1], "assets:cash") {
		t.Errorf("home should scroll back to the top:\n%s", strings.Join(lines(m), "\n"))
	}
}

func TestUnknownKeysDoNothing(t *testing.T) {
	m := loaded(20).Move("down")

	if row, _ := m.Move("x").Position(); row != 2 {
		t.Errorf("row = %d, want 2", row)
	}
}

func TestRefreshKeepsTheCursorRowNumberAndClampsIt(t *testing.T) {
	m := loaded(20).Move("end")

	if row, _ := m.WithAccounts(accounts()).Position(); row != 7 {
		t.Errorf("row after refresh = %d, want 7", row)
	}
	if row, total := m.WithAccounts(accounts()[:3]).Position(); total != 2 || row != 2 {
		t.Errorf("after the list shrank: %d/%d, want 2/2", row, total)
	}
}

func TestAMultiCommodityBalanceShowsTheExtraCount(t *testing.T) {
	wallet := domain.Account{Name: "assets:wallet", Source: domain.SourceManual, Balance: domain.MultiAmount{eur(100)[0], eur(200)[0]}}

	got := lines(New().Resize(180, 10).WithAccounts([]domain.Account{wallet}))

	if !strings.Contains(got[1], "1,00 EUR + 1 cur") {
		t.Errorf("row = %q", got[1])
	}
}

func TestATinyPanelStillRendersARow(t *testing.T) {
	got := lines(New().Resize(20, 3).WithAccounts(accounts()))

	if len(got) != 3 || !strings.HasPrefix(got[1], "│") {
		t.Errorf("tiny panel = %q", got)
	}
}

func TestEveryLineFillsThePanelInside(t *testing.T) {
	const inside = 58 // a 60-wide panel less its borders
	selected := accountLine(accounts()[5], inside, true)
	open := accountLine(accounts()[5], inside, false)
	closed := accountLine(accounts()[2], inside, false)

	// Unselected rows leave one cell before the border; the selection highlight runs to it.
	if got := lipgloss.Width(selected); got != inside {
		t.Errorf("selected line is %d wide, want %d", got, inside)
	}
	if got := lipgloss.Width(open); got != inside-1 {
		t.Errorf("open line is %d wide, want %d", got, inside-1)
	}
	if got := lipgloss.Width(closed); got != inside-1 {
		t.Errorf("closed line is %d wide, want %d", got, inside-1)
	}
}
