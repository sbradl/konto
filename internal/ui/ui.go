// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"context"
	"errors"
	"os/exec"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/ui/dashboard"
)

// The smallest terminal konto lays its views out in (spec §6.8).
const (
	minWidth  = 100
	minHeight = 30
)

// tabCount is the number of v1 views.
const tabCount = 5

// tabNames are the five v1 views, in tab order; number key i+1 selects tab i.
func tabNames() [tabCount]string {
	return [tabCount]string{"dash", "import", "sched", "forecast", "reports"}
}

type ledgerLoader interface {
	Load(ctx context.Context) (domain.Ledger, error)
}

type checkRunner interface {
	Run(ctx context.Context, snapshot domain.Ledger) (domain.CheckRun, error)
}

// accountPlanner decides account changes and returns the changeset to write.
type accountPlanner interface {
	Close(account domain.Account) (domain.Changeset, error)
	Reopen(account domain.Account) (domain.Changeset, error)
}

// journalWriter applies a changeset to the journal.
type journalWriter interface {
	Apply(changeset domain.Changeset) error
}

type journalCreator interface {
	Create() error
}

// journalCreateFailedMsg reports that the first-run journal could not be written.
type journalCreateFailedMsg struct{ err error }

// ledgerLoadedMsg carries the result of loading the journal and checking it.
type ledgerLoadedMsg struct {
	ledger domain.Ledger
	run    domain.CheckRun
	err    error
}

// Model is konto's single tea.Model.
type Model struct {
	journalPath string
	loader      ledgerLoader
	checker     checkRunner
	accounts    accountPlanner
	writer      journalWriter
	prompt      *prompt          // the y/n question awaiting an answer
	notice      string           // a one-line message shown until the next key
	run         *domain.CheckRun // the latest check run; nil until the first one lands
	checkCursor int              // the selected issue in the checks panel
	checkOffset int              // the first visible issue in the checks panel
	creator     journalCreator
	editor      func(file string, line int) *exec.Cmd
	ledger      *domain.Ledger
	loadErr     error
	refreshing  bool  // a reload is in flight; the held snapshot stays on screen
	refreshErr  error // the last reload failed; the snapshot is stale
	active      int   // index into tabNames
	popup       popupKind
	dash        dashboard.Model
	firstRun    bool
	parseIssue  *domain.CheckIssue
	createErr   error
	width       int
	height      int
}

// writtenMsg reports that a changeset was applied (or why it was not).
type writtenMsg struct{ err error }

// editorClosedMsg reports that the user's editor has exited.
type editorClosedMsg struct{}

// Ports are the use cases and adapters the root model drives.
type Ports struct {
	Loader   ledgerLoader   // reads the journal into a snapshot
	Checker  checkRunner    // checks the journal and the snapshot
	Creator  journalCreator // writes the first-run journal
	Accounts accountPlanner // plans closing and reopening accounts
	Writer   journalWriter  // applies planned changes to the journal
	// Editor builds the command that opens a file at a line.
	Editor func(file string, line int) *exec.Cmd
}

// New returns the root model for the journal at journalPath, driving the given ports.
func New(journalPath string, ports Ports) Model {
	return Model{
		journalPath: journalPath, loader: ports.Loader, checker: ports.Checker, creator: ports.Creator,
		editor: ports.Editor, accounts: ports.Accounts, writer: ports.Writer, dash: dashboard.New(),
	}
}

// Init starts loading the ledger.
func (m Model) Init() tea.Cmd { return m.load }

// load reads the journal and, if it loads, runs every check over it, so the screen
// always shows a snapshot together with its own check status.
func (m Model) load() tea.Msg {
	ctx := context.Background()
	snapshot, err := m.loader.Load(ctx)
	if err != nil {
		return ledgerLoadedMsg{ledger: snapshot, err: err}
	}
	run, err := m.checker.Run(ctx, snapshot)
	return ledgerLoadedMsg{ledger: snapshot, run: run, err: err}
}

// create writes the first-run journal, then loads it.
func (m Model) create() tea.Msg {
	if err := m.creator.Create(); err != nil {
		return journalCreateFailedMsg{err: err}
	}
	return m.load()
}

// Update handles app-level input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		m.dash = m.dash.Resize(m.width, max(0, m.height-chromeRows))
	}
	if loaded, ok := msg.(ledgerLoadedMsg); ok {
		m = m.onLoaded(loaded)
	}
	if failed, ok := msg.(journalCreateFailedMsg); ok {
		m.createErr = failed.err
	}
	if _, ok := msg.(editorClosedMsg); ok {
		return m, m.load
	}
	if written, ok := msg.(writtenMsg); ok {
		return m.onWritten(written)
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		return m.onKey(key)
	}
	return m, nil
}

// onLoaded takes a load result in. Once a good snapshot is held, a failed reload
// keeps it (stale-while-revalidate) and only flags the error.
func (m Model) onLoaded(loaded ledgerLoadedMsg) Model {
	m.refreshing = false
	if m.hasSnapshot() && loaded.err != nil {
		m.refreshErr = loaded.err
		return m
	}
	m.refreshErr = nil
	m.ledger, m.loadErr = &loaded.ledger, loaded.err
	m.dash = m.dash.WithAccounts(loaded.ledger.Accounts)
	if loaded.err == nil {
		m.run = &loaded.run
		m.checkCursor = max(0, min(m.checkCursor, len(loaded.run.Issues)-1))
		m = m.scrollChecks()
	}
	m.firstRun = errors.Is(loaded.err, ledger.ErrNoJournal)
	m.parseIssue = issueOf(loaded.err)
	return m
}

// hasSnapshot reports whether a good ledger is on screen.
func (m Model) hasSnapshot() bool {
	return m.ledger != nil && m.loadErr == nil
}

func (m Model) onKey(key tea.KeyPressMsg) (Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	if m.tooSmall() {
		return m, nil
	}
	if m.firstRun {
		return m, m.onFirstRunKey(key.String())
	}
	if m.parseIssue != nil {
		return m, m.onParseErrorKey(key.String())
	}
	if m.ledger != nil {
		return m.onViewKey(key.String())
	}
	return m, nil
}

func (m Model) onFirstRunKey(key string) tea.Cmd {
	switch key {
	case "y":
		return m.create
	case "n":
		return tea.Quit
	}
	return nil
}

func (m Model) onParseErrorKey(key string) tea.Cmd {
	if key == "enter" {
		return m.openEditor(*m.parseIssue)
	}
	return nil
}

// onViewKey handles the global keys of the normal views: 1–5 switch the view, r reloads, ? and ! open the popups.
func (m Model) onViewKey(key string) (Model, tea.Cmd) {
	if m.prompt != nil {
		return m.onPromptKey(key)
	}
	m.notice = ""
	if key == "r" {
		m.refreshing = true
		return m, m.load
	}
	if m.popup != popupNone {
		return m.onPopupKey(key)
	}
	switch key {
	case "?":
		m.popup = popupKeys
	case "!":
		m.popup = popupChecks
	}
	if m.active == 0 {
		m = m.onDashboardKey(key)
	}
	if number, err := strconv.Atoi(key); err == nil && number >= 1 && number <= tabCount {
		m.active = number - 1
	}
	return m, nil
}

// openEditor suspends konto while the user's editor shows the issue.
func (m Model) openEditor(issue domain.CheckIssue) tea.Cmd {
	return tea.ExecProcess(m.editor(issue.File, issue.Line), editorClosed)
}

// editorClosed turns the editor's exit, however it went, into a reload request.
func editorClosed(error) tea.Msg { return editorClosedMsg{} }
