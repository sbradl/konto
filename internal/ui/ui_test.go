package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ledger"
)

const journalPath = "/data/household.journal"

type fakeLoader struct {
	ledger domain.Ledger
	err    error
}

func (f fakeLoader) Load(context.Context) (domain.Ledger, error) { return f.ledger, f.err }

type fakeCreator struct {
	calls int
	err   error
}

func (f *fakeCreator) Create() error {
	f.calls++
	return f.err
}

// journalLoader is a loader whose journal appears once the creator has run.
type journalLoader struct {
	creator *fakeCreator
}

func (j journalLoader) Load(context.Context) (domain.Ledger, error) {
	if j.creator.calls == 0 {
		return domain.Ledger{}, ledger.ErrNoJournal
	}
	return domain.Ledger{}, nil
}

// recordingEditor is an editor command builder that notes what it was asked to open.
func recordingEditor(calls *[]editorCall) func(file string, line int) *exec.Cmd {
	return func(file string, line int) *exec.Cmd {
		if calls != nil {
			*calls = append(*calls, editorCall{file, line})
		}
		return exec.CommandContext(context.Background(), "true")
	}
}

type editorCall struct {
	file string
	line int
}

func newModel() Model { return New(journalPath, fakeLoader{}, &fakeCreator{}, recordingEditor(nil)) }

func ledgerWith(transactions int) domain.Ledger {
	return domain.Ledger{Transactions: make([]domain.Transaction, transactions)}
}

// started returns the model after the terminal reported its size and the
// loader finished with the given result.
func started(t *testing.T, loader fakeLoader, width int) Model {
	t.Helper()
	model := New(journalPath, loader, &fakeCreator{}, recordingEditor(nil))
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init should start loading the ledger")
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 40})
	updated, _ = updated.Update(cmd())
	return updated.(Model)
}

func firstLine(m Model) string {
	first, _, _ := strings.Cut(m.View().Content, "\n")
	return first
}

// Until the journal has loaded, konto says so instead of showing empty chrome.
func TestViewShowsLoadingUntilLoaded(t *testing.T) {
	content := newModel().View().Content

	if !strings.Contains(content, "loading journal…") {
		t.Errorf("content = %q, want a loading indicator", content)
	}
	if strings.Contains(content, "1 dash") {
		t.Errorf("content = %q, should not show the tab bar yet", content)
	}
}

// Once loaded, the user sees the five-tab bar on the first line of the screen.
func TestViewShowsTabBarOnFirstLine(t *testing.T) {
	want := " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "

	if first := firstLine(started(t, fakeLoader{}, 120)); !strings.Contains(first, want) {
		t.Errorf("first line = %q, want it to contain %q", first, want)
	}
}

// konto takes over the whole terminal (alternate screen).
func TestViewUsesAlternateScreen(t *testing.T) {
	if !newModel().View().AltScreen {
		t.Error("view should request the alternate screen")
	}
}

// Pressing q or Ctrl+C quits.
func TestQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		_, cmd := newModel().Update(key)

		if cmd == nil {
			t.Fatalf("%v: expected a command, got nil", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v: command should quit", key)
		}
	}
}

// Other keys and messages leave the program running.
func TestOtherInputDoesNotQuit(t *testing.T) {
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: 'x', Text: "x"}, tea.WindowSizeMsg{Width: 120, Height: 40}} {
		if _, cmd := newModel().Update(msg); cmd != nil {
			t.Errorf("%T should not produce a command", msg)
		}
	}
}

// Once the ledger has loaded, the tab bar shows how many transactions it holds.
func TestTabBarShowsTransactionCountAfterLoad(t *testing.T) {
	first := firstLine(started(t, fakeLoader{ledger: ledgerWith(54)}, 120))

	if want := journalPath + " · 54 txns "; !strings.HasSuffix(first, want) {
		t.Errorf("line should end with %q, got %q", want, first)
	}
}

// A failed load is shown below the tab bar instead of being swallowed.
func TestFailedLoadIsShown(t *testing.T) {
	model := started(t, fakeLoader{err: errors.New("hledger exploded")}, 120)

	_, rest, _ := strings.Cut(model.View().Content, "\n")

	if !strings.Contains(rest, "hledger exploded") {
		t.Errorf("screen below the tab bar should show the error, got %q", rest)
	}
}

// The journal path sits at the right edge of the tab bar, opposite the tabs.
func TestTabBarShowsJournalPathAtRightEdge(t *testing.T) {
	first := firstLine(started(t, fakeLoader{}, 120))

	if !strings.HasPrefix(first, " 1 dash") {
		t.Errorf("tabs should start the line, got %q", first)
	}
	if want := journalPath + " · 0 txns "; !strings.HasSuffix(first, want) {
		t.Errorf("line should end with %q, got %q", want, first)
	}
	if width := lipgloss.Width(first); width != 120 {
		t.Errorf("tab bar width = %d, want 120", width)
	}
}

// On a terminal too narrow for both, the tabs and the path are still separated.
func TestTabBarKeepsTabsAndPathApartWhenNarrow(t *testing.T) {
	first := firstLine(started(t, fakeLoader{}, 40))

	if !strings.Contains(first, "5 reports "+journalPath) {
		t.Errorf("tabs and path should be separated by a space, got %q", first)
	}
}

// With no journal yet, konto offers to create one at the resolved path and
// shows no tab bar.
func TestFirstRunScreenOffersToCreateTheJournal(t *testing.T) {
	model := started(t, fakeLoader{err: ledger.ErrNoJournal}, 120)

	content := model.View().Content

	for _, want := range []string{journalPath, "y to create", "n to quit"} {
		if !strings.Contains(content, want) {
			t.Errorf("first-run screen should contain %q, got %q", want, content)
		}
	}
	if strings.Contains(content, "1 dash") {
		t.Errorf("first-run screen should not show the tab bar, got %q", content)
	}
}

// The loading indicator is centred on the screen.
func TestLoadingIndicatorIsCentred(t *testing.T) {
	updated, _ := newModel().Update(tea.WindowSizeMsg{Width: 41, Height: 11})

	lines := strings.Split(updated.View().Content, "\n")

	if len(lines) != 11 {
		t.Fatalf("screen has %d lines, want 11", len(lines))
	}
	want := strings.Repeat(" ", 12) + "loading journal…"
	if !strings.HasPrefix(lines[5], want) {
		t.Errorf("middle line = %q, want it to start with %q", lines[5], want)
	}
}

func firstRunModel(t *testing.T, creator *fakeCreator) Model {
	t.Helper()
	model := New(journalPath, journalLoader{creator: creator}, creator, recordingEditor(nil))
	updated, _ := model.Update(model.Init()())
	return updated.(Model)
}

func press(m Model, text string) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyPressMsg{Code: rune(text[0]), Text: text})
	return updated.(Model), cmd
}

func pressEnter(m Model) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return cmd
}

// Pressing y on the first-run screen creates the journal and enters the views.
func TestYCreatesTheJournalAndEntersTheViews(t *testing.T) {
	creator := &fakeCreator{}
	model, cmd := press(firstRunModel(t, creator), "y")

	if cmd == nil {
		t.Fatal("y should start creating the journal")
	}
	updated, _ := model.Update(cmd())

	if creator.calls != 1 {
		t.Errorf("Create called %d times, want 1", creator.calls)
	}
	if content := updated.View().Content; !strings.Contains(content, "1 dash") {
		t.Errorf("after creating, content = %q, want the tab bar", content)
	}
}

// If the journal cannot be created, the first-run screen says why.
func TestFailedCreationIsShownOnTheFirstRunScreen(t *testing.T) {
	creator := &fakeCreator{err: errors.New("permission denied")}
	model, cmd := press(firstRunModel(t, creator), "y")

	updated, _ := model.Update(cmd())

	content := updated.View().Content
	if !strings.Contains(content, "permission denied") || !strings.Contains(content, "n to quit") {
		t.Errorf("content = %q, want the error on the first-run screen", content)
	}
}

// Pressing n on the first-run screen quits without creating anything.
func TestNQuitsWithoutCreating(t *testing.T) {
	creator := &fakeCreator{}
	_, cmd := press(firstRunModel(t, creator), "n")

	if cmd == nil {
		t.Fatal("n should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("n should produce a quit")
	}
	if creator.calls != 0 {
		t.Errorf("Create called %d times, want 0", creator.calls)
	}
}

// y and n mean nothing once the journal is open.
func TestYAndNDoNothingOutsideTheFirstRunScreen(t *testing.T) {
	model := started(t, fakeLoader{}, 120)

	for _, key := range []string{"y", "n"} {
		if _, cmd := press(model, key); cmd != nil {
			t.Errorf("%s should do nothing in the normal views", key)
		}
	}
}

// A journal hledger cannot parse shows its ERR issue with a file:line
// reference and an offer to jump to the editor, instead of the views.
func TestParseErrorScreenShowsIssueAndEditorOffer(t *testing.T) {
	issue := domain.CheckIssue{
		Check: "parseable", Severity: domain.SeverityErr, Message: "unexpected 't'",
		File: "/data/household.journal", Line: 3, Col: 1,
	}
	model := started(t, fakeLoader{err: fmt.Errorf("loading ledger: %w", domain.JournalError{Issue: issue})}, 120)

	content := model.View().Content

	for _, want := range []string{"ERR", "parseable", "/data/household.journal:3:1", "unexpected 't'", "Enter", "editor"} {
		if !strings.Contains(content, want) {
			t.Errorf("parse-error screen should contain %q, got %q", want, content)
		}
	}
	if strings.Contains(content, "1 dash") {
		t.Errorf("parse-error screen should not show the tab bar, got %q", content)
	}
}

func parseErrorModel(t *testing.T, calls *[]editorCall) Model {
	t.Helper()
	issue := domain.CheckIssue{Check: "parseable", Severity: domain.SeverityErr, File: "/data/household.journal", Line: 3, Col: 1}
	loader := fakeLoader{err: fmt.Errorf("loading ledger: %w", domain.JournalError{Issue: issue})}
	model := New(journalPath, loader, &fakeCreator{}, recordingEditor(calls))
	updated, _ := model.Update(model.Init()())
	return updated.(Model)
}

// Enter on the parse-error screen opens the editor at the issue's line.
func TestEnterOpensTheEditorAtTheIssue(t *testing.T) {
	var calls []editorCall
	cmd := pressEnter(parseErrorModel(t, &calls))

	if cmd == nil {
		t.Fatal("Enter should start the editor")
	}
	if want := []editorCall{{"/data/household.journal", 3}}; !reflect.DeepEqual(calls, want) {
		t.Errorf("editor calls = %v, want %v", calls, want)
	}
}

// Enter means nothing outside the parse-error screen.
func TestEnterDoesNothingOutsideTheParseErrorScreen(t *testing.T) {
	var calls []editorCall
	model := New(journalPath, fakeLoader{}, &fakeCreator{}, recordingEditor(&calls))
	updated, _ := model.Update(model.Init()())

	cmd := pressEnter(updated.(Model))

	if cmd != nil || len(calls) != 0 {
		t.Errorf("Enter should do nothing in the normal views (cmd=%v, calls=%v)", cmd != nil, calls)
	}
}

// When the editor closes, konto reloads the journal.
func TestClosingTheEditorReloadsTheJournal(t *testing.T) {
	_, cmd := parseErrorModel(t, nil).Update(editorClosedMsg{})

	if cmd == nil {
		t.Fatal("closing the editor should reload the journal")
	}
	if _, ok := cmd().(ledgerLoadedMsg); !ok {
		t.Error("the reload should produce a ledgerLoadedMsg")
	}
}

// However the editor exits, konto reloads the journal.
func TestEditorExitBecomesReload(t *testing.T) {
	for _, exit := range []error{nil, errors.New("exit status 1")} {
		if _, ok := editorClosed(exit).(editorClosedMsg); !ok {
			t.Errorf("editorClosed(%v) should be an editorClosedMsg", exit)
		}
	}
}
