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

type journalCreator interface {
	Create() error
}

// journalCreateFailedMsg reports that the first-run journal could not be written.
type journalCreateFailedMsg struct{ err error }

// ledgerLoadedMsg carries the result of loading the journal.
type ledgerLoadedMsg struct {
	ledger domain.Ledger
	err    error
}

// Model is konto's single tea.Model.
type Model struct {
	journalPath string
	loader      ledgerLoader
	creator     journalCreator
	editor      func(file string, line int) *exec.Cmd
	ledger      *domain.Ledger
	loadErr     error
	refreshing  bool  // a reload is in flight; the held snapshot stays on screen
	refreshErr  error // the last reload failed; the snapshot is stale
	active      int   // index into tabNames
	popup       popupKind
	firstRun    bool
	parseIssue  *domain.CheckIssue
	createErr   error
	width       int
	height      int
}

// editorClosedMsg reports that the user's editor has exited.
type editorClosedMsg struct{}

// New returns the root model for the journal at journalPath, loaded through
// loader; creator writes the journal on first run; editor builds the command
// that opens a file at a line.
func New(
	journalPath string, loader ledgerLoader, creator journalCreator, editor func(file string, line int) *exec.Cmd,
) Model {
	return Model{journalPath: journalPath, loader: loader, creator: creator, editor: editor}
}

// Init starts loading the ledger.
func (m Model) Init() tea.Cmd { return m.load }

func (m Model) load() tea.Msg {
	snapshot, err := m.loader.Load(context.Background())
	return ledgerLoadedMsg{ledger: snapshot, err: err}
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
	if m.popup != popupNone {
		return m.onPopupKey(key), nil
	}
	switch key {
	case "?":
		m.popup = popupKeys
	case "!":
		m.popup = popupChecks
	}
	if key == "r" {
		m.refreshing = true
		return m, m.load
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
