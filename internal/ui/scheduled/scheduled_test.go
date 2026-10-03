package scheduled

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

var escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func plain(s string) string { return escapes.ReplaceAllString(s, "") }

func date(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

func rule(line int, period, description string, extra ...string) domain.PeriodicRule {
	lines := append([]string{"~ " + period + "  " + description}, extra...)
	return domain.PeriodicRule{Period: period, Description: description, Lines: lines, Pos: domain.SourcePos{File: "j", Line: line}}
}

func occurrences(line int, amount int64, dates ...string) []domain.Transaction {
	list := make([]domain.Transaction, 0, len(dates))
	for _, d := range dates {
		list = append(list, domain.Transaction{Date: date(d), Source: domain.SourcePos{File: "j", Line: line}, Postings: []domain.Posting{
			{Account: "expenses:x", Amounts: eur(amount)}, {Account: "assets:checking", Amounts: eur(-amount)},
		}})
	}
	return list
}

func household() domain.Ledger {
	ledger := domain.Ledger{Rules: []domain.PeriodicRule{
		rule(10, "monthly from 2026-10-05", "Streaming  ; shared account", "    ; note", "    expenses:streaming  13,00 EUR", "    assets:checking"),
		rule(20, "monthly from 2026-10-01", "Vermieter Mueller", "    expenses:rent  900,00 EUR", "    assets:checking"),
		rule(30, "2026-01-05", "Old once-off", "    expenses:rent  5,00 EUR", "    assets:checking"),
	}}
	ledger.Rules[0].Description = "Streaming"
	ledger.Rules[0].Lines[0] = "~ monthly from 2026-10-05  Streaming  ; shared account"
	ledger.Upcoming = append(occurrences(10, 1300, "2026-10-05", "2026-11-05"), occurrences(20, 90000,
		"2026-10-01", "2026-11-01", "2026-12-01", "2027-01-01", "2027-02-01", "2027-03-01", "2027-04-01")...)
	return ledger
}

func loaded() Model { return New().Resize(180, 30).WithLedger(household()) }

func lines(m Model) []string { return strings.Split(plain(m.View()), "\n") }

func TestRulesListSortedByNextOccurrenceWithCadenceAndStatus(t *testing.T) {
	got := lines(loaded())

	if !strings.Contains(got[0], "╭─ RULES ") {
		t.Errorf("first row = %q, want the RULES panel", got[0])
	}
	for i, want := range []string{"2026-10-01  Vermieter Mueller", "2026-10-05  Streaming", "Old once-off"} {
		if !strings.Contains(got[i+1], want) {
			t.Errorf("row %d = %q, want it to hold %q", i, got[i+1], want)
		}
	}
	if !strings.Contains(got[1], "monthly from 2026-10-01") || !strings.Contains(got[1], "upcoming") {
		t.Errorf("row = %q, want the raw cadence and an upcoming status", got[1])
	}
	if !strings.Contains(got[3], "none in the next 12 months") {
		t.Errorf("row = %q, want the none-in-12-months status", got[3])
	}
}

func TestTheFirstRuleIsSelectedAndArrowsMoveTheCursorWithinLimits(t *testing.T) {
	m := loaded()
	if row, total := m.Position(); row != 1 || total != 3 {
		t.Errorf("position = %d/%d, want 1/3", row, total)
	}
	if !strings.Contains(m.View(), "\x1b[1;97;100m") && !strings.Contains(m.View(), "100m") {
		t.Errorf("the selected row should be highlighted")
	}

	for _, step := range []struct {
		key  string
		want int
	}{{"down", 2}, {"end", 3}, {"pgdown", 3}, {"up", 2}, {"home", 1}, {"pgup", 1}, {"x", 1}} {
		m = m.Move(step.key)
		if row, _ := m.Position(); row != step.want {
			t.Errorf("after %s the cursor is on row %d, want %d", step.key, row, step.want)
		}
	}
}

func TestEmptyViewHasNoPositionAndSaysHowToAddARule(t *testing.T) {
	m := New().Resize(180, 30).WithLedger(domain.Ledger{})

	if row, total := m.Position(); row != 0 || total != 0 {
		t.Errorf("position = %d/%d, want none", row, total)
	}
	if view := plain(m.View()); !strings.Contains(view, "press n to add a rule") {
		t.Errorf("empty state should hint at n:\n%s", view)
	}
	if got := m.Move("down"); got.cursor != 0 {
		t.Errorf("nothing to move over")
	}
}

func TestEntryShowsTheBlockAsWrittenWithTheRuleLineMagentaAndCommentsDim(t *testing.T) {
	m := loaded()
	view := plain(m.View())
	if !strings.Contains(view, "~ monthly from 2026-10-01  Vermieter Mueller") || !strings.Contains(view, "expenses:rent  900,00 EUR") {
		t.Errorf("entry panel should show the first rule as written:\n%s", view)
	}
	if !strings.Contains(m.View(), "\x1b[35m~ monthly from 2026-10-01  Vermieter Mueller") {
		t.Errorf("the ~ line should be magenta")
	}

	streaming := m.Move("down").View()
	if !strings.Contains(streaming, "\x1b[90m; shared account") || !strings.Contains(streaming, "\x1b[90m    ; note") {
		t.Errorf("comments should be dim:\n%q", streaming)
	}
}

func TestOccurrencesListsTheNextSixOfTheSelectedRule(t *testing.T) {
	view := plain(loaded().View())
	_, occurrencesPanel, _ := strings.Cut(view, "OCCURRENCES")

	for _, want := range []string{"2026-10-01", "2026-11-01", "2026-12-01", "2027-01-01", "2027-02-01", "2027-03-01"} {
		if !strings.Contains(occurrencesPanel, want) {
			t.Errorf("occurrences lack %s", want)
		}
	}
	if strings.Contains(occurrencesPanel, "2027-04-01") {
		t.Errorf("only the next six are listed")
	}
	next := plain(loaded().Move("down").View())
	_, panel, _ := strings.Cut(next, "OCCURRENCES")
	if !strings.Contains(panel, "2026-10-05") {
		t.Errorf("occurrences follow the cursor:\n%s", panel)
	}
	none := plain(loaded().Move("end").View())
	if _, panel, _ := strings.Cut(none, "OCCURRENCES"); !strings.Contains(panel, "none in the next 12 months") {
		t.Errorf("a rule without occurrences says so:\n%s", panel)
	}
}

func TestAnnualImpactListsEveryRuleAndTheTotalColoured(t *testing.T) {
	view := loaded().View()
	text := plain(view)

	_, impact, _ := strings.Cut(text, "ANNUAL IMPACT")
	for _, want := range []string{"Vermieter Mueller", "-6.300,00 EUR", "Streaming", "-26,00 EUR", "Total", "-6.326,00 EUR"} {
		if !strings.Contains(impact, want) {
			t.Errorf("annual impact lacks %q:\n%s", want, impact)
		}
	}
	if !strings.Contains(view, "\x1b[31m-6.300,00 EUR") || !strings.Contains(view, "\x1b[90m0") {
		t.Errorf("negative impact should be red and zero dim:\n%q", view)
	}
}

func TestPositiveImpactIsGreen(t *testing.T) {
	ledger := domain.Ledger{
		Rules:    []domain.PeriodicRule{rule(1, "monthly", "Salary")},
		Upcoming: occurrences(1, -300000, "2026-10-25"),
	}

	if view := New().Resize(180, 30).WithLedger(ledger).View(); !strings.Contains(view, "\x1b[32m3.000,00 EUR") {
		t.Errorf("a positive impact is green:\n%q", view)
	}
}

func TestTheCursorStaysInRangeWhenTheRulesShrink(t *testing.T) {
	m := loaded().Move("end")

	shrunk := m.WithLedger(domain.Ledger{Rules: household().Rules[:1], Upcoming: household().Upcoming})

	if row, total := shrunk.Position(); row != 1 || total != 1 {
		t.Errorf("position = %d/%d, want 1/1", row, total)
	}
}

func TestLongRuleListsScrollWithTheCursorAndPageByTheListHeight(t *testing.T) {
	ledger := domain.Ledger{}
	for i := range 40 {
		ledger.Rules = append(ledger.Rules, rule(i+1, "monthly", fmt.Sprintf("Rule %02d", i)))
	}
	m := New().Resize(180, 30).WithLedger(ledger)

	end, _, _ := strings.Cut(plain(m.Move("end").View()), "ENTRY")
	if !strings.Contains(end, "Rule 39") || strings.Contains(end, "Rule 00") {
		t.Errorf("End shows the last rule and scrolls the first away:\n%s", end)
	}
	if paged := m.Move("pgdown"); paged.cursor <= 1 {
		t.Errorf("pgdown should jump a page, cursor %d", paged.cursor)
	}
	down := m
	for range 20 {
		down = down.Move("down")
	}
	if !strings.Contains(plain(down.View()), "Rule 20") {
		t.Errorf("the selected row is always visible")
	}
	if back := down.Move("home"); back.offset != 0 {
		t.Errorf("home scrolls back to the top")
	}
}

func TestViewFillsTheSizeAndNeverGetsWiderThanTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{100, 27}, {120, 37}, {180, 57}, {240, 67}} {
		view := New().Resize(size[0], size[1]).WithLedger(household()).View()
		got := strings.Split(plain(view), "\n")
		if len(got) != size[1] {
			t.Errorf("%v: %d rows, want %d", size, len(got), size[1])
		}
		for _, line := range got {
			if lipgloss.Width(line) > size[0] {
				t.Errorf("%v: line is %d wide: %q", size, lipgloss.Width(line), line)
			}
		}
	}
}

func TestBeforeAnythingLoadedThePanelsAreEmpty(t *testing.T) {
	if view := plain(New().Resize(180, 30).View()); !strings.Contains(view, "RULES") {
		t.Errorf("view:\n%s", view)
	}
}
