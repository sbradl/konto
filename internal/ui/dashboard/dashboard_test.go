package dashboard

import (
	"fmt"
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

func lines(m Model) []string { return strings.Split(plain(m.accountsPanel()), "\n") }

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
	raw := strings.Split(loaded(20).accountsPanel(), "\n")

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
	if raw := strings.Split(m.accountsPanel(), "\n"); !strings.Contains(raw[3], ";100m") {
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

func viewLines(m Model) []string { return strings.Split(plain(m.View()), "\n") }

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
}

func TestNetIncomeIsGreenOrRedBySign(t *testing.T) {
	positive := New().Resize(180, 20).WithLedger(ledgerFigures()).View()
	negativeLedger := ledgerFigures()
	negativeLedger.Month.Income = eur(0)
	negative := New().Resize(180, 20).WithLedger(negativeLedger).View()

	if !strings.Contains(positive, "\x1b[32m1.815,00 EUR") {
		t.Errorf("positive net income should be green: %q", strings.Split(positive, "\n")[1])
	}
	if !strings.Contains(negative, "\x1b[31m-1.185,00 EUR") {
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
	var rows []string
	inside := false
	for _, line := range viewLines(m) {
		switch {
		case strings.Contains(line, "╭─ RECENT "):
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

	if !strings.Contains(view, "\x1b[31m-1,00 EUR") || !strings.Contains(view, "\x1b[32m1,00 EUR") {
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
		if w := lipgloss.Width(line); w != 180 {
			t.Errorf("row is %d wide, want 180: %q", w, line)
		}
	}
}

func TestRecentCutsLongDescriptionsToKeepTheColumns(t *testing.T) {
	ledger := withTransactions(1)
	ledger.Transactions[0].Description = strings.Repeat("long ", 40)

	row := recentRows(New().Resize(110, 24).WithLedger(ledger))[0]

	if lipgloss.Width(row) != 110 || !strings.HasSuffix(strings.TrimRight(row, " │"), "-10,00 EUR") {
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
	var rows []string
	inside := false
	for _, line := range viewLines(m) {
		switch {
		case strings.Contains(line, "╭─ UPCOMING "):
			inside = true
		case inside && strings.Contains(line, "╰"):
			return rows
		case inside:
			rows = append(rows, line)
		}
	}
	return rows
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

func TestUpcomingListsTheNextOccurrencesSoonestFirstUnderRecent(t *testing.T) {
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

	_, panel, _ := strings.Cut(strings.Join(rows, ""), "││")
	if len(rows) != upcomingHeight-panelChrome || strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(panel, "││", ""), "│", "")) != "" {
		t.Errorf("rows = %q, want an empty panel", rows)
	}
}

func TestUpcomingSaysHowManyOccurrencesDontFit(t *testing.T) {
	rows := upcomingRows(New().Resize(180, 20).WithLedger(withUpcoming("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l")))

	if !strings.Contains(rows[7], "… 5 more") {
		t.Errorf("last row = %q, want seven listed then … 5 more", rows[3])
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

func TestSpendPanelIsEmptyWithoutHistoryOrRoom(t *testing.T) {
	for name, panel := range map[string]string{
		"no history": spendPanel(nil, 60, 10),
		"too narrow": spendPanel(spendHistory(100), 8, 10),
		"too short":  spendPanel(spendHistory(100), 60, 5),
	} {
		if strings.ContainsAny(plain(panel), "⠁⠂⠄⡀⢀⠈⠐⠠⣀⠤⠉k") || !strings.Contains(plain(panel), "SPEND") {
			t.Errorf("%s: want an empty SPEND panel:\n%s", name, plain(panel))
		}
	}
}

func TestUpcomingTakesTheWholeBandOnANarrowScreen(t *testing.T) {
	ledger := withUpcoming("a")
	ledger.Spend = spendHistory(100, 200)

	view := strings.Join(viewLines(New().Resize(120, 30).WithLedger(ledger)), "\n")

	if !strings.Contains(view, "UPCOMING") || strings.Contains(view, "SPEND") {
		t.Errorf("a narrow screen drops the chart:\n%s", view)
	}
}

func TestSpendChartAppearsAtExactlyTheMinimumBandWidth(t *testing.T) {
	ledger := withUpcoming("a")
	ledger.Spend = spendHistory(100, 200)

	view := strings.Join(viewLines(New().Resize(minAccountsWidth+2*minBandWidth, 30).WithLedger(ledger)), "\n")

	if !strings.Contains(view, "SPEND") {
		t.Errorf("two minimum-width panels fit, so the chart shows:\n%s", view)
	}
}
