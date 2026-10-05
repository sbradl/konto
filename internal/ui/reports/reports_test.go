package reports

import (
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
