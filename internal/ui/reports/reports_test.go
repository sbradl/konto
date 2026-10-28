package reports

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
)

var euro = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

func eur(mantissa int64) domain.MultiAmount {
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}}
}

var escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

func sample() domain.Report {
	return domain.Report{
		Period: domain.MonthToDate,
		BalanceSheet: []domain.ReportRow{
			{Kind: domain.RowHeader, Label: "Assets"},
			{Kind: domain.RowLeaf, Label: "assets:checking", Amount: eur(1324000), Delta: new(eur(135000))},
			{Kind: domain.RowLeaf, Label: "assets:savings", Amount: eur(1350000), Delta: new(eur(0))},
			{Kind: domain.RowHeader, Label: "Liabilities"},
			{Kind: domain.RowLeaf, Label: "liabilities:creditcard", Amount: eur(-14000), Delta: new(eur(-3500))},
			{Kind: domain.RowGrandTotal, Label: "Net worth", Amount: eur(2660000), Delta: new(eur(131500))},
		},
		IncomeStatement: []domain.ReportRow{
			{Kind: domain.RowHeader, Label: "Income"},
			{Kind: domain.RowLeaf, Label: "income:salary", Amount: eur(300000)},
			{Kind: domain.RowGrandTotal, Label: "Net", Amount: eur(181500)},
		},
	}
}

func loaded() Model { return New().Resize(180, 30).WithReport(sample()) }

func lines(m Model) []string { return strings.Split(plain(m.View()), "\n") }

func TestPeriodBarNamesThePeriodAndCyclesWithNext(t *testing.T) {
	m := New().Resize(180, 40)
	if !strings.Contains(plain(m.View()), "month to date") {
		t.Errorf("a new Reports view starts on month to date:\n%s", plain(m.View()))
	}

	for _, want := range []domain.Period{domain.QuarterToDate, domain.YearToDate, domain.AllTime, domain.MonthToDate} {
		m = m.NextPeriod()
		if m.Period() != want {
			t.Errorf("period = %v, want %v", m.Period(), want)
		}
	}
}

func TestTwoPanelsShareTheScreenRowAndFillTheSize(t *testing.T) {
	got := lines(loaded())

	if len(got) != 30 {
		t.Errorf("view has %d rows, want 30", len(got))
	}
	if !strings.Contains(got[1], "╭─ BALANCE SHEET ") || !strings.Contains(got[1], "╭─ INCOME STATEMENT ") {
		t.Errorf("row 1 = %q, want both panel titles", got[1])
	}
	for _, line := range got {
		if w := lipgloss.Width(line); w > 180 {
			t.Errorf("line is %d wide: %q", w, line)
		}
	}
}

func TestBalanceSheetShowsBalanceAndDelta(t *testing.T) {
	view := plain(loaded().View())

	for _, want := range []string{"Δ period", "assets:checking", "13.240,00 EUR", "1.350,00 EUR", "Net worth", "26.600,00 EUR"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestBalanceSheetWithoutDeltasHasNoDeltaColumn(t *testing.T) {
	report := sample()
	for i := range report.BalanceSheet {
		report.BalanceSheet[i].Delta = nil
	}

	view := plain(New().Resize(180, 30).WithReport(report).View())

	if strings.Contains(view, "Δ period") || !strings.Contains(view, "13.240,00 EUR") {
		t.Errorf("no Δ column under all time:\n%s", view)
	}
}

func TestStylesFollowRowKindsAndSigns(t *testing.T) {
	view := loaded().View()

	for name, want := range map[string]string{
		"header cyan bold":     "\x1b[1;36mAssets",
		"negative balance red": "\x1b[31m-140,00 EUR",
		"positive delta green": "\x1b[32m1.350,00 EUR",
		"negative delta red":   "\x1b[31m-35,00 EUR",
		"zero delta dim":       "\x1b[90m0\x1b[m",
		"grand total emphasis": "\x1b[1;97mNet worth",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("%s: want %q in the view", name, want)
		}
	}
}

func TestTheIncomeStatementHasAccountAndAmountOnly(t *testing.T) {
	view := plain(loaded().View())
	_, statement, _ := strings.Cut(view, "INCOME STATEMENT")

	if strings.Contains(statement, "%") || !strings.Contains(statement, "3.000,00 EUR") {
		t.Errorf("statement:\n%s", statement)
	}
}

func many(n int) domain.Report {
	report := sample()
	for i := range n {
		report.IncomeStatement = append(report.IncomeStatement[:len(report.IncomeStatement)-1],
			domain.ReportRow{Kind: domain.RowLeaf, Label: fmt.Sprintf("expenses:cat%02d", i), Amount: eur(100)},
			domain.ReportRow{Kind: domain.RowGrandTotal, Label: "Net", Amount: eur(0)})
	}
	return report
}

func TestOverflowSaysHowManyRowsAreHidden(t *testing.T) {
	view := plain(New().Resize(100, 20).WithReport(many(40)).View())

	if !strings.Contains(view, "… ") || !strings.Contains(view, " more") || strings.Contains(view, "expenses:cat39") {
		t.Errorf("the statement should be cut with a … N more row:\n%s", view)
	}
}

func TestScrollingMovesBothTablesAndTheEndsShowTheLastRows(t *testing.T) {
	m := New().Resize(100, 20).WithReport(many(40))

	scrolled := plain(m.Move("down").Move("down").Move("down").View())
	if strings.Contains(scrolled, "Income") && strings.Contains(strings.Split(scrolled, "\n")[2], "Income") {
		t.Errorf("after 3 downs the first rows should be scrolled off:\n%s", scrolled)
	}
	if !strings.Contains(scrolled, "expenses:cat02") {
		t.Errorf("scrolling reveals later rows:\n%s", scrolled)
	}

	end := plain(m.Move("end").View())
	if !strings.Contains(end, "Net") || strings.Contains(end, "more") {
		t.Errorf("End shows the last row, nothing hidden below:\n%s", end)
	}
	top := plain(m.Move("end").Move("home").View())
	if !strings.Contains(top, "Assets") || !strings.Contains(top, "Income") {
		t.Errorf("Home shows the first rows:\n%s", top)
	}
}

func TestPagingAndLimits(t *testing.T) {
	m := New().Resize(100, 20).WithReport(many(40))

	paged := m.Move("pgdown")
	if paged.offset == 0 {
		t.Errorf("pgdown should scroll")
	}
	if back := paged.Move("pgup"); back.offset != 0 {
		t.Errorf("pgup should return, offset %d", back.offset)
	}
	if up := m.Move("up"); up.offset != 0 {
		t.Errorf("cannot scroll above the top")
	}
	if past := m.Move("end").Move("down"); past.offset != m.Move("end").offset {
		t.Errorf("cannot scroll below the end")
	}
	if same := m.Move("x"); same.offset != 0 {
		t.Errorf("other keys do nothing")
	}
}

func TestShortReportsDoNotScroll(t *testing.T) {
	if m := loaded().Move("down").Move("end"); m.offset != 0 {
		t.Errorf("offset = %d, want 0 for a report that fits", m.offset)
	}
}

func TestNoRowIsHighlighted(t *testing.T) {
	if view := loaded().Move("down").View(); strings.Contains(view, "100m") {
		t.Errorf("Reports has no cursor:\n%q", view)
	}
}

func TestAnEmptyReportRendersEmptyPanels(t *testing.T) {
	view := plain(New().Resize(180, 30).WithReport(domain.Report{Period: domain.MonthToDate}).View())

	if !strings.Contains(view, "BALANCE SHEET") || !strings.Contains(view, "INCOME STATEMENT") {
		t.Errorf("empty tables still have their panels:\n%s", view)
	}
}

func TestBeforeTheFirstReportThePanelsAreEmptyToo(t *testing.T) {
	if view := plain(New().Resize(180, 30).View()); !strings.Contains(view, "BALANCE SHEET") {
		t.Errorf("view:\n%s", view)
	}
}

func TestTheBarSaysLoadingUntilTheFirstReportArrives(t *testing.T) {
	loading := plain(New().Resize(180, 30).View())
	loaded := plain(New().Resize(180, 30).WithReport(domain.Report{Period: domain.MonthToDate}).View())

	if !strings.Contains(loading, "loading the reports") {
		t.Errorf("before the first report:\n%s", loading)
	}
	if strings.Contains(loaded, "loading") {
		t.Errorf("after the first report:\n%s", loaded)
	}
}

func TestTheNextPeriodDropsTheOldNumbersUntilItsReportArrives(t *testing.T) {
	view := plain(loaded().Move("end").NextPeriod().View())

	if !strings.Contains(view, "loading the reports") || strings.Contains(view, "assets:checking") {
		t.Errorf("view:\n%s", view)
	}
	if next := loaded().Move("end").NextPeriod(); next.offset != 0 {
		t.Errorf("offset %d, want the next period's tables from the top", next.offset)
	}
}

func TestAReportThatCannotBeBuiltSaysWhyUntilOneIs(t *testing.T) {
	failed := New().Resize(180, 30).WithFailure(errors.New("hledger bal: boom"))
	refreshed := loaded().WithFailure(errors.New("hledger bal: boom"))

	if view := plain(failed.View()); !strings.Contains(view, "hledger bal: boom") || strings.Contains(view, "loading") {
		t.Errorf("failed first build:\n%s", view)
	}
	if view := plain(refreshed.View()); strings.Contains(view, "boom") || !strings.Contains(view, "assets:checking") {
		t.Errorf("a failed rebuild keeps the report on screen:\n%s", view)
	}
	if view := plain(failed.WithReport(sample()).View()); strings.Contains(view, "boom") {
		t.Errorf("a built report clears the failure:\n%s", view)
	}
	if view := plain(failed.NextPeriod().View()); strings.Contains(view, "boom") {
		t.Errorf("the next period clears the failure:\n%s", view)
	}
}

func TestLongLabelsAreCutToKeepTheAmounts(t *testing.T) {
	report := sample()
	report.BalanceSheet[1].Label = strings.Repeat("long:", 30)

	for line := range strings.SplitSeq(plain(New().Resize(100, 30).WithReport(report).View()), "\n") {
		if lipgloss.Width(line) > 100 {
			t.Errorf("line too wide: %q", line)
		}
	}
}

func TestForeignCurrenciesRenderAsTheyAre(t *testing.T) {
	report := sample()
	report.BalanceSheet[1].Amount = domain.MultiAmount{
		{Commodity: "PLN", Mantissa: 100, Places: 2, Style: euro}, {Commodity: "CZK", Mantissa: 200, Places: 2, Style: euro},
	}

	if view := plain(New().Resize(180, 30).WithReport(report).View()); !strings.Contains(view, "PLN") {
		t.Errorf("view:\n%s", view)
	}
}

func TestEndShowsTheLastRowOfABalanceSheetWhoseDeltaTitleTakesALine(t *testing.T) {
	m := New().Resize(100, 9).WithReport(sample()).Move("end")

	if view := plain(m.View()); !strings.Contains(view, "Net worth") || strings.Contains(view, "more") {
		t.Errorf("after End:\n%s", view)
	}
}

func TestRowsAndColumnsSitWhereTheLayoutPutsThem(t *testing.T) {
	m := New().Resize(100, 9).WithReport(sample())
	top := " period  month to date\n" +
		"╭─ BALANCE SHEET ────────────────────────────────╮╭─ INCOME STATEMENT ─────────────────────────────╮\n" +
		"│                                       Δ period ││ Income                                         │\n"
	bottom := "╰────────────────────────────────────────────────╯╰────────────────────────────────────────────────╯"
	wantTop := top +
		"│ Assets                                         ││  income:salary                   3.000,00 EUR  │\n" +
		"│  assets:checking  13.240,00 EUR  1.350,00 EUR  ││ Net                              1.815,00 EUR  │\n" +
		"│  assets:savings   13.500,00 EUR             0  ││                                                │\n" +
		"│ Liabilities                                    ││                                                │\n" +
		"│ … 2 more                                       ││                                                │\n" + bottom
	wantEnd := top +
		"│  assets:checking  13.240,00 EUR  1.350,00 EUR  ││  income:salary                   3.000,00 EUR  │\n" +
		"│  assets:savings   13.500,00 EUR             0  ││ Net                              1.815,00 EUR  │\n" +
		"│ Liabilities                                    ││                                                │\n" +
		"│  liabilities:cred   -140,00 EUR    -35,00 EUR  ││                                                │\n" +
		"│ Net worth         26.600,00 EUR  1.315,00 EUR  ││                                                │\n" + bottom

	if got := plain(m.View()); got != wantTop {
		t.Errorf("top =\n%s\nwant\n%s", got, wantTop)
	}
	if got := plain(m.Move("end").View()); got != wantEnd {
		t.Errorf("end =\n%s\nwant\n%s", got, wantEnd)
	}
}

func TestNarrowPanelsCutLabelsAndLetTheAmountsOverflowTheirColumns(t *testing.T) {
	want := " period  month to date\n" +
		"╭─ BALANCE SHEET ─────╮╭─ INCOME STATEMENT ──╮\n" +
		"│            Δ period ││ Income              │\n" +
		"│ Assets              ││  inco 3.000,00 EUR  │\n" +
		"│  assets:checking 13.││ Net   1.815,00 EUR  │\n" +
		"│  assets:savings 13.5││                     │\n" +
		"│ Liabilities         ││                     │\n" +
		"│  liabilities:creditc││                     │\n" +
		"│ Net worth 26.600,00 ││                     │\n" +
		"│                     ││                     │\n" +
		"│                     ││                     │\n" +
		"│                     ││                     │\n" +
		"│                     ││                     │\n" +
		"╰─────────────────────╯╰─────────────────────╯"

	if got := plain(New().Resize(46, 14).WithReport(sample()).View()); got != want {
		t.Errorf("view =\n%s\nwant\n%s", got, want)
	}
}

func TestATableWithoutDeltasUsesEveryRowBelowTheBorder(t *testing.T) {
	report := domain.Report{Period: domain.MonthToDate}
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		report.IncomeStatement = append(report.IncomeStatement,
			domain.ReportRow{Kind: domain.RowLeaf, Label: name, Amount: eur(100)})
	}

	view := plain(New().Resize(100, 7).WithReport(report).View())

	if !strings.Contains(view, "… 3 more") {
		t.Errorf("a four-row panel showing six rows gives its last row to the overflow line:\n%s", view)
	}
}

func TestKeyPMovesToTheNextPeriodAndAsksForItsReport(t *testing.T) {
	m := New().Resize(180, 40)

	next, action := m.Key("p")
	if action != LoadReport || next.Period() != domain.QuarterToDate {
		t.Errorf("p: action = %v, period = %v; want the quarter's report asked for", action, next.Period())
	}
	if _, action := m.Key("down"); action != NoAction {
		t.Errorf("down: action = %v, want none", action)
	}
}
