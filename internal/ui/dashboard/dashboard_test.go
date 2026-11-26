package dashboard

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
	"github.com/sbradl/konto/internal/ui/uitest"
)

var euro = domain.DefaultAmount("EUR").Style

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

// opening is the escape sequence the style starts its text with.
func opening(style lipgloss.Style) string {
	before, _, _ := strings.Cut(style.Render("x"), "x")

	return before
}

func lines(m Model) []string { return strings.Split(uitest.Plain(m.accountsPanel()), "\n") }

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

func TestPanelHasTheAccountsTitleAndIsAsTallAsItsRows(t *testing.T) {
	got := lines(loaded(40))

	if want := len(listed()) + layout.PanelChrome; !strings.HasPrefix(got[0], "╭─ ACCOUNTS ") || len(got) != want {
		t.Errorf("panel has %d rows and starts %q, want %d rows from a titled border", len(got), got[0], want)
	}
}

func TestPanelFillsTheHeightWhenActivityDoesNotFit(t *testing.T) {
	if got := lines(loaded(minAccountsHeight + activityHeight - 1)); len(got) != minAccountsHeight+activityHeight-1 {
		t.Errorf("panel has %d rows, want the whole height", len(got))
	}
}

func TestPanelScrollsRatherThanSqueezeActivity(t *testing.T) {
	if got := lines(loaded(activityHeight + 6)); len(got) != 6 {
		t.Errorf("panel has %d rows, want what Activity leaves", len(got))
	}
}

func TestPanelIsAtLeastAsTallAsTheCardsBesideIt(t *testing.T) {
	if got := lines(New().Resize(180, 30).WithAccounts(nil)); len(got) != cardHeight {
		t.Errorf("panel has %d rows, want the cards' %d", len(got), cardHeight)
	}
}

// listed is how many accounts() the panel lists: assets and liabilities.
func listed() []domain.Account { return New().WithAccounts(accounts()).accounts }

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
	raw := strings.Split(loaded(20).accountsPanel(), "\n")

	if !strings.Contains(raw[6], opening(theme.Negative)) { // creditcard is negative: ANSI red
		t.Errorf("negative balance is not red: %q", raw[6])
	}

	if strings.Contains(raw[3], opening(theme.Negative)) { // checking is positive
		t.Errorf("positive balance must not be red: %q", raw[3])
	}

	if !strings.Contains(raw[7], opening(theme.Dim)) { // closed rows are dim (ANSI 8 = bright black)
		t.Errorf("closed row is not dim: %q", raw[7])
	}

	if !strings.Contains(raw[1], opening(theme.Selected)) { // the first row is selected: ANSI 8 background
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

	if raw := strings.Split(m.accountsPanel(), "\n"); !strings.Contains(raw[3], opening(theme.Selected)) {
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

func TestSelectedIsTheAccountUnderTheCursor(t *testing.T) {
	m := loaded(20)

	if account, ok := m.Selected(); !ok || account.Name != "assets:cash" {
		t.Errorf("Selected() = %q, %v; want assets:cash", account.Name, ok)
	}

	if account, _ := m.Move("end").Selected(); account.Name != "assets:old-account" {
		t.Errorf("after End: Selected() = %q, want assets:old-account", account.Name)
	}

	if _, ok := New().Selected(); ok {
		t.Error("an empty panel selects nothing")
	}
}

func ledgerFigures() domain.Ledger {
	return domain.Ledger{
		Accounts: accounts(),
		Month: domain.MonthSummary{
			Month: time.September, Income: eur(300000), Expenses: eur(118500),
		},
	}
}

func viewLines(m Model) []string { return strings.Split(uitest.Plain(m.View()), "\n") }

func TestStatCardsShowNetWorthAndTheMonthsNetIncome(t *testing.T) {
	m := New().Resize(180, 20).WithLedger(ledgerFigures())

	got := strings.Join(viewLines(m), "\n")

	for _, want := range []string{
		"╭─ NET WORTH ", "╭─ SEPTEMBER NET INCOME ", "1.815,00 EUR", "3.000,00 in / 1.185,00 out", "assets + liabilities",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("view lacks %q:\n%s", want, got)
		}
	}
	// open assets and liabilities: 50,00 + 13.240,00 + 1,00 + 13.500,00 + 0 - 140,00
	if !strings.Contains(got, "26.651,00 EUR") {
		t.Errorf("net worth should sum the open assets and liabilities:\n%s", got)
	}
}

func TestCardsSitRightOfTheAccountsPanelAndTheViewFillsTheHeight(t *testing.T) {
	got := viewLines(New().Resize(180, 20).WithLedger(ledgerFigures()))

	if len(got) != 20 {
		t.Errorf("view has %d rows, want 20", len(got))
	}

	if !strings.HasPrefix(got[0], "╭─ ACCOUNTS ") || !strings.Contains(got[0], "╮╭─ NET WORTH ") {
		t.Errorf("first row = %q, want the Accounts panel then the cards", got[0])
	}
	for _, line := range got {
		if w := lipgloss.Width(line); w > 180 {
			t.Errorf("line is %d wide, want at most 180: %q", w, line)
		}
	}

	if !strings.HasPrefix(got[cardHeight], "│ ") || !strings.HasSuffix(got[cardHeight], "│"+strings.Repeat(" ", 120)) {
		t.Errorf("row under the cards = %q, want the Accounts panel continuing beside blank room", got[cardHeight])
	}

	below := len(listed()) + layout.PanelChrome
	if !strings.HasPrefix(got[below], "╭─ ACTIVITY ") || lipgloss.Width(got[below]) != 180 {
		t.Errorf("row under Accounts = %q, want Activity across the full width", got[below])
	}
}

func TestNetIncomeIsGreenOrRedBySign(t *testing.T) {
	positive := New().Resize(180, 20).WithLedger(ledgerFigures()).View()
	negativeLedger := ledgerFigures()
	negativeLedger.Month.Income = eur(0)
	negative := New().Resize(180, 20).WithLedger(negativeLedger).View()

	if !strings.Contains(positive, opening(theme.Ok)+"1.815,00 EUR") {
		t.Errorf("positive net income should be green: %q", strings.Split(positive, "\n")[1])
	}

	if !strings.Contains(negative, opening(theme.Negative)+"-1.185,00 EUR") {
		t.Errorf("negative net income should be red: %q", strings.Split(negative, "\n")[1])
	}
}

func TestCardsGiveWayOnANarrowScreen(t *testing.T) {
	m := New().Resize(80, 20).WithLedger(ledgerFigures())

	if got := strings.Join(viewLines(m), "\n"); strings.Contains(got, "NET WORTH") {
		t.Errorf("a screen too narrow for two cards should show the Accounts panel alone:\n%s", got)
	}
}

func TestAMonthWithoutActivityShowsZeros(t *testing.T) {
	ledger := domain.Ledger{Accounts: accounts(), Month: domain.MonthSummary{Month: time.October}}

	got := strings.Join(viewLines(New().Resize(180, 20).WithLedger(ledger)), "\n")

	if !strings.Contains(got, "OCTOBER NET INCOME") || !strings.Contains(got, "0,00 EUR") || !strings.Contains(got, "0,00 in / 0,00 out") {
		t.Errorf("view should show zeros for an empty month:\n%s", got)
	}
}

func TestSeveralCommoditiesShowAsExtraCounts(t *testing.T) {
	pln := domain.MultiAmount{{Commodity: "PLN", Mantissa: 100, Places: 2, Style: euro}}
	ledger := ledgerFigures()
	ledger.Month.Income = ledger.Month.Income.Plus(pln)

	if got := strings.Join(viewLines(New().Resize(180, 20).WithLedger(ledger)), "\n"); !strings.Contains(got, "3.000,00 EUR + 1 cur in") {
		t.Errorf("a multi-commodity figure should read + N cur:\n%s", got)
	}
}

func TestCardsAppearOnceTwoMinimumWidthsFitBesideThePanel(t *testing.T) {
	// The Accounts panel is 50 wide at these sizes, so the cards get what is left.
	for width, want := range map[int]bool{97: false, 98: true} {
		got := strings.Contains(strings.Join(viewLines(New().Resize(width, 20).WithLedger(ledgerFigures())), "\n"), "NET WORTH")

		if got != want {
			t.Errorf("width %d: cards shown = %v, want %v", width, got, want)
		}
	}
}

func TestACardWithoutAnyFigureStillRenders(t *testing.T) {
	ledger := domain.Ledger{Accounts: []domain.Account{{Name: "assets:new"}}, Month: domain.MonthSummary{Month: time.June}}

	if got := strings.Join(viewLines(New().Resize(180, 20).WithLedger(ledger)), "\n"); !strings.Contains(got, "JUNE NET INCOME") {
		t.Errorf("view should still show the cards:\n%s", got)
	}
}

func withTransactions(n int) domain.Ledger {
	ledger := ledgerFigures()
	for i := range n {
		date := time.Date(2026, 9, 1+i%28, 0, 0, 0, 0, time.UTC)
		ledger.Transactions = append(ledger.Transactions, domain.Transaction{
			Date: date, Description: fmt.Sprintf("Shop %02d", i),
			Postings: []domain.Posting{
				{Account: "expenses:food", Amounts: eur(1000)},
				{Account: "assets:cash", Amounts: eur(-1000)},
			},
		})
	}

	return ledger
}

// withUpcoming adds one occurrence a day from 2026-10-01 to the ledger.
func withUpcoming(ledger domain.Ledger, descriptions ...string) domain.Ledger {
	for i, description := range descriptions {
		ledger.Upcoming = append(ledger.Upcoming, domain.Transaction{
			Date: time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC), Description: description,
			Postings: []domain.Posting{
				{Account: "expenses:rent", Amounts: eur(90000)},
				{Account: "assets:cash", Amounts: eur(-90000)},
			},
		})
	}
	ledger.Today = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	return ledger
}

func activityRows(m Model) []string {
	return panelRows(m, "ACTIVITY", func(line []rune) []rune { return line })
}

// panelRows are the inside rows of the titled panel, read from one column of the view.
func panelRows(m Model, title string, column func([]rune) []rune) []string {
	var rows []string
	inside := false
	for _, full := range viewLines(m) {
		line := string(column([]rune(full)))
		switch {
		case strings.Contains(line, "╭─ "+title+" "):
			inside = true
		case inside && strings.Contains(line, "╰"):
			return rows
		case inside:
			rows = append(rows, line)
		}
	}

	return rows
}

// thirty are the descriptions of thirty occurrences.
func thirty() []string {
	listed := make([]string, 30)
	for i := range listed {
		listed[i] = fmt.Sprintf("entry %02d", i)
	}

	return listed
}

func TestActivityListsUpcomingFurthestFirstOverTodayOverRecentNewestFirst(t *testing.T) {
	rows := activityRows(New().Resize(180, 40).WithLedger(withUpcoming(withTransactions(2), "Vermieter Mueller", "Streaming")))

	for i, want := range []string{
		"October 2026", "2026-10-02  Streaming", "2026-10-01  Vermieter Mueller", "━━ today 2026-09-30 ━━",
		"September 2026", "2026-09-02  Shop 01", "2026-09-01  Shop 00",
	} {
		if !strings.Contains(rows[i], want) {
			t.Errorf("row %d = %q, want it to hold %q", i, rows[i], want)
		}
	}
}

func TestAnOverdueOccurrenceSitsJustAboveTodayInRed(t *testing.T) {
	ledger := withUpcoming(withTransactions(1), "Streaming")
	ledger.Past = []domain.Transaction{{
		Date: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Description: "Gym",
		Postings: []domain.Posting{{Account: "expenses:gym", Amounts: eur(3000)}, {Account: "assets:cash", Amounts: eur(-3000)}},
	}}
	m := New().Resize(180, 40).WithLedger(ledger)
	rows := activityRows(m)

	today := slices.IndexFunc(rows, func(row string) bool { return strings.Contains(row, "━━ today") })
	if today < 1 || !strings.Contains(rows[today-1], "2026-09-15  Gym") {
		t.Errorf("the overdue Gym sits right above the today line:\n%s", strings.Join(rows, "\n"))
	}

	if !strings.Contains(m.View(), "\x1b[31m2026-09-15") {
		t.Errorf("an overdue date is red:\n%q", m.View())
	}
}

func TestEachMonthOpensWithItsIncomeAndExpensesAboveItsLatestRow(t *testing.T) {
	ledger := withUpcoming(withTransactions(1), "rent")
	ledger.Upcoming = append(ledger.Upcoming, domain.Transaction{
		Date: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), Description: "salary",
		Postings: []domain.Posting{{Account: "assets:cash", Amounts: eur(300000)}, {Account: "income:salary", Amounts: eur(-300000)}},
	})
	ledger.Transactions = append(ledger.Transactions, domain.Transaction{
		Date: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), Description: "august",
		Postings: []domain.Posting{{Account: "expenses:food", Amounts: eur(500)}, {Account: "assets:cash", Amounts: eur(-500)}},
	})
	m := New().Resize(180, 40).WithLedger(ledger)

	for i, want := range []string{
		"── October 2026 ── -900,00 EUR (0,00 EUR / -900,00 EUR) ──", "rent",
		"── September 2026 ── +2.990,00 EUR (+3.000,00 EUR / -10,00 EUR) ──", "salary", "today", "Shop 00", // the month counts both sides of today
		"── August 2026 ── -5,00 EUR (0,00 EUR / -5,00 EUR) ──", "august",
	} {
		if row := activityRows(m)[i]; !strings.Contains(row, want) {
			t.Errorf("row %d = %q, want it to hold %q", i, row, want)
		}
	}

	if view := m.View(); !strings.Contains(view, opening(theme.Ok)+"+2.990,00 EUR") ||
		!strings.Contains(view, "── \x1b[m"+opening(theme.Negative)+"-900,00 EUR") ||
		!strings.Contains(view, "("+opening(theme.Ok)+"+3.000,00 EUR") ||
		!strings.Contains(view, opening(theme.Negative)+"-10,00 EUR\x1b[m)") {
		t.Errorf("the net should be signed and coloured, income green and expenses red:\n%q", view)
	}

	if row := activityRows(m)[0]; !strings.HasSuffix(row, "─ │") || lipgloss.Width(row) != 180 {
		t.Errorf("month line = %q, want it to run to the panel's edge", row)
	}

	if view := m.View(); !strings.Contains(view, opening(theme.Warn)+" ── ") {
		t.Errorf("the month line's rule should be yellow:\n%q", view)
	}
}

func TestTodayLineSpansThePanelInTheWarnColour(t *testing.T) {
	m := New().Resize(180, 40).WithLedger(withUpcoming(withTransactions(1)))

	if view := m.View(); !strings.Contains(view, opening(theme.Warn)+" ━━ today 2026-09-30 ━") {
		t.Errorf("the today line should be yellow:\n%q", view)
	}

	if row := activityRows(m)[0]; !strings.HasSuffix(row, "━ │") || lipgloss.Width(row) != 180 {
		t.Errorf("today line = %q, want it to run to the panel's edge", row)
	}
}

func TestActivityRowShowsDescriptionCategoryAmountAndBalance(t *testing.T) {
	ledger := domain.Ledger{Accounts: accounts(), Transactions: []domain.Transaction{
		{Date: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), Description: "REWE", Postings: []domain.Posting{
			{Account: "expenses:groceries", Amounts: eur(25000)}, {Account: "assets:cash", Amounts: eur(-25000)},
		}},
		{Date: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), Description: "Acme GmbH", Postings: []domain.Posting{
			{Account: "assets:cash", Amounts: eur(300000)}, {Account: "income:salary", Amounts: eur(-300000)},
		}},
	}}

	rows := activityRows(New().Resize(180, 40).WithLedger(ledger))[2:] // under the today and month lines

	if !strings.Contains(rows[0], "2026-09-25  Acme GmbH") || !strings.Contains(rows[0], "income:salary") ||
		!strings.HasSuffix(rows[0], "3.000,00 EUR        50,00 EUR │") {
		t.Errorf("first row = %q, want the salary and the balance after it", rows[0])
	}

	if !strings.Contains(rows[1], "2026-09-10  REWE") || !strings.Contains(rows[1], "expenses:groceries") ||
		!strings.HasSuffix(rows[1], "-250,00 EUR    -2.950,00 EUR │") {
		t.Errorf("second row = %q, want the groceries and the balance after them", rows[1])
	}
}

func TestUpcomingBalancesRunForwardFromTodaysOverToday(t *testing.T) {
	rows := activityRows(New().Resize(180, 40).WithLedger(withUpcoming(ledgerFigures(), "a", "b")))[1:] // under October

	for i, want := range []string{"-900,00 EUR    -1.750,00 EUR │", "-900,00 EUR      -850,00 EUR │"} {
		if !strings.HasSuffix(rows[i], want) {
			t.Errorf("row %d = %q, want it to end with %q", i, rows[i], want)
		}
	}
}

func TestAmountsAndBalancesAreGreenOrRed(t *testing.T) {
	ledger := domain.Ledger{
		Accounts: []domain.Account{{Name: "assets:a", Source: domain.SourceManual, Balance: eur(-200)}},
		Transactions: []domain.Transaction{
			{Date: day1, Description: "in", Postings: []domain.Posting{{Account: "assets:a", Amounts: eur(400)}, {Account: "income:x", Amounts: eur(-400)}}},
			{Date: day1, Description: "out", Postings: []domain.Posting{{Account: "assets:a", Amounts: eur(-500)}, {Account: "expenses:x", Amounts: eur(500)}}},
		},
	}

	view := New().Resize(180, 20).WithLedger(ledger).View()

	for _, want := range []string{
		opening(theme.Ok) + "4,00 EUR", opening(theme.Negative) + "-5,00 EUR", // amounts
		opening(theme.Ok) + "3,00 EUR", opening(theme.Negative) + "-2,00 EUR", // balances
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%q", want, view)
		}
	}
}

var day1 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func TestActivityOpensWithTheTodayLineInTheMiddle(t *testing.T) {
	m := New().Resize(180, 30).WithLedger(withUpcoming(withTransactions(30), thirty()...))
	rows := activityRows(m) // 30 less Accounts' 9 rows less the border: 19 inside

	if len(rows) != 19 || !strings.Contains(rows[9], "today") {
		t.Fatalf("today should be the middle of 19 rows:\n%s", strings.Join(rows, "\n"))
	}

	if !strings.Contains(rows[0], "… 23 more") || !strings.Contains(rows[18], "… 23 more") {
		t.Errorf("first and last rows should count what is beyond them:\n%s", strings.Join(rows, "\n"))
	}

	if row, total := press(m, "tab").Position(); row != 23 || total != 63 {
		t.Errorf("position = %d/%d, want 23/63", row, total)
	}
}

func TestTodayIsTheFirstRowWithoutUpcomingOccurrences(t *testing.T) {
	rows := activityRows(New().Resize(180, 30).WithLedger(withUpcoming(withTransactions(3))))

	if !strings.Contains(rows[0], "today") || !strings.Contains(rows[1], "September 2026") || !strings.Contains(rows[2], "Shop 02") {
		t.Errorf("rows:\n%s", strings.Join(rows, "\n"))
	}
}

func TestActivityRowsFillThePanelWidth(t *testing.T) {
	for _, line := range activityRows(New().Resize(180, 24).WithLedger(withTransactions(3)))[:4] {
		if w := lipgloss.Width(line); w != 180 {
			t.Errorf("row is %d wide, want 180: %q", w, line)
		}
	}
}

func TestActivityCutsLongDescriptionsToKeepTheColumns(t *testing.T) {
	ledger := withTransactions(1)
	ledger.Transactions[0].Description = strings.Repeat("long ", 40)

	row := activityRows(New().Resize(60, 24).WithLedger(ledger))[2]

	if lipgloss.Width(row) != 60 || !strings.HasSuffix(row, "-10,00 EUR        50,00 EUR │") || strings.Contains(row, "expenses") {
		t.Errorf("row = %q, want it cut to the width with the amount and balance kept and no room for the category", row)
	}
}

func TestActivityColumnsKeepTheirPlaceWhileScrolling(t *testing.T) {
	ledger := withTransactions(40)
	// the newest page stays under a million, the oldest pass it and need two cells more
	ledger.Accounts = []domain.Account{{Name: "assets:cash", Source: domain.SourceManual, Balance: eur(99980000)}}
	m := press(New().Resize(180, 24).WithLedger(ledger), "tab")

	top, bottom := activityRows(press(m, "home"))[2], activityRows(press(m, "end"))[1]

	if at, want := strings.Index(top, "-10,00 EUR"), strings.Index(bottom, "-10,00 EUR"); at != want {
		t.Errorf("the amount moves from column %d to %d:\n%s\n%s", want, at, bottom, top)
	}
}

func TestActivityFiguresHaveRoomForAHundredThousand(t *testing.T) {
	row := activityRows(New().Resize(180, 24).WithLedger(withTransactions(1)))[2]

	if !strings.HasSuffix(row, "     -10,00 EUR        50,00 EUR │") {
		t.Errorf("row = %q, want amount and balance %d wide", row, len("-100.000,00 EUR"))
	}
}

func TestActivityRowsAreNotSelectable(t *testing.T) {
	m := press(New().Resize(180, 24).WithLedger(withUpcoming(withTransactions(3), "a")), "tab", "down")

	for _, line := range strings.Split(m.View(), "\n")[m.accountsHeight():] {
		if strings.Contains(line, "100m") {
			t.Errorf("an Activity row is highlighted: %q", line)
		}
	}
}

func TestActivityGivesWayWhenThereIsNoRoomUnderAccounts(t *testing.T) {
	view := strings.Join(viewLines(New().Resize(180, minAccountsHeight+activityHeight-1).WithLedger(withUpcoming(ledgerFigures(), "a"))), "\n")

	if strings.Contains(view, "ACTIVITY") || !strings.Contains(view, "NET WORTH") {
		t.Errorf("a short screen keeps Accounts and the cards only:\n%s", view)
	}
}

func TestHiddenPanelsCountWhatTheTerminalIsTooSmallFor(t *testing.T) {
	for name, tc := range map[string]struct{ width, height, want int }{
		"everything fits":          {180, 40, 0},
		"no room for activity":     {180, minAccountsHeight + activityHeight - 1, 1},
		"activity just fits":       {minAccountsWidth + 2*minCardWidth, minAccountsHeight + activityHeight, 0},
		"no room for the cards":    {minAccountsWidth + 2*minCardWidth - 1, 40, 2},
		"accounts alone, too tiny": {40, 5, 3},
	} {
		if got := New().Resize(tc.width, tc.height).HiddenPanels(); got != tc.want {
			t.Errorf("%s: %d hidden, want %d", name, got, tc.want)
		}
	}
}

func TestKeyAsksToCloseOrReopenAndMovesOnCursorKeys(t *testing.T) {
	m := loaded(20)
	for key, want := range map[string]Action{"c": CloseSelected{}, "o": ReopenSelected{}, "x": nil} {
		if _, got := m.Key(key); got != want {
			t.Errorf("%s: action = %v, want %v", key, got, want)
		}
	}

	if moved, action := m.Key("down"); action != nil || moved.cursor.Row != 1 {
		t.Errorf("down: action = %v, row = %d; want the cursor moved", action, moved.cursor.Row)
	}
}

func TestBalancesWrittenWithoutAllTheirDecimalsStillShowTwo(t *testing.T) {
	style := domain.DefaultAmount("EUR").Style
	whole := domain.Account{Name: "assets:whole", Balance: domain.MultiAmount{{Commodity: "EUR", Mantissa: -6, Style: style}}}
	half := domain.Account{Name: "assets:half", Balance: domain.MultiAmount{{Commodity: "EUR", Mantissa: 65, Places: 1, Style: style}}}

	view := strings.Join(lines(New().Resize(120, 30).WithAccounts([]domain.Account{whole, half})), "\n")
	if !strings.Contains(view, "-6,00 EUR") || !strings.Contains(view, "6,50 EUR") {
		t.Errorf("the accounts panel:\n%s", view)
	}
}

func press(m Model, keys ...string) Model {
	for _, key := range keys {
		m, _ = m.Key(key)
	}

	return m
}

// focusedTitles are the panel titles that carry the focus mark.
func focusedTitles(m Model) []string {
	var titles []string
	for _, line := range viewLines(m) {
		for _, title := range []string{"ACCOUNTS", "ACTIVITY"} {
			if strings.Contains(line, "╭─ "+title+" ◂") {
				titles = append(titles, title)
			}
		}
	}

	return titles
}

func TestTabAndShiftTabMoveTheFocusBetweenAccountsAndActivity(t *testing.T) {
	m := New().Resize(180, 30).WithLedger(withUpcoming(ledgerFigures(), "a"))

	for i, want := range []string{"ACCOUNTS", "ACTIVITY", "ACCOUNTS"} {
		if got := focusedTitles(press(m, slices.Repeat([]string{"tab"}, i)...)); !slices.Equal(got, []string{want}) {
			t.Errorf("after %d tabs the focus is on %v, want %s", i, got, want)
		}
	}

	if got := focusedTitles(press(m, "shift+tab")); !slices.Equal(got, []string{"ACTIVITY"}) {
		t.Errorf("shift+tab from Accounts focuses %v, want Activity", got)
	}
}

func TestTabStaysOnAccountsWithoutRoomForActivity(t *testing.T) {
	short := New().Resize(180, minAccountsHeight+activityHeight-1).WithLedger(withUpcoming(ledgerFigures(), "a"))

	if got := focusedTitles(press(short, "tab")); !slices.Equal(got, []string{"ACCOUNTS"}) {
		t.Errorf("without Activity, tab focuses %v, want Accounts", got)
	}
}

func TestTheFocusFallsBackToAccountsWhenActivityNoLongerFits(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming(ledgerFigures(), "a")), "tab").Resize(180, 12)

	if got := focusedTitles(m); !slices.Equal(got, []string{"ACCOUNTS"}) {
		t.Errorf("focus on %v, want Accounts", got)
	}

	if row, _ := press(m, "down").Position(); row != 2 {
		t.Errorf("the arrows should drive Accounts again, row %d", row)
	}
}

func TestTheArrowsScrollTheFocusedActivityPanel(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming(withTransactions(30), thirty()...)), "tab") // 19 inside

	up := press(m, "up")
	if rows := activityRows(up); !strings.Contains(rows[10], "today") {
		t.Errorf("one up moves today a row down:\n%s", strings.Join(rows, "\n"))
	}

	if account, _ := up.Selected(); account.Name != "assets:cash" {
		t.Errorf("scrolling Activity moved the Accounts cursor to %s", account.Name)
	}

	home := activityRows(press(m, "home"))
	if !strings.Contains(home[1], "entry 29") || !strings.Contains(home[18], "… 45 more") {
		t.Errorf("home shows the furthest occurrence first:\n%s", strings.Join(home, "\n"))
	}

	end := activityRows(press(m, "end"))
	if !strings.Contains(end[0], "… 45 more") || !strings.Contains(end[18], "Shop 00") {
		t.Errorf("end shows the oldest transaction last:\n%s", strings.Join(end, "\n"))
	}
}

func TestTCentresTheTodayLineAgainFromEitherPanel(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming(withTransactions(30), thirty()...)), "tab", "end")

	for _, keys := range [][]string{{"t"}, {"tab", "t"}} {
		centred, action := press(m, keys[:len(keys)-1]...).Key("t")
		if rows := activityRows(centred); action != nil || !strings.Contains(rows[9], "today") {
			t.Errorf("after %v today should be the middle row again:\n%s", keys, strings.Join(rows, "\n"))
		}
	}

	if got := Bindings(); !slices.Contains(got, layout.Binding{Key: "t", Action: "today"}) {
		t.Errorf("bindings = %v, want t today", got)
	}
}

func TestAScrolledPanelStaysOnItsLastPageWhenTheLedgerShrinks(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming(withTransactions(30), thirty()...)), "tab", "end").
		WithLedger(withUpcoming(withTransactions(20), thirty()...))

	if rows := activityRows(m); !strings.Contains(rows[18], "Shop 00") {
		t.Errorf("rows:\n%s", strings.Join(rows, "\n"))
	}

	if row, total := m.Position(); row != 35 || total != 53 {
		t.Errorf("position = %d/%d, want 35/53", row, total)
	}
}

func TestAnEmptyActivityHoldsTheTodayLineAlone(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming(ledgerFigures())), "tab")

	if row, total := m.Position(); row != 1 || total != 1 {
		t.Errorf("position = %d/%d, want 1/1", row, total)
	}
}

func TestAMonthThatCameToNothingHasNoSign(t *testing.T) {
	line := uitest.Plain(monthLine(day1, domain.MonthSummary{Income: eur(500), Expenses: eur(500)}, 80))

	if !strings.Contains(line, "── 0,00 EUR (+5,00 EUR / -5,00 EUR) ──") {
		t.Errorf("line = %q, want an unsigned zero", line)
	}
}
