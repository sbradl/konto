package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

func issue(severity domain.Severity, check, file string, line int) domain.CheckIssue {
	return domain.CheckIssue{Check: check, Severity: severity, File: file, Line: line, Message: check + " failed"}
}

// withRun is a started model whose check run holds the given issues.
func withRun(t *testing.T, calls *[]editorCall, issues ...domain.CheckIssue) Model {
	t.Helper()
	model := New(journalPath, Ports{Loader: fakeLoader{}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(calls)})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(ledgerLoadedMsg{run: domain.CheckRun{Total: 12, Issues: issues}})
	return updated.(Model)
}

func TestLoadingTheLedgerRunsTheChecks(t *testing.T) {
	model := started(t, fakeLoader{})

	if model.checks.run == nil || model.checks.run.Total != 12 {
		t.Errorf("run = %+v, want the fake run of 12 checks", model.checks.run)
	}
}

func TestAFailedLoadRunsNoChecks(t *testing.T) {
	model := New(journalPath, Ports{Loader: fakeLoader{err: errors.New("boom")}, Checker: fakeChecker{err: errors.New("must not run")}, Creator: &fakeCreator{}, Editor: recordingEditor(nil)})

	msg, ok := model.load().(ledgerLoadedMsg)

	if !ok || msg.err == nil || !strings.Contains(msg.err.Error(), "boom") {
		t.Errorf("load = %+v, want the load error and no check run", msg)
	}
}

func TestAFailedCheckRunKeepsTheLastRunAndFlagsTheError(t *testing.T) {
	model := withRun(t, nil, issue(domain.SeverityWarn, "tags", "/j", 3))
	model.checker = fakeChecker{err: errors.New("hledger died")}

	updated, _ := model.Update(model.load())
	model = updated.(Model)

	if len(model.checks.run.Issues) != 1 || model.failure == nil {
		t.Errorf("run = %+v, failure = %v; want the old run and an error flag", model.checks.run, model.failure)
	}
	if got := lastLine(model); !strings.Contains(got, "✗ error") {
		t.Errorf("status bar = %q, want the error indicator", got)
	}
}

func TestStatusBarCountsAndColoursTheIssues(t *testing.T) {
	for name, tc := range map[string]struct {
		issues []domain.CheckIssue
		want   string
		escape string
	}{
		"clean":       {nil, "● clean", "\x1b[32;100m● clean"},
		"warnings":    {[]domain.CheckIssue{issue(domain.SeverityWarn, "a", "", 0)}, "● 1 warn", "\x1b[33;100m● 1 warn"},
		"errors only": {[]domain.CheckIssue{issue(domain.SeverityErr, "a", "", 0)}, "● 1 err   ", "\x1b[31;100m● 1 err"},
		"errors win": {[]domain.CheckIssue{
			issue(domain.SeverityWarn, "a", "", 0), issue(domain.SeverityErr, "b", "", 0), issue(domain.SeverityWarn, "c", "", 0),
		}, "● 1 err · 2 warn", "\x1b[31;100m● 1 err · 2 warn"},
	} {
		model := withRun(t, nil, tc.issues...)

		raw := strings.Split(model.View().Content, "\n")
		last := raw[len(raw)-1]
		if !strings.Contains(uitest.Plain(last), tc.want) || !strings.Contains(last, tc.escape) {
			t.Errorf("%s: status bar = %q, want %q drawn as %q", name, last, tc.want, tc.escape)
		}
	}
}

// A journal that failed to load has no check run: no indicator, and the panel says so.
func TestNoIndicatorWithoutACheckRun(t *testing.T) {
	model := started(t, fakeLoader{err: errors.New("hledger exploded")})

	if got := lastLine(model); strings.Contains(got, "●") {
		t.Errorf("status bar = %q, want no indicator", got)
	}
	panel, _ := press(model, "!")
	if content := uitest.Plain(panel.View().Content); !strings.Contains(content, "no checks have run yet") {
		t.Errorf("panel should say nothing ran:\n%s", content)
	}
}

func TestPanelListsEachIssueWithSeverityCheckLocationAndMessage(t *testing.T) {
	model, _ := press(withRun(t, nil, issue(domain.SeverityWarn, "accounts", "/data/main.journal", 10)), "!")

	content := uitest.Plain(model.View().Content)
	if !strings.Contains(content, " WARN  accounts  main.journal:10  accounts failed") {
		t.Errorf("panel lacks the issue row:\n%s", content)
	}
}

func TestPanelOmitsTheLocationOfIssuesWithoutOne(t *testing.T) {
	bare := domain.CheckIssue{Check: "tags", Severity: domain.SeverityWarn, Message: "no place to jump to"}
	model, _ := press(withRun(t, nil, bare), "!")

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, " WARN  tags  no place to jump to") {
		t.Errorf("panel =\n%s", content)
	}
}

func TestPanelSelectsWithArrowsAndStopsAtTheEnds(t *testing.T) {
	model, _ := press(withRun(t, nil,
		issue(domain.SeverityWarn, "one", "", 0), issue(domain.SeverityWarn, "two", "", 0)), "!")

	selected := func(m Model) string {
		for line := range strings.SplitSeq(m.View().Content, "\n") {
			if strings.Contains(line, ";1m WARN") {
				return uitest.Plain(line)
			}
		}
		return ""
	}
	if got := selected(model); !strings.Contains(got, "one") {
		t.Errorf("first row should start selected, got %q", got)
	}
	model = pressKey(pressKey(model, tea.KeyDown), tea.KeyDown)
	if got := selected(model); !strings.Contains(got, "two") {
		t.Errorf("↓ twice should stop on the last row, got %q", got)
	}
	model = pressKey(pressKey(pressKey(model, tea.KeyUp), tea.KeyUp), tea.KeyUp)
	if got := selected(model); !strings.Contains(got, "one") {
		t.Errorf("↑ should stop on the first row, got %q", got)
	}
}

func TestPanelScrollsToKeepTheSelectionVisible(t *testing.T) {
	issues := make([]domain.CheckIssue, 20)
	for i := range issues {
		issues[i] = issue(domain.SeverityWarn, "check"+string(rune('a'+i)), "", 0)
	}
	model, _ := press(withRun(t, nil, issues...), "!")
	model = resized(model, 100, 30) // half of 30 is 15 rows: a title and 14 issues

	if rows := strings.Count(uitest.Plain(model.popupBox()), "\n") + 1; rows != 15 {
		t.Fatalf("popup is %d rows, want 15", rows)
	}
	for i := range 19 {
		model = pressKey(model, tea.KeyDown)
		if !strings.Contains(strings.Join(strings.Split(model.View().Content, "\n"), "\n"), ";1m WARN  check"+string(rune('a'+i+1))) {
			t.Fatalf("after %d ↓ the selected row is not visible:\n%s", i+1, uitest.Plain(model.View().Content))
		}
	}
	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "checkt") {
		t.Errorf("the last issue should be visible after scrolling:\n%s", content)
	}
}

func TestEnterOpensTheSelectedIssueInTheEditor(t *testing.T) {
	var calls []editorCall
	model, _ := press(withRun(t, &calls, issue(domain.SeverityWarn, "accounts", "/data/main.journal", 10)), "!")

	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil || len(calls) != 1 || calls[0] != (editorCall{"/data/main.journal", 10}) {
		t.Errorf("cmd = %v, editor calls = %v; want the editor at main.journal:10", cmd, calls)
	}
}

func TestEnterDoesNothingOnAnIssueWithoutALocation(t *testing.T) {
	var calls []editorCall
	model, _ := press(withRun(t, &calls, issue(domain.SeverityWarn, "tags", "", 0)), "!")

	if _, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || len(calls) != 0 {
		t.Errorf("cmd = %v, calls = %v; want nothing", cmd, calls)
	}
}

func TestEnterDoesNothingInACleanPanel(t *testing.T) {
	model, _ := press(withRun(t, nil), "!")

	if _, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Errorf("cmd = %v, want nothing", cmd)
	}
}

func TestOtherKeysDoNothingInTheChecksPanel(t *testing.T) {
	model, _ := press(withRun(t, nil, issue(domain.SeverityWarn, "one", "", 0)), "!")

	if _, cmd := press(model, "x"); cmd != nil {
		t.Errorf("cmd = %v, want nothing", cmd)
	}
}

func TestArrowsDoNothingInTheKeysPopup(t *testing.T) {
	model, _ := press(withRun(t, nil, issue(domain.SeverityWarn, "one", "", 0)), "?")

	if got := pressKey(model, tea.KeyDown); got.checks.cursor.Row != 0 || got.popup != popupKeys {
		t.Errorf("cursor = %d, popup = %v; want the keys popup untouched", got.checks.cursor.Row, got.popup)
	}
}

func TestTheCursorStaysInRangeWhenIssuesDisappear(t *testing.T) {
	model, _ := press(withRun(t, nil,
		issue(domain.SeverityWarn, "one", "", 0), issue(domain.SeverityWarn, "two", "", 0)), "!")
	model = pressKey(model, tea.KeyDown)

	updated, _ := model.Update(ledgerLoadedMsg{run: domain.CheckRun{Total: 12}})

	if got := updated.(Model).checks.cursor.Row; got != 0 {
		t.Errorf("cursor = %d, want 0 once the issues are gone", got)
	}
}

func TestPanelShowsPathsInsideTheJournalDirectoryRelativeToIt(t *testing.T) {
	model, _ := press(withRun(t, nil,
		issue(domain.SeverityWarn, "a", "/data/sub/extra.journal", 4),
		issue(domain.SeverityWarn, "b", "/elsewhere/other.journal", 2)), "!")

	content := uitest.Plain(model.View().Content)
	for _, want := range []string{" a  sub/extra.journal:4  ", " b  /elsewhere/other.journal:2  "} {
		if !strings.Contains(content, want) {
			t.Errorf("panel lacks %q:\n%s", want, content)
		}
	}
}

func TestPanelKeysAreHarmlessWithoutACheckRun(t *testing.T) {
	model, _ := press(started(t, fakeLoader{err: errors.New("hledger exploded")}), "!")

	model = pressKey(model, tea.KeyDown)
	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil || model.checks.cursor.Row != 0 {
		t.Errorf("cmd = %v, cursor = %d; want nothing to happen", cmd, model.checks.cursor.Row)
	}
}

func TestErrorsAreRedAndWarningsAmberInThePanel(t *testing.T) {
	model, _ := press(withRun(t, nil,
		issue(domain.SeverityWarn, "a", "", 0), issue(domain.SeverityErr, "b", "", 0), issue(domain.SeverityWarn, "c", "", 0)), "!")

	content := model.View().Content
	if !strings.Contains(content, "\x1b[31mERR") || !strings.Contains(content, "\x1b[33mWARN") {
		t.Errorf("ERR should be red and WARN amber:\n%q", content)
	}
}

// A failed reload keeps the stale snapshot; the panel carries hledger's error text.
func TestChecksPanelShowsTheFailureOfAReload(t *testing.T) {
	model := started(t, fakeLoader{})
	failed, _ := model.Update(ledgerLoadedMsg{err: errors.New("hledger: boom\nsecond line")})
	opened, _ := press(failed.(Model), "!")

	content := uitest.Plain(opened.View().Content)

	if !strings.Contains(content, "✗ hledger: boom") || !strings.Contains(content, "second line") {
		t.Errorf("the panel should show the reload failure:\n%s", content)
	}
	if !strings.Contains(content, "all ") {
		t.Errorf("the stale check result stays beneath it:\n%s", content)
	}
}

func TestAShrinkKeepsTheSelectedIssueInView(t *testing.T) {
	issues := make([]domain.CheckIssue, 20)
	for i := range issues {
		issues[i] = issue(domain.SeverityWarn, "check"+string(rune('a'+i)), "", 0)
	}
	model, _ := press(withRun(t, nil, issues...), "!")
	for range 19 {
		model = pressKey(model, tea.KeyDown)
	}

	model = resized(model, 100, 30)

	if !strings.Contains(model.View().Content, ";1m WARN  checkt") {
		t.Errorf("the selected issue is off screen:\n%s", uitest.Plain(model.View().Content))
	}
}
