//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sbradl/konto/e2e/shimconf"
)

const (
	defaultToday  = "2026-09-30"
	defaultWidth  = 120
	defaultHeight = 40
)

// suite is what every scenario shares: paths and binaries built once.
type suite struct {
	root        string // repository root
	realHledger string
	shim        string // built hledgershim binary
	timeout     time.Duration
	artifacts   string // where failed scenarios dump screen + stderr; "" = off

	buildOnce sync.Once
	konto     string
	buildErr  error
}

// buildDir is the temp directory holding the shim and konto binaries.
func (s *suite) buildDir() string { return filepath.Dir(s.shim) }

func newSuite() (*suite, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	realHledger, err := exec.LookPath("hledger")
	if err != nil {
		return nil, fmt.Errorf("the e2e suite needs the real hledger on PATH: %w", err)
	}

	bin, err := os.MkdirTemp("", "konto-e2e-bin-")
	if err != nil {
		return nil, err
	}

	shim := filepath.Join(bin, "hledgershim")
	if out, err := goBuild(root, shim, "./e2e/hledgershim"); err != nil {
		return nil, fmt.Errorf("building hledgershim: %w\n%s", err, out)
	}

	timeout, err := stepTimeout()
	if err != nil {
		return nil, err
	}

	return &suite{
		root: root, realHledger: realHledger, shim: shim, timeout: timeout,
		artifacts: os.Getenv("KONTO_E2E_ARTIFACTS"),
		konto:     filepath.Join(bin, "konto"),
	}, nil
}

// stepTimeout is how long a step waits for konto: 5s, or what $KONTO_E2E_TIMEOUT says.
func stepTimeout() (time.Duration, error) {
	value := os.Getenv("KONTO_E2E_TIMEOUT")
	if value == "" {
		return 5 * time.Second, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("KONTO_E2E_TIMEOUT: %w", err)
	}

	return timeout, nil
}

// kontoBinary builds cmd/konto on first use, so the harness self-tests need not build it.
func (s *suite) kontoBinary() (string, error) {
	s.buildOnce.Do(func() {
		if out, err := goBuild(s.root, s.konto, "./cmd/konto"); err != nil {
			s.buildErr = fmt.Errorf("building konto: %w\n%s", err, out)
		}
	})

	return s.konto, s.buildErr
}

func goBuild(root, out, pkg string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "go", "build", "-tags", "e2e", "-o", out, pkg)
	cmd.Dir = root

	return cmd.CombinedOutput()
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

// world is one scenario's sandbox: its own HOME, journal, PATH and terminal.
type world struct {
	suite *suite

	dir       string // scenario root, the <tmp> of the features
	home      string
	downloads string
	binDir    string // PATH entry holding the shim as "hledger"
	journal   string
	rules     string

	env    map[string]string
	unset  map[string]bool
	width  int
	height int

	shim     shimconf.Config
	shimConf string
	shimLog  string

	s              *session
	earlier        []*session // instances started before the current one
	journalAtStart []byte
	filesAtStart   map[string][]byte // every journal file of the sandbox at start
	rulesAtStart   []byte
	baseline       string
	tabAtBaseline  string
	lastStepType   string
	focusedCard    statCard  // the stat card a step last looked at
	focusedRecent  recentRow // the Recent row a step last looked at
	focusedRow     string    // the checks panel row a step last looked at
	leftRow        string    // the Import row the cursor was last moved to
}

func newWorld(s *suite) (*world, error) {
	dir, err := os.MkdirTemp("", "konto-e2e-")
	if err != nil {
		return nil, err
	}
	w := &world{
		suite:     s,
		dir:       dir,
		home:      filepath.Join(dir, "home"),
		downloads: filepath.Join(dir, "home", "Downloads"),
		binDir:    filepath.Join(dir, "bin"),
		journal:   filepath.Join(dir, "main.journal"),
		rules:     filepath.Join(dir, "categories.rules"),
		shimConf:  filepath.Join(dir, "shim.json"),
		shimLog:   filepath.Join(dir, "hledger-calls.log"),
		width:     defaultWidth,
		height:    defaultHeight,
		unset:     map[string]bool{},
	}
	for _, d := range []string{w.home, w.downloads, w.binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	if err := os.Symlink(s.shim, filepath.Join(w.binDir, "hledger")); err != nil {
		return nil, err
	}

	w.env = w.sandboxEnv()
	if err := w.writeShim(); err != nil {
		return nil, err
	}

	return w, nil
}

// sandboxEnv is the environment konto runs in: everything points into the scenario's sandbox.
func (w *world) sandboxEnv() map[string]string {
	return map[string]string{
		"HOME":                   w.home,
		"PATH":                   w.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TERM":                   "xterm-256color",
		"LANG":                   "C.UTF-8",
		"LEDGER_FILE":            w.journal,
		"KONTO_TODAY":            defaultToday,
		"KONTO_E2E":              "1", // konto reports in its window title when it is idle (cmd/konto/idle.go)
		"XDG_CONFIG_HOME":        filepath.Join(w.home, ".config"),
		"XDG_STATE_HOME":         filepath.Join(w.home, ".local", "state"),
		"XDG_DATA_HOME":          filepath.Join(w.home, ".local", "share"),
		"XDG_CACHE_HOME":         filepath.Join(w.home, ".cache"),
		"XDG_DOWNLOAD_DIR":       w.downloads,
		"KONTO_E2E_SHIM_CONFIG":  w.shimConf,
		"KONTO_E2E_SHIM_LOG":     w.shimLog,
		"KONTO_E2E_REAL_HLEDGER": w.suite.realHledger,
		"EDITOR":                 "/bin/false",
		"VISUAL":                 "",
	}
}

func (w *world) writeShim() error {
	if w.s != nil { // a running konto finishes its first load before hledger starts misbehaving
		w.waitLoaded(w.s)
	}

	b, err := json.Marshal(w.shim)
	if err != nil {
		return err
	}

	return os.WriteFile(w.shimConf, b, 0o644)
}

func (w *world) environ() []string {
	var env []string
	for k, v := range w.env {
		if !w.unset[k] {
			env = append(env, k+"="+v)
		}
	}

	return env
}

// expand replaces the placeholders the features use in paths and arguments.
func (w *world) expand(s string) string {
	return strings.NewReplacer(
		"<tmp>", w.dir,
		"<journal path>", w.journal,
		"<absolute journal path>", w.journal,
	).Replace(s)
}

func (w *world) fixture(name string) string {
	return filepath.Join(w.suite.root, "testdata", "journals", name)
}

// useLedger installs a fixture ledger (testdata/ledgers/<name>: a main journal and year files, ADR 0013)
// beside the shared categories.rules.
func (w *world) useLedger(name string) error {
	dir := filepath.Join(w.suite.root, "testdata", "ledgers", name)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("fixture ledger %q: %w", name, err)
	}
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}

		if err := os.WriteFile(filepath.Join(filepath.Dir(w.journal), entry.Name()), b, 0o644); err != nil {
			return err
		}
	}

	rules, err := os.ReadFile(w.fixture("categories.rules"))
	if err != nil {
		return err
	}

	return os.WriteFile(w.rules, rules, 0o644)
}

// useJournal installs a fixture journal as the main journal. Every fixture
// but "minimal" is paired with the shared categories.rules; minimal gets the
// empty one the first-run skeleton writes.
func (w *world) useJournal(name, appended string) error {
	b, err := os.ReadFile(w.fixture(name + ".journal"))
	if err != nil {
		return fmt.Errorf("fixture journal %q: %w", name, err)
	}

	if err := os.WriteFile(w.journal, append(b, []byte(appended)...), 0o644); err != nil {
		return err
	}

	if err := w.copyIncludes(b); err != nil {
		return err
	}

	rules := []byte{}
	if name != "minimal" {
		if rules, err = os.ReadFile(w.fixture("categories.rules")); err != nil {
			return err
		}
	}

	return os.WriteFile(w.rules, rules, 0o644)
}

func (w *world) start(args ...string) error {
	if w.s != nil {
		return errors.New("konto is already started in this scenario")
	}

	bin, err := w.suite.kontoBinary()
	if err != nil {
		return err
	}
	w.recordFilesAtStart()
	for i, a := range args {
		args[i] = w.expand(a)
	}

	s, err := startSession(bin, args, w.environ(), w.dir, w.width, w.height)
	if err != nil {
		return err
	}
	w.s = s
	w.settle(s)

	return nil
}

// recordFilesAtStart keeps the journal and rules as konto found them, for steps that check what it changed.
func (w *world) recordFilesAtStart() {
	w.journalAtStart, _ = os.ReadFile(w.journal)
	w.filesAtStart = w.journalFiles()
	w.rulesAtStart, _ = os.ReadFile(w.rules)
}

func (w *world) session() (*session, error) {
	if w.s == nil {
		return nil, errors.New("konto has not been started in this scenario")
	}

	return w.s, nil
}

// waitLoaded waits until konto is idle: its first load done, keys sent while "loading journal…" shows
// being (rightly) ignored, and every later load, write or plan landed.
func (w *world) waitLoaded(s *session) { s.settle(w.suite.timeout) }

// settle waits for konto to finish what the last input started. A slowed hledger is there to watch work in
// flight, so then it only waits for the input to be taken in.
func (w *world) settle(s *session) {
	if w.shim.DelayMs > 0 {
		s.handled(w.suite.timeout)

		return
	}
	s.settle(w.suite.timeout)
}

// awaitIdle waits until no load, write or plan is in flight, so what comes next sees its result.
func (w *world) awaitIdle() error {
	s, err := w.session()
	if err != nil {
		return err
	}

	if !s.settle(w.suite.timeout) {
		return fmt.Errorf("konto still busy after %s:\n%s", w.suite.timeout, s.screenText())
	}

	return nil
}

// press sends keys, each once konto is idle, and waits for what each started to finish.
func (w *world) press(keys ...string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	for _, k := range keys {
		b, err := keyBytes(k)
		if err != nil {
			return err
		}
		w.waitLoaded(s)

		if err := s.send(b); err != nil {
			return err
		}
		w.settle(s)
	}

	return nil
}

// spinner is the status bar's mark of a write or reload in flight.
const spinner = "⣾"

// dump writes the failed scenario's screen, stderr and hledger calls.
func (w *world) dump(name string) {
	if w.suite.artifacts == "" || w.s == nil {
		return
	}
	calls, _ := os.ReadFile(w.shimLog)
	report := fmt.Sprintf("== screen ==\n%s\n\n== stderr ==\n%s\n\n== hledger calls ==\n%s", w.s.screenText(), w.s.stderr.String(), calls)
	_ = os.MkdirAll(w.suite.artifacts, 0o755)
	_ = os.WriteFile(filepath.Join(w.suite.artifacts, artifactName(name)), []byte(report), 0o644)
}

// artifactName is the scenario's name made safe as a file name.
func artifactName(scenario string) string {
	return strings.NewReplacer("/", "_", " ", "_", `"`, "_").Replace(scenario) + ".txt"
}

func (w *world) close() {
	for _, earlier := range w.earlier {
		earlier.stop()
	}

	if w.s != nil {
		w.s.stop()
	}

	if os.Getenv("KONTO_E2E_KEEP") == "" {
		_ = os.RemoveAll(w.dir)
	}
}

// copyIncludes puts the fixtures a journal includes by plain file name next to it.
func (w *world) copyIncludes(journal []byte) error {
	for line := range strings.Lines(string(journal)) {
		name, ok := strings.CutPrefix(line, "include ")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)

		b, err := os.ReadFile(w.fixture(name))
		if err != nil {
			return fmt.Errorf("fixture include %q: %w", name, err)
		}

		if err := os.WriteFile(filepath.Join(filepath.Dir(w.journal), name), b, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// appendToJournal adds text to the end of the main journal, as a hand edit or another program would.
func (w *world) appendToJournal(text string) error {
	f, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(text)

	return errors.Join(werr, f.Close())
}

// journalFiles reads every journal file of the sandbox: the main journal and the files beside it.
func (w *world) journalFiles() map[string][]byte {
	files := map[string][]byte{}
	for _, pattern := range []string{"*.journal", "*/*.journal"} {
		paths, _ := filepath.Glob(filepath.Join(filepath.Dir(w.journal), pattern))
		for _, path := range paths {
			files[path], _ = os.ReadFile(path)
		}
	}

	return files
}
