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
	forecastsvc "github.com/sbradl/konto/internal/forecast"
	"github.com/sbradl/konto/internal/ledger"
	"github.com/sbradl/konto/internal/ui/dashboard"
	forecastview "github.com/sbradl/konto/internal/ui/forecast"
	"github.com/sbradl/konto/internal/ui/imports"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/reports"
	"github.com/sbradl/konto/internal/ui/scheduled"
)

// The smallest terminal konto lays its views out in (spec §6.8).
const (
	minWidth  = 100
	minHeight = 30
)

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
	Report(ctx context.Context, period domain.ReportPeriod) (domain.Report, error)
}

// forecaster builds the Forecast view's projection of the loaded ledger.
type forecaster interface {
	Forecast(ctx context.Context, snapshot domain.Ledger) (forecastsvc.Forecast, error)
}

// csvImporter lists folders for the file picker and reads a bank export against the journal.
type csvImporter interface {
	Browse(dir string) ([]domain.FileEntry, error)
	Open(ctx context.Context, path string, snapshot domain.Ledger) (csvimport.Staged, error)
}

// journalWriter writes a changeset to the journal: Prepare completes it with what else the write brings, once,
// and Apply writes that prepared changeset as it is.
type journalWriter interface {
	Prepare(ctx context.Context, changeset domain.Changeset) (domain.Changeset, error)
	Apply(changeset domain.Changeset) error
}

// changeVerifier checks a prepared changeset before it is written and says why the journal would be wrong after it.
type changeVerifier interface {
	Verify(ctx context.Context, changeset domain.Changeset) error
}

type journalCreator interface {
	Create() error
}

// journalCreateFailedMsg reports that the first-run journal could not be written.
type journalCreateFailedMsg struct{ err error }

// ledgerLoadedMsg carries the result of loading the journal and checking it; load numbers the load, so the
// result of one asked for before the latest is dropped (ADR 0004).
type ledgerLoadedMsg struct {
	load   int
	ledger domain.Ledger
	run    domain.CheckRun
	err    error
}

func (msg journalCreateFailedMsg) takeIn(m Model) (Model, tea.Cmd) {
	m.createErr = msg.err

	return m, nil
}

func (msg ledgerLoadedMsg) takeIn(m Model) (Model, tea.Cmd) { return m.onLedgerLoaded(msg) }

// Model is konto's single tea.Model.
type Model struct {
	// ports: the use cases and adapters the root drives
	journalPath string
	loader      ledgerLoader
	checker     checkRunner
	creator     journalCreator
	accounts    accountPlanner
	writer      journalWriter
	verifier    changeVerifier
	reporter    reportBuilder
	forecaster  forecaster
	rules       rulePlanner
	importer    csvImporter
	editor      func(file string, line int) *exec.Cmd

	// view models
	dash      dashboard.Model
	imports   imports.Model
	scheduled scheduled.Model
	forecasts forecastview.Model
	reports   reports.Model

	// session state
	ledger    *domain.Ledger  // the latest good snapshot; nil until a load succeeds
	loadErr   error           // why the journal could not be loaded, while no snapshot is held
	createErr error           // why the first-run journal could not be written
	checks    checksPanel     // the latest check run and the panel's cursor over it
	failure   error           // the last hledger call that failed, flagged until a reload succeeds
	loads     int             // the number of the latest load asked for; results of earlier ones are stale
	reloading bool            // the latest load is in flight; the held snapshot stays on screen
	writing   bool            // a changeset is on its way to the journal
	saving    *scheduled.Save // the form's save being planned; nil when none is
	prompt    *prompt         // the y/n question awaiting an answer
	notice    string          // a one-line message shown until the next key
	popup     popupKind       // the popup over the view, if any
	active    int             // index into tabs
	width     int
	height    int
}

// browsedMsg carries a folder's listing for the file picker; a listing does not depend on the journal, so
// it is never stale.
type browsedMsg struct {
	dir     string
	entries []domain.FileEntry
	err     error
}

func (msg browsedMsg) takeIn(m Model) (Model, tea.Cmd) {
	m.imports = m.imports.WithListing(msg.dir, msg.entries, msg.err)

	return m, nil
}

// openedMsg carries an export read against the journal of load. When the staged import was read again after
// the journal changed, earlier is what had been staged.
type openedMsg struct {
	staged  csvimport.Staged
	err     error
	earlier *csvimport.Staged
	load    int
}

func (msg openedMsg) takeIn(m Model) (Model, tea.Cmd) { return m.onOpened(msg), nil }

// forecastLoadedMsg carries a forecast built from the journal of a load, or why it could not be built.
type forecastLoadedMsg struct {
	load     int
	forecast forecastsvc.Forecast
	err      error
}

func (msg forecastLoadedMsg) takeIn(m Model) (Model, tea.Cmd) { return m.onForecast(msg), nil }

// reportLoadedMsg carries a built report for the period and load it was asked for.
type reportLoadedMsg struct {
	load   int
	period domain.ReportPeriod
	report domain.Report
	err    error
}

func (msg reportLoadedMsg) takeIn(m Model) (Model, tea.Cmd) { return m.onReport(msg), nil }

// writtenMsg reports that a changeset was applied, and what was to follow, or why it was not. A write is never
// stale: whatever was reloaded meanwhile, the journal has changed (or not) and the model must hear of it.
type writtenMsg struct {
	err     error
	pending pendingWrite
}

func (msg writtenMsg) takeIn(m Model) (Model, tea.Cmd) { return m.onWritten(msg) }

// editorClosedMsg reports that the user's editor has exited.
type editorClosedMsg struct{}

func (editorClosedMsg) takeIn(m Model) (Model, tea.Cmd) { return m.reload() }

// Ports are the use cases and adapters the root model drives.
type Ports struct {
	Loader   ledgerLoader   // reads the journal into a snapshot
	Checker  checkRunner    // checks the journal and the snapshot
	Creator  journalCreator // writes the first-run journal
	Accounts accountPlanner // plans closing and reopening accounts
	Writer   journalWriter  // applies planned changes to the journal
	Verifier changeVerifier // checks an import's changes before they are written; none skips the check
	Reports  reportBuilder  // builds the Reports view's tables
	Forecast forecaster     // projects the Forecast view's balances
	Rules    rulePlanner    // plans the Scheduled form's writes
	Importer csvImporter    // lists folders and reads bank exports
	// DownloadDir is where the file picker starts.
	DownloadDir string
	// Editor builds the command that opens a file at a line.
	Editor func(file string, line int) *exec.Cmd
}

// New returns the root model for the journal at journalPath, driving the given ports.
func New(journalPath string, ports Ports) Model {
	return Model{
		journalPath: journalPath,
		loader:      ports.Loader,
		checker:     ports.Checker,
		creator:     ports.Creator,
		accounts:    ports.Accounts,
		writer:      ports.Writer,
		verifier:    ports.Verifier,
		reporter:    ports.Reports,
		forecaster:  ports.Forecast,
		rules:       ports.Rules,
		importer:    ports.Importer,
		editor:      ports.Editor,
		dash:        dashboard.New(),
		imports:     imports.New(ports.DownloadDir),
		scheduled:   scheduled.New(),
		forecasts:   forecastview.New(),
		reports:     reports.New(),
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
		return ledgerLoadedMsg{load: m.loads, ledger: snapshot, err: err}
	}
	run, err := m.checker.Run(ctx, snapshot)

	return ledgerLoadedMsg{load: m.loads, ledger: snapshot, run: run, err: err}
}

// reload loads the journal again, the spinner showing until it lands; whatever an earlier load still in
// flight finds out is dropped.
func (m Model) reload() (Model, tea.Cmd) {
	m.loads++
	m.reloading = true

	return m, m.load
}

// fresh reports whether a result asked for from the snapshot of load is still about the journal on screen.
func (m Model) fresh(load int) bool { return load == m.loads }

// create writes the first-run journal, then loads it.
func (m Model) create() tea.Msg {
	if err := m.creator.Create(); err != nil {
		return journalCreateFailedMsg{err: err}
	}

	return m.load()
}

// result is what a command found out; it takes itself into the model and says what to run next.
type result interface {
	takeIn(m Model) (Model, tea.Cmd)
}

// Every async message is a result; most ask for nothing further.
var _ = []result{
	journalCreateFailedMsg{},
	ledgerLoadedMsg{},
	browsedMsg{},
	openedMsg{},
	forecastLoadedMsg{},
	reportLoadedMsg{},
	writtenMsg{},
	editorClosedMsg{},
	formReadyMsg{},
	ruleSavedMsg{},
}

// Update routes a message: the terminal's size and keys to the root, what a command found out to its handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg), nil
	case tea.KeyPressMsg:
		return m.onKey(msg)
	case result:
		return msg.takeIn(m)
	}

	return m, nil
}

// resize hands every view the content area below the tab bar and above the status bar.
func (m Model) resize(size tea.WindowSizeMsg) Model {
	m.width, m.height = size.Width, size.Height
	content := max(0, m.height-chromeRows)
	for _, tab := range tabs() {
		m = tab.resize(m, m.width, content)
	}
	m.checks = m.checks.scrolled(m.height)

	return m
}

// onLedgerLoaded takes the latest load in and builds what follows from it: the report, the staged export read
// again, the forecast. A load asked for before the latest is dropped.
func (m Model) onLedgerLoaded(loaded ledgerLoadedMsg) (Model, tea.Cmd) {
	if !m.fresh(loaded.load) {
		return m, nil
	}
	m = m.onLoaded(loaded)

	return m, tea.Batch(m.loadReport(loaded.err), m.reopen(loaded), m.loadForecast(loaded))
}

// onLoaded takes a load result in. Once a good snapshot is held, a failed reload
// keeps it (stale-while-revalidate) and only flags the error.
func (m Model) onLoaded(loaded ledgerLoadedMsg) Model {
	m.reloading = false
	if loaded.err != nil {
		if m.hasSnapshot() {
			m.failure = loaded.err
		} else {
			m.loadErr = loaded.err
		}

		return m
	}
	m.ledger, m.loadErr, m.failure = &loaded.ledger, nil, nil
	m.dash = m.dash.WithLedger(loaded.ledger)
	m.scheduled = m.scheduled.WithLedger(loaded.ledger)
	m.checks = m.checks.withRun(loaded.run, m.height)

	return m
}

// firstRun reports that there is no journal yet.
func (m Model) firstRun() bool { return errors.Is(m.loadErr, ledger.ErrNoJournal) }

// parseIssue is the located journal error that kept the journal from loading, if there is one.
func (m Model) parseIssue() *domain.CheckIssue { return issueOf(m.loadErr) }

// loadReport builds the Reports view's report for its current period, once the journal has loaded.
func (m Model) loadReport(loadErr error) tea.Cmd {
	if m.reporter == nil || loadErr != nil {
		return nil
	}
	period := m.reports.Period()

	return func() tea.Msg {
		report, err := m.reporter.Report(context.Background(), period)

		return reportLoadedMsg{load: m.loads, period: period, report: report, err: err}
	}
}

// onReport takes a built report in; one for a period no longer shown or of an earlier load is dropped, and a
// failed one is flagged, the Reports view saying why when it has no report of the period to show.
func (m Model) onReport(loaded reportLoadedMsg) Model {
	shown := loaded.period == m.reports.Period()
	switch {
	case !shown || !m.fresh(loaded.load):
	case loaded.err != nil:
		m.failure = loaded.err
		m.reports = m.reports.WithFailure(loaded.err)
	default:
		m.reports = m.reports.WithReport(loaded.report)
	}

	return m
}

// loadForecast projects the freshly loaded journal for the Forecast view.
func (m Model) loadForecast(loaded ledgerLoadedMsg) tea.Cmd {
	if m.forecaster == nil || loaded.err != nil {
		return nil
	}

	return func() tea.Msg {
		built, err := m.forecaster.Forecast(context.Background(), loaded.ledger)

		return forecastLoadedMsg{load: loaded.load, forecast: built, err: err}
	}
}

// onForecast takes a built forecast in; one of an earlier load is dropped, and a failed one leaves the last
// projection on screen with the error flagged.
func (m Model) onForecast(built forecastLoadedMsg) Model {
	if !m.fresh(built.load) {
		return m
	}

	if built.err != nil {
		m.failure = built.err

		return m
	}
	m.forecasts = m.forecasts.WithForecast(built.forecast)

	return m
}

// hasSnapshot reports whether a good ledger is on screen.
func (m Model) hasSnapshot() bool { return m.ledger != nil }

func (m Model) onKey(key tea.KeyPressMsg) (Model, tea.Cmd) {
	name := keyName(key)
	if m.capturing() && name != "ctrl+c" {
		return m.onCapturedKey(name)
	}
	switch name {
	case "q", "ctrl+c":
		return m.quit()
	}
	switch m.screen() {
	case screenFirstRun:
		return m, m.onFirstRunKey(name)
	case screenParseIssue:
		return m, m.onParseErrorKey(name)
	case screenViews, screenLoadFailed:
		return m.onViewKey(name)
	case screenTooSmall, screenLoading:
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

// capturing reports whether a dialog of the view on screen, the Import dialog or the Scheduled form, has the
// keyboard: every key is then text for it.
func (m Model) capturing() bool {
	if m.screen() != screenViews || m.prompt != nil || m.popup != popupNone {
		return false
	}
	capture := m.activeTab().capture

	return capture != nil && capture.active(m)
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
	if key == layout.KeyEnter {
		return m.openEditor(*m.parseIssue())
	}

	return nil
}

// onViewKey handles a key of the normal views: the open prompt or popup takes it first, then the root's own
// keys, and only then the view on screen.
func (m Model) onViewKey(key string) (Model, tea.Cmd) {
	if m.prompt != nil {
		return m.onPromptKey(key)
	}

	m.notice = ""
	if m.popup != popupNone {
		return m.onPopupKey(key)
	}

	if updated, cmd, global := m.onGlobalKey(key); global {
		return updated, cmd
	}

	if m.screen() == screenLoadFailed {
		return m, nil // no journal to act on
	}
	m, cmd := m.onActiveViewKey(key)

	return m, cmd
}

// onGlobalKey handles the keys the root keeps from every view (spec §6.4): r reloads, ? and ! open the popups,
// 1–5 switch the view; q and Ctrl+C are handled before, since they work on every screen. global is false
// for any other key.
func (m Model) onGlobalKey(key string) (updated Model, cmd tea.Cmd, global bool) {
	switch key {
	case "r":
		updated, cmd = m.reload()

		return updated, cmd, true
	case "?":
		m.popup = popupKeys
	case "!":
		m.popup = popupChecks
	default:
		index, isTab := tabIndex(key)
		if !isTab {
			return m, nil, false
		}
		m.active = index
	}

	return m, nil, true
}

// tabIndex is the tab a number key selects: 1 the first.
func tabIndex(key string) (int, bool) {
	number, err := strconv.Atoi(key)
	if err != nil || number < 1 || number > tabCount {
		return 0, false
	}

	return number - 1, true
}

// onReportsKey hands a key to the Reports view and builds the report it asks for.
func (m Model) onReportsKey(key string) (Model, tea.Cmd) {
	var action reports.Action

	m.reports, action = m.reports.Key(key)
	if _, load := action.(reports.LoadReport); load {
		return m, m.loadReport(nil)
	}

	return m, nil
}

// openEditor suspends konto while the user's editor shows the issue.
func (m Model) openEditor(issue domain.CheckIssue) tea.Cmd {
	return tea.ExecProcess(m.editor(issue.File, issue.Line), editorClosed)
}

// editorClosed turns the editor's exit, however it went, into a reload request.
func editorClosed(error) tea.Msg { return editorClosedMsg{} }
