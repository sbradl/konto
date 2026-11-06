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

func TestPanelFillsTheHeightWhenUpcomingDoesNotFit(t *testing.T) {
	if got := lines(loaded(minAccountsHeight + upcomingHeight - 1)); len(got) != minAccountsHeight+upcomingHeight-1 {
		t.Errorf("panel has %d rows, want the whole height", len(got))
	}
}

func TestPanelScrollsRatherThanSqueezeUpcoming(t *testing.T) {
	if got := lines(loaded(upcomingHeight + 6)); len(got) != 6 {
		t.Errorf("panel has %d rows, want what Upcoming leaves", len(got))
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

	if !strings.HasPrefix(got[cardHeight], "│ ") || !strings.Contains(got[cardHeight], "╭─ RECENT ") {
		t.Errorf("row under the cards = %q, want the Accounts panel continuing beside the Recent panel", got[cardHeight])
	}

	below := len(listed()) + layout.PanelChrome
	if !strings.HasPrefix(got[below], "╭─ UPCOMING ") {
		t.Errorf("row under Accounts = %q, want Upcoming", got[below])
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

func recentRows(m Model) []string {
	return panelRows(m, "RECENT", func(line []rune) []rune { return line[m.accountsWidth():] })
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

func TestRecentListsNewestFirstWithDescriptionCategoryAndAmount(t *testing.T) {
	ledger := domain.Ledger{Accounts: accounts(), Transactions: []domain.Transaction{
		{Date: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), Description: "REWE", Postings: []domain.Posting{
			{Account: "expenses:groceries", Amounts: eur(25000)}, {Account: "assets:cash", Amounts: eur(-25000)},
		}},
		{Date: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), Description: "Acme GmbH", Postings: []domain.Posting{
			{Account: "assets:cash", Amounts: eur(300000)}, {Account: "income:salary", Amounts: eur(-300000)},
		}},
	}}

	rows := recentRows(New().Resize(180, 20).WithLedger(ledger))

	if !strings.Contains(rows[0], "2026-09-25  Acme GmbH") || !strings.Contains(rows[0], "income:salary") || !strings.Contains(rows[0], "3.000,00 EUR") {
		t.Errorf("first row = %q, want the salary", rows[0])
	}

	if !strings.Contains(rows[1], "2026-09-10  REWE") || !strings.Contains(rows[1], "expenses:groceries") || !strings.Contains(rows[1], "-250,00 EUR") {
		t.Errorf("second row = %q, want the groceries", rows[1])
	}
}

func TestRecentAmountsAreGreenOrRed(t *testing.T) {
	ledger := domain.Ledger{Transactions: []domain.Transaction{
		{Date: day1, Description: "in", Postings: []domain.Posting{{Account: "assets:a", Amounts: eur(100)}, {Account: "income:x", Amounts: eur(-100)}}},
		{Date: day1, Description: "out", Postings: []domain.Posting{{Account: "assets:a", Amounts: eur(-100)}, {Account: "expenses:x", Amounts: eur(100)}}},
	}}

	view := New().Resize(180, 20).WithLedger(ledger).View()

	if !strings.Contains(view, opening(theme.Negative)+"-1,00 EUR") || !strings.Contains(view, opening(theme.Ok)+"1,00 EUR") {
		t.Errorf("amounts should be red when negative and green otherwise:\n%q", view)
	}
}

var day1 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func TestRecentSaysHowManyRowsDontFit(t *testing.T) {
	rows := recentRows(New().Resize(180, 24).WithLedger(withTransactions(200))) // Recent is 10 rows tall: 8 inside

	if len(rows) != 8 {
		t.Fatalf("panel has %d rows, want 8", len(rows))
	}

	if !strings.Contains(rows[7], "… 193 more") || strings.Contains(rows[6], "more") {
		t.Errorf("last rows = %q / %q, want 7 transactions then … 193 more", rows[6], rows[7])
	}
}

func TestRecentShowsExactlyAsManyAsFitWithoutAMoreRow(t *testing.T) {
	rows := recentRows(New().Resize(180, 24).WithLedger(withTransactions(8))) // exactly 8 inside

	if text := strings.Join(rows, "\n"); strings.Contains(text, "more") || !strings.Contains(text, "Shop 07") {
		t.Errorf("all 8 fit, so no … more row:\n%s", text)
	}
}

func TestRecentShowsEverythingThatFitsWithoutAMoreRow(t *testing.T) {
	rows := recentRows(New().Resize(180, 24).WithLedger(withTransactions(3)))

	if strings.Contains(strings.Join(rows, "\n"), "more") {
		t.Errorf("all 8 fit, so no … more row:\n%s", strings.Join(rows, "\n"))
	}
}

func TestRecentRowsFillThePanelWidth(t *testing.T) {
	for _, line := range recentRows(New().Resize(180, 24).WithLedger(withTransactions(3)))[:3] {
		if w := lipgloss.Width(line); w != 120 { // the screen less the 60-wide Accounts column
			t.Errorf("row is %d wide, want 120: %q", w, line)
		}
	}
}

func TestRecentCutsLongDescriptionsToKeepTheColumns(t *testing.T) {
	ledger := withTransactions(1)
	ledger.Transactions[0].Description = strings.Repeat("long ", 40)

	row := recentRows(New().Resize(110, 24).WithLedger(ledger))[0]

	if lipgloss.Width(row) != 110-minAccountsWidth || !strings.HasSuffix(strings.TrimRight(row, " │"), "-10,00 EUR") {
		t.Errorf("row = %q, want it cut to the width with the amount kept", row)
	}
}

func TestRecentIsNotSelectable(t *testing.T) {
	view := New().Resize(180, 24).WithLedger(withTransactions(3)).Move("down").View()

	for line := range strings.SplitSeq(view, "\n") {
		// Each row is the Accounts panel's border-to-border run, then the right column.
		if _, right, found := strings.Cut(line, "│╭"); found && strings.Contains(right, "100m") {
			t.Errorf("a right-column row is highlighted: %q", line)
		}

		if _, right, found := strings.Cut(line, "││"); found && strings.Contains(right, "100m") {
			t.Errorf("a Recent row is highlighted: %q", line)
		}
	}
}

func upcomingRows(m Model) []string {
	return panelRows(m, "UPCOMING", func(line []rune) []rune { return line[:m.accountsWidth()] })
}

func withUpcoming(descriptions ...string) domain.Ledger {
	ledger := ledgerFigures()
	for i, description := range descriptions {
		ledger.Upcoming = append(ledger.Upcoming, domain.Transaction{
			Date: time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC), Description: description,
			Postings: []domain.Posting{
				{Account: "expenses:rent", Amounts: eur(90000)},
				{Account: "assets:cash", Amounts: eur(-90000)},
			},
		})
	}

	return ledger
}

func TestUpcomingListsTheNextOccurrencesSoonestFirstUnderAccounts(t *testing.T) {
	rows := upcomingRows(New().Resize(180, 20).WithLedger(withUpcoming("Vermieter Mueller", "Streaming")))

	if !strings.Contains(rows[0], "2026-10-01  Vermieter Mueller") || !strings.Contains(rows[1], "2026-10-02  Streaming") {
		t.Errorf("rows = %q, want the occurrences in date order", rows)
	}

	if !strings.Contains(rows[0], "-900,00 EUR") {
		t.Errorf("first row = %q, want the amount seen from the account", rows[0])
	}
}

func TestUpcomingWithoutRulesIsAnEmptyPanel(t *testing.T) {
	rows := upcomingRows(New().Resize(180, 20).WithLedger(withUpcoming()))

	panel := strings.ReplaceAll(strings.Join(rows, ""), "│", "")
	if len(rows) == 0 || strings.TrimSpace(panel) != "" {
		t.Errorf("rows = %q, want an empty panel", rows)
	}
}

func TestUpcomingSaysHowManyOccurrencesDontFit(t *testing.T) {
	rows := upcomingRows(New().Resize(180, 20).WithLedger(withUpcoming("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l")))

	if len(rows) != 9 || !strings.Contains(rows[8], "… 4 more") {
		t.Errorf("rows = %q, want eight listed then … 4 more", rows)
	}
}

func TestUpcomingGivesWayWhenThereIsNoRoomForBothPanels(t *testing.T) {
	view := strings.Join(viewLines(New().Resize(180, 14).WithLedger(withUpcoming("a"))), "\n")

	if strings.Contains(view, "UPCOMING") || !strings.Contains(view, "RECENT") {
		t.Errorf("a short screen keeps Recent only:\n%s", view)
	}
}

func TestUpcomingIsNotSelectable(t *testing.T) {
	view := New().Resize(180, 20).WithLedger(withUpcoming("a", "b")).Move("down").View()

	for line := range strings.SplitSeq(view, "\n") {
		if _, right, found := strings.Cut(line, "││"); found && strings.Contains(right, "100m") {
			t.Errorf("a right-column row is highlighted: %q", line)
		}
	}
}

func TestUpcomingFitsWhenRecentKeepsItsMinimum(t *testing.T) {
	view := strings.Join(viewLines(New().Resize(180, cardHeight+upcomingHeight+minRecentHeight).WithLedger(withUpcoming("a"))), "\n")

	if !strings.Contains(view, "UPCOMING") || !strings.Contains(view, "RECENT") {
		t.Errorf("exactly enough room for both panels should show both:\n%s", view)
	}
}

func spendHistory(totals ...int64) []domain.MonthSpend {
	history := make([]domain.MonthSpend, len(totals))
	for i, total := range totals {
		history[i] = domain.MonthSpend{Month: time.Date(2026, time.Month(3+i), 1, 0, 0, 0, 0, time.UTC), Total: eur(total)}
	}

	return history
}

func TestSpendChartLabelsTheSixMonthsOnTheXAxis(t *testing.T) {
	ledger := withUpcoming()
	ledger.Spend = spendHistory(117500, 112000, 119500, 115000, 120500, 113000)

	chart := strings.Join(viewLines(New().Resize(180, 40).WithLedger(ledger)), "\n")

	for _, month := range []string{"Mar", "Apr", "May", "Jun", "Jul", "Aug"} {
		if !strings.Contains(chart, month) {
			t.Errorf("x axis lacks %s:\n%s", month, chart)
		}
	}

	if strings.Contains(chart, "Sep") {
		t.Errorf("the current month is not complete and must not be plotted")
	}
}

func TestSpendChartYAxisCountsThousandsAndTheLineIsBraille(t *testing.T) {
	ledger := withUpcoming()
	ledger.Spend = spendHistory(117500, 112000, 119500, 115000, 120500, 113000)

	view := strings.Join(viewLines(New().Resize(180, 40).WithLedger(ledger)), "\n")

	if !strings.Contains(view, "1.3k") || !strings.Contains(view, "0.0k") {
		t.Errorf("y axis should read in thousands:\n%s", view)
	}

	if !strings.ContainsAny(view, "⠁⠂⠄⡀⢀⠈⠐⠠⣀⠤⠉") {
		t.Errorf("expected a braille line:\n%s", view)
	}
}

func TestSpendChartUsesOnlyAnsi16Colours(t *testing.T) {
	ledger := withUpcoming()
	ledger.Spend = spendHistory(117500, 112000, 119500)

	view := New().Resize(180, 40).WithLedger(ledger).View()

	if strings.Contains(view, "38;2;") || strings.Contains(view, "38;5;") {
		t.Errorf("chart must use ANSI-16 colours only:\n%q", view)
	}
}

func TestSpendChartPlotsTheCommodityOfTheFirstMonthWithSpend(t *testing.T) {
	values, top := spendValues([]domain.MonthSpend{
		{},
		{Total: domain.MultiAmount{{Commodity: "PLN", Mantissa: 10000, Places: 2}, {Commodity: "EUR", Mantissa: 500, Places: 2}}},
		{Total: eur(99900)},
	})

	if values[0] != 0 || values[1] != 100 || values[2] != 0 || top != 100 {
		t.Errorf("values = %v top = %v, want only PLN plotted", values, top)
	}
}

func TestSpendCommodityIsNoneWhenNoMonthSpentAnything(t *testing.T) {
	if got := spendCommodity([]domain.MonthSpend{{}, {}}); got != "" {
		t.Errorf("commodity = %q, want none", got)
	}
}

func TestSpendPanelIsEmptyWithoutHistoryOrRoom(t *testing.T) {
	for name, panel := range map[string]string{
		"no history": spendPanel(nil, 60, 10),
		"too narrow": spendPanel(spendHistory(100), 8, 10),
		"too short":  spendPanel(spendHistory(100), 60, 5),
	} {
		if strings.ContainsAny(uitest.Plain(panel), "⠁⠂⠄⡀⢀⠈⠐⠠⣀⠤⠉k") || !strings.Contains(uitest.Plain(panel), "SPEND") {
			t.Errorf("%s: want an empty SPEND panel:\n%s", name, uitest.Plain(panel))
		}
	}
}

func TestSpendChartSitsUnderRecentWhenItFits(t *testing.T) {
	ledger := withUpcoming("a")
	ledger.Spend = spendHistory(100, 200)

	for height, want := range map[int]bool{cardHeight + spendHeight + minRecentHeight: true, cardHeight + spendHeight + minRecentHeight - 1: false} {
		got := viewLines(New().Resize(180, height).WithLedger(ledger))
		spend := slices.IndexFunc(got, func(line string) bool { return strings.Contains(line, "╭─ SPEND ") })
		recent := slices.IndexFunc(got, func(line string) bool { return strings.Contains(line, "╭─ RECENT ") })

		if (spend >= 0) != want || (want && spend != height-spendHeight) || recent != cardHeight {
			t.Errorf("height %d: spend at %d, recent at %d:\n%s", height, spend, recent, strings.Join(got, "\n"))
		}
	}
}

func TestHiddenPanelsCountWhatTheTerminalIsTooSmallFor(t *testing.T) {
	for name, tc := range map[string]struct{ width, height, want int }{
		"everything fits":          {180, 40, 0},
		"no room for the chart":    {180, cardHeight + spendHeight + minRecentHeight - 1, 1},
		"chart just fits":          {180, cardHeight + spendHeight + minRecentHeight, 0},
		"no room for upcoming":     {180, minAccountsHeight + upcomingHeight - 1, 1 + 1},
		"upcoming just fits":       {minAccountsWidth + 2*minCardWidth, minAccountsHeight + upcomingHeight, 1},
		"no room for the cards":    {minAccountsWidth + 2*minCardWidth - 1, 40, 4},
		"cards just fit":           {minAccountsWidth + 2*minCardWidth, 40, 0},
		"accounts alone, too tiny": {40, 5, 5},
	} {
		if got := New().Resize(tc.width, tc.height).HiddenPanels(); got != tc.want {
			t.Errorf("%s: %d hidden, want %d", name, got, tc.want)
		}
	}
}

func TestSpendPanelDrawsTheChartAtTheSmallestSizeItFits(t *testing.T) {
	want := "╭─ SPEND ╮\n│    │   │\n│    │   │\n│0.0k└───│\n│    Mar │\n╰────────╯"

	if got := uitest.Plain(spendPanel(spendHistory(100), minChartSize+layout.PanelChrome, minChartSize/2+layout.PanelChrome)); got != want {
		t.Errorf("spend panel =\n%s\nwant\n%s", got, want)
	}
}

func TestSpendChartPlacesTheLineAndTheMonthLabelsAlongTheAxes(t *testing.T) {
	want := "╭─ SPEND ──────────────────────────────╮\n" +
		"│1.3k│⣀⣀⣀⡀   ⢀⣀⣀⡠⠤⠤⠤⠤⣀⣀⣀⣀⣀⡠⠤⠤⠤⠤⠤⢄⣀⣀⡀   │\n" +
		"│    │   ⠈⠉⠉⠉⠁                         │\n" +
		"│0.9k│                                 │\n" +
		"│    │                                 │\n" +
		"│0.4k│                                 │\n" +
		"│    │                                 │\n" +
		"│0.0k└─────────────────────────────────│\n" +
		"│    Mar   Apr  May   Jun   Jul   Aug  │\n" +
		"╰──────────────────────────────────────╯"

	got := uitest.Plain(spendPanel(spendHistory(117500, 112000, 119500, 115000, 120500, 113000), 40, 10))

	if got != want {
		t.Errorf("spend panel =\n%s\nwant\n%s", got, want)
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
		for _, title := range []string{"ACCOUNTS", "UPCOMING", "RECENT"} {
			if strings.Contains(line, "╭─ "+title+" ◂") {
				titles = append(titles, title)
			}
		}
	}

	return titles
}

func TestTabMovesTheFocusThroughThePanelsAndBack(t *testing.T) {
	m := New().Resize(180, 30).WithLedger(withUpcoming("a"))

	for i, want := range []string{"ACCOUNTS", "UPCOMING", "RECENT", "ACCOUNTS"} {
		if got := focusedTitles(press(m, slices.Repeat([]string{"tab"}, i)...)); !slices.Equal(got, []string{want}) {
			t.Errorf("after %d tabs the focus is on %v, want %s", i, got, want)
		}
	}

	if got := focusedTitles(press(m, "shift+tab")); !slices.Equal(got, []string{"RECENT"}) {
		t.Errorf("shift+tab from Accounts focuses %v, want Recent", got)
	}
}

func TestTabSkipsPanelsTheTerminalHasNoRoomFor(t *testing.T) {
	short := New().Resize(180, minAccountsHeight+upcomingHeight-1).WithLedger(withUpcoming("a"))
	narrow := New().Resize(80, 30).WithLedger(withUpcoming("a"))

	if got := focusedTitles(press(short, "tab")); !slices.Equal(got, []string{"RECENT"}) {
		t.Errorf("without Upcoming, tab focuses %v, want Recent", got)
	}

	if got := focusedTitles(press(narrow, "tab", "tab")); !slices.Equal(got, []string{"ACCOUNTS"}) {
		t.Errorf("without the right column, tab twice focuses %v, want Accounts", got)
	}
}

func TestTheFocusFallsBackToAccountsWhenItsPanelNoLongerFits(t *testing.T) {
	m := press(New().Resize(180, 30).WithLedger(withUpcoming("a")), "tab", "tab").Resize(80, 30)

	if got := focusedTitles(m); !slices.Equal(got, []string{"ACCOUNTS"}) {
		t.Errorf("focus on %v, want Accounts", got)
	}

	if row, _ := press(m, "down").Position(); row != 2 {
		t.Errorf("the arrows should drive Accounts again, row %d", row)
	}
}

func letters(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("entry %02d", i)
	}

	return names
}

func TestTheArrowsScrollTheFocusedUpcomingPanel(t *testing.T) {
	m := press(New().Resize(180, 20).WithLedger(withUpcoming(letters(12)...)), "tab") // 9 rows inside

	down := press(m, "down")
	if rows := upcomingRows(down); !strings.Contains(rows[0], "entry 01") || !strings.Contains(rows[8], "… 3 more") {
		t.Errorf("one down shows from the second entry:\n%s", strings.Join(rows, "\n"))
	}

	if row, total := down.Position(); row != 2 || total != 12 {
		t.Errorf("position = %d/%d, want 2/12", row, total)
	}

	if account, _ := down.Selected(); account.Name != "assets:cash" {
		t.Errorf("scrolling Upcoming moved the Accounts cursor to %s", account.Name)
	}

	end := press(m, "end", "down")
	if rows := upcomingRows(end); !strings.Contains(rows[0], "entry 03") || !strings.Contains(rows[8], "entry 11") {
		t.Errorf("end shows the last page without a more row:\n%s", strings.Join(rows, "\n"))
	}
}

func TestTheArrowsScrollTheFocusedRecentPanelAboveAndWithoutTheChart(t *testing.T) {
	for height, inside := range map[int]int{30: 30 - cardHeight - spendHeight - layout.PanelChrome, 18: 18 - cardHeight - layout.PanelChrome} {
		m := press(New().Resize(180, height).WithLedger(withTransactions(40)), "tab", "tab")
		if height < minAccountsHeight+upcomingHeight {
			m = press(m, "shift+tab") // no Upcoming to pass
		}

		rows := recentRows(press(m, "end"))
		if len(rows) != inside || strings.Contains(rows[inside-1], "more") || !strings.Contains(rows[inside-1], "Shop 00") {
			t.Errorf("height %d: end should show the oldest %d entries:\n%s", height, inside, strings.Join(rows, "\n"))
		}
	}
}

func TestAScrolledPanelStaysOnItsLastPageWhenTheLedgerShrinks(t *testing.T) {
	m := press(New().Resize(180, 20).WithLedger(withUpcoming(letters(12)...)), "tab", "end").WithLedger(withUpcoming(letters(10)...))

	if rows := upcomingRows(m); !strings.Contains(rows[0], "entry 01") || !strings.Contains(rows[8], "entry 09") {
		t.Errorf("rows:\n%s", strings.Join(rows, "\n"))
	}

	if row, _ := m.Position(); row != 2 {
		t.Errorf("position row = %d, want 2", row)
	}
}

func TestAnEmptyFocusedPanelHasNoPosition(t *testing.T) {
	if row, total := press(New().Resize(180, 30).WithLedger(withUpcoming()), "tab").Position(); row != 0 || total != 0 {
		t.Errorf("position = %d/%d, want 0/0", row, total)
	}
}
