package ui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

// fakeReporter builds a report whose single row names the period it was asked for.
type fakeReporter struct {
	periods *[]domain.Period
	err     error
}

func (f fakeReporter) Report(_ context.Context, period domain.Period) (domain.Report, error) {
	*f.periods = append(*f.periods, period)
	return domain.Report{Period: period, IncomeStatement: []domain.ReportRow{
		{Kind: domain.RowLeaf, Label: "for " + period.String()},
	}}, f.err
}

var regexpPosition = regexp.MustCompile(`\b\d+/\d+\b`)

var keyCodes = map[string]rune{
	"up": tea.KeyUp, "down": tea.KeyDown, "home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
}

func startedWithReports(t *testing.T, reporter fakeReporter) Model {
	t.Helper()
	model := New(journalPath, Ports{
		Loader: fakeLoader{}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil), Reports: reporter,
	})
	cmd := model.Init()
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, next := updated.Update(cmd())
	return deliver(updated.(Model), next)
}

// deliver feeds the messages a command produces back into the model until nothing is left.
func deliver(m Model, cmd tea.Cmd) Model {
	for cmd != nil {
		msg := cmd()
		if _, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, c := range msg.(tea.BatchMsg) {
				m = deliver(m, c)
			}
			return m
		}
		updated, next := m.Update(msg)
		m, cmd = updated.(Model), next
	}
	return m
}

func TestTheReportLoadsWithTheLedgerAndShowsOnTheReportsTab(t *testing.T) {
	var periods []domain.Period
	model := startedWithReports(t, fakeReporter{periods: &periods})

	reports, _ := press(model, "5")

	if content := uitest.Plain(reports.View().Content); !strings.Contains(content, "for month to date") || !strings.Contains(content, "BALANCE SHEET") {
		t.Errorf("Reports tab:\n%s", content)
	}
	if len(periods) != 1 {
		t.Errorf("the report was built %d times, want once", len(periods))
	}
}

func TestPCyclesTheReportsPeriodAndKeepsItAcrossViews(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")

	model, cmd := press(model, "p")
	model = deliver(model, cmd)
	away, _ := press(model, "3")
	back, _ := press(away, "5")

	if content := uitest.Plain(back.View().Content); !strings.Contains(content, "quarter to date") || !strings.Contains(content, "for quarter to date") {
		t.Errorf("the period survives a view switch:\n%s", content)
	}
}

func TestPDoesNothingOutsideReports(t *testing.T) {
	var periods []domain.Period
	model := startedWithReports(t, fakeReporter{periods: &periods})

	_, cmd := press(model, "p")

	if cmd != nil || len(periods) != 1 {
		t.Errorf("p on the Dashboard must not reload reports; built %d times", len(periods))
	}
}

func TestAStaleReportIsDropped(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")
	model, _ = press(model, "p")

	stale, _ := model.Update(reportLoadedMsg{period: domain.MonthToDate, report: domain.Report{
		IncomeStatement: []domain.ReportRow{{Kind: domain.RowLeaf, Label: "STALE"}},
	}})

	if strings.Contains(uitest.Plain(stale.(Model).View().Content), "STALE") {
		t.Errorf("a report for another period must not be shown")
	}
}

func TestAFailedReportKeepsTheOldOneAndFlagsTheError(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")

	failed, _ := model.Update(reportLoadedMsg{period: domain.MonthToDate, err: errors.New("hledger bal: boom")})

	content := uitest.Plain(failed.(Model).View().Content)
	if !strings.Contains(content, "for month to date") || !strings.Contains(content, "✗ error") {
		t.Errorf("stale report stays and the error shows:\n%s", content)
	}
}

func TestAReportOfANewPeriodThatFailsSaysWhyInTheView(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")
	model, _ = press(model, "p")

	failed, _ := model.Update(reportLoadedMsg{period: domain.QuarterToDate, err: errors.New("hledger bal: boom")})

	content := uitest.Plain(failed.(Model).View().Content)
	if strings.Contains(content, "for month to date") || !strings.Contains(content, "hledger bal: boom") ||
		!strings.Contains(content, "✗ error") {
		t.Errorf("view:\n%s", content)
	}
}

func TestArrowsScrollTheReportsAndOtherViewsIgnoreThem(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")

	for _, key := range []string{"down", "end", "home", "pgdown", "pgup", "up"} {
		moved, _ := model.Update(tea.KeyPressMsg{Code: keyCodes[key]})
		if strings.Contains(lastLine(moved.(Model)), "/") && strings.Contains(lastLine(moved.(Model)), "1/") {
			t.Errorf("%s: Reports has no position counter", key)
		}
	}
}

func TestReportsHaveNoPositionCounter(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")

	if got := lastLine(model); regexpPosition.MatchString(got) {
		t.Errorf("status bar = %q, want no n/total on a view without a cursor", got)
	}
}

func TestReportsFollowTheTerminalSize(t *testing.T) {
	var periods []domain.Period
	model, _ := press(startedWithReports(t, fakeReporter{periods: &periods}), "5")

	resizedModel := resized(model, 160, 50)

	rows := strings.Split(uitest.Plain(resizedModel.View().Content), "\n")
	if len(rows) != 50 || !strings.HasPrefix(rows[len(rows)-2], "╰") {
		t.Errorf("view has %d rows and the panels end at %q, want 50 rows with the panels' bottom border above the status bar", len(rows), rows[len(rows)-2])
	}
}

func TestWithoutAReporterTheReportsTabStillRenders(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "5")

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "BALANCE SHEET") {
		t.Errorf("Reports tab:\n%s", content)
	}
}
