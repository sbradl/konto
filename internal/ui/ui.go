// Package ui is the root Bubble Tea model (ADR 0004): tab chrome and
// app-level key dispatch.
package ui

import (
	"context"
	"errors"
	"os/exec"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/ui/dashboard"
	"github.com/sbradl/konto/internal/ui/imports"
	"github.com/sbradl/konto/internal/ui/reports"
	"github.com/sbradl/konto/internal/ui/scheduled"
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

// reportBuilder builds the Reports view's tables for a period.
type reportBuilder interface {
	Report(ctx context.Context, period domain.Period) (domain.Report, error)
}

// csvImporter lists folders for the file picker and reads a bank export against the journal.
type csvImporter interface {
	Browse(dir string) ([]domain.FileEntry, error)
	Open(ctx context.Context, path string, snapshot domain.Ledger) (csvimport.Staged, error)
}

// journalWriter applies a changeset to the journal.
type journalWriter interface {
	Apply(changeset domain.Changeset) error
}

// changeVerifier checks a changeset before it is written and says why the journal would be wrong after it.
type changeVerifier interface {
	Verify(ctx context.Context, changeset domain.Changeset) error
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
	verifier    changeVerifier
	reporter    reportBuilder
	reports     reports.Model
	scheduled   scheduled.Model
	importer    csvImporter
	imports     imports.Model
	prompt      *prompt          // the y/n question awaiting an answer
	previewTop  int              // the first line of the prompt's preview on screen
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

// browsedMsg carries a folder's listing for the file picker.
type browsedMsg struct {
	dir     string
	entries []domain.FileEntry
	err     error
}

// openedMsg carries an export read against the journal. When the staged import was read again after the
// journal changed, earlier is what had been staged.
type openedMsg struct {
	staged  csvimport.Staged
	err     error
	earlier *csvimport.Staged
}

// reportLoadedMsg carries a built report for the period it was asked for.
type reportLoadedMsg struct {
	period domain.Period
	report domain.Report
	err    error
}

// writtenMsg reports that a changeset was applied (or why it was not); imported is the confirmation of an
// import's write.
type writtenMsg struct {
	err      error
	imported string
}

// editorClosedMsg reports that the user's editor has exited.
type editorClosedMsg struct{}

// Ports are the use cases and adapters the root model drives.
type Ports struct {
	Loader   ledgerLoader   // reads the journal into a snapshot
	Checker  checkRunner    // checks the journal and the snapshot
	Creator  journalCreator // writes the first-run journal
	Accounts accountPlanner // plans closing and reopening accounts
	Writer   journalWriter  // applies planned changes to the journal
	Verifier changeVerifier // checks an import's changes before they are written; none skips the check
	Reports  reportBuilder  // builds the Reports view's tables
	Importer csvImporter    // lists folders and reads bank exports
	// DownloadDir is where the file picker starts.
	DownloadDir string
	// Editor builds the command that opens a file at a line.
	Editor func(file string, line int) *exec.Cmd
}

// New returns the root model for the journal at journalPath, driving the given ports.
func New(journalPath string, ports Ports) Model {
	return Model{
		journalPath: journalPath, loader: ports.Loader, checker: ports.Checker, creator: ports.Creator,
		editor: ports.Editor, accounts: ports.Accounts, writer: ports.Writer, verifier: ports.Verifier,
		dash:     dashboard.New(),
		reporter: ports.Reports, reports: reports.New(), scheduled: scheduled.New(),
		importer: ports.Importer, imports: imports.New(ports.DownloadDir),
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
		m.reports = m.reports.Resize(m.width, max(0, m.height-chromeRows))
		m.scheduled = m.scheduled.Resize(m.width, max(0, m.height-chromeRows))
		m.imports = m.imports.Resize(m.width, max(0, m.height-chromeRows))
	}
	if loaded, ok := msg.(ledgerLoadedMsg); ok {
		m = m.onLoaded(loaded)
		return m, tea.Batch(m.loadReport(loaded.err), m.reopen(loaded))
	}
	if report, ok := msg.(reportLoadedMsg); ok {
		return m.onReport(report), nil
	}
	if browsed, ok := msg.(browsedMsg); ok {
		m.imports = m.imports.WithListing(browsed.dir, browsed.entries, browsed.err)
		return m, nil
	}
	if opened, ok := msg.(openedMsg); ok {
		return m.onOpened(opened), nil
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
	m.dash = m.dash.WithLedger(loaded.ledger)
	m.scheduled = m.scheduled.WithLedger(loaded.ledger)
	if loaded.err == nil {
		m.run = &loaded.run
		m.checkCursor = max(0, min(m.checkCursor, len(loaded.run.Issues)-1))
		m = m.scrollChecks()
	}
	m.firstRun = errors.Is(loaded.err, ledger.ErrNoJournal)
	m.parseIssue = issueOf(loaded.err)
	return m
}

// loadReport builds the Reports view's report for its current period, once the journal has loaded.
func (m Model) loadReport(loadErr error) tea.Cmd {
	if m.reporter == nil || loadErr != nil {
		return nil
	}
	period := m.reports.Period()
	return func() tea.Msg {
		report, err := m.reporter.Report(context.Background(), period)
		return reportLoadedMsg{period: period, report: report, err: err}
	}
}

// onReport takes a built report in; one for a period no longer shown is dropped, and a failed one
// leaves the old report on screen with the error flagged.
func (m Model) onReport(loaded reportLoadedMsg) Model {
	switch {
	case loaded.period != m.reports.Period():
	case loaded.err != nil:
		m.refreshErr = loaded.err
	default:
		m.reports = m.reports.WithReport(loaded.report)
	}
	return m
}

// hasSnapshot reports whether a good ledger is on screen.
func (m Model) hasSnapshot() bool {
	return m.ledger != nil && m.loadErr == nil
}

func (m Model) onKey(key tea.KeyPressMsg) (Model, tea.Cmd) {
	name := keyName(key)
	if m.capturing() && name != "ctrl+c" {
		return m.onImportKey(name)
	}
	switch name {
	case "q", "ctrl+c":
		return m.quit()
	}
	if m.tooSmall() {
		return m, nil
	}
	if m.firstRun {
		return m, m.onFirstRunKey(name)
	}
	if m.parseIssue != nil {
		return m, m.onParseErrorKey(name)
	}
	if m.ledger != nil {
		return m.onViewKey(name)
	}
	return m, nil
}

// quit leaves konto, asking first when an import is staged that leaving would discard. A second q while
// the question is open, or a terminal too small to answer it in, quits at once.
func (m Model) quit() (Model, tea.Cmd) {
	if _, staged := m.imports.Staged(); staged && !m.tooSmall() && (m.prompt == nil || !m.prompt.quit) {
		m.prompt = &prompt{question: "discard the staged import?", quit: true}
		return m, nil
	}
	return m, tea.Quit
}

// keyName is the key as the views name it; a space is typed text, not a word.
func keyName(key tea.KeyPressMsg) string {
	if name := key.String(); name != "space" {
		return name
	}
	return " "
}

// capturing reports whether the Import view's dialog has the keyboard: every key is then text for it.
func (m Model) capturing() bool {
	return m.ledger != nil && m.importer != nil && m.prompt == nil && m.popup == popupNone &&
		m.active == importTab && m.imports.Capturing() && !m.tooSmall()
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
	m, cmd := m.onActiveViewKey(key)
	if number, err := strconv.Atoi(key); err == nil && number >= 1 && number <= tabCount {
		m.active = number - 1
	}
	return m, cmd
}

// onActiveViewKey hands a key to the view on screen.
func (m Model) onActiveViewKey(key string) (Model, tea.Cmd) {
	switch m.active {
	case 0:
		return m.onDashboardKey(key), nil
	case importTab:
		return m.onImportKey(key)
	case scheduledTab:
		return m.onScheduledKey(key), nil
	case reportsTab:
		return m.onReportsKey(key)
	}
	return m, nil
}

// Tab indices of the views.
const (
	importTab    = 1
	scheduledTab = 2
	reportsTab   = 4
)

// onReportsKey scrolls the tables, or on p moves to the next period and builds its report.
func (m Model) onReportsKey(key string) (Model, tea.Cmd) {
	if key != "p" {
		m.reports = m.reports.Move(key)
		return m, nil
	}
	m.reports = m.reports.NextPeriod()
	return m, m.loadReport(nil)
}

// openEditor suspends konto while the user's editor shows the issue.
func (m Model) openEditor(issue domain.CheckIssue) tea.Cmd {
	return tea.ExecProcess(m.editor(issue.File, issue.Line), editorClosed)
}

// editorClosed turns the editor's exit, however it went, into a reload request.
func editorClosed(error) tea.Msg { return editorClosedMsg{} }
