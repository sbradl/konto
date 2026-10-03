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
)

const (
	defaultToday  = "2026-09-30"
	defaultWidth  = 120
	defaultHeight = 40
	settleQuiet   = 120 * time.Millisecond
	reactWindow   = 250 * time.Millisecond
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
	timeout := 5 * time.Second
	if v := os.Getenv("KONTO_E2E_TIMEOUT"); v != "" {
		if timeout, err = time.ParseDuration(v); err != nil {
			return nil, fmt.Errorf("KONTO_E2E_TIMEOUT: %w", err)
		}
	}
	return &suite{
		root: root, realHledger: realHledger, shim: shim, timeout: timeout,
		artifacts: os.Getenv("KONTO_E2E_ARTIFACTS"),
		konto:     filepath.Join(bin, "konto"),
	}, nil
}

// kontoBinary builds cmd/konto on first use, so harness self-tests and a
// suite of only @wip scenarios run before the binary exists.
func (s *suite) kontoBinary() (string, error) {
	s.buildOnce.Do(func() {
		if _, err := os.Stat(filepath.Join(s.root, "cmd", "konto")); err != nil {
			s.buildErr = errors.New("cmd/konto does not exist yet; tag the scenario @wip until the shell is built")
			return
		}
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

// shimConfig mirrors the hledgershim config file.
type shimConfig struct {
	Version    string   `json:"version,omitempty"`
	DelayMs    int      `json:"delayMs,omitempty"`
	Fail       string   `json:"fail,omitempty"`
	GarbageFor []string `json:"garbageFor,omitempty"`
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

	shim     shimConfig
	shimConf string
	shimLog  string

	s              *session
	earlier        []*session // instances started before the current one
	journalAtStart []byte
	rulesAtStart   []byte
	baseline       string
	tabAtBaseline  string
	lastStepType   string
	focusedRow     string // the checks panel row a step last looked at
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
	w.env = map[string]string{
		"HOME":                   w.home,
		"PATH":                   w.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TERM":                   "xterm-256color",
		"LANG":                   "C.UTF-8",
		"LEDGER_FILE":            w.journal,
		"KONTO_TODAY":            defaultToday,
		"XDG_CONFIG_HOME":        filepath.Join(w.home, ".config"),
		"XDG_STATE_HOME":         filepath.Join(w.home, ".local", "state"),
		"XDG_DATA_HOME":          filepath.Join(w.home, ".local", "share"),
		"XDG_CACHE_HOME":         filepath.Join(w.home, ".cache"),
		"XDG_DOWNLOAD_DIR":       w.downloads,
		"KONTO_E2E_SHIM_CONFIG":  w.shimConf,
		"KONTO_E2E_SHIM_LOG":     w.shimLog,
		"KONTO_E2E_REAL_HLEDGER": s.realHledger,
		"EDITOR":                 "/bin/false",
		"VISUAL":                 "",
	}
	if err := w.writeShim(); err != nil {
		return nil, err
	}
	return w, nil
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
	w.journalAtStart, _ = os.ReadFile(w.journal)
	w.rulesAtStart, _ = os.ReadFile(w.rules)
	for i, a := range args {
		args[i] = w.expand(a)
	}
	s, err := startSession(bin, args, w.environ(), w.dir, w.width, w.height)
	if err != nil {
		return err
	}
	w.s = s
	s.waitFor(w.suite.timeout, func(screen string) bool { return strings.TrimSpace(screen) != "" || s.exited() })
	s.settle(w.suite.timeout)
	return nil
}

func (w *world) session() (*session, error) {
	if w.s == nil {
		return nil, errors.New("konto has not been started in this scenario")
	}
	return w.s, nil
}

// waitLoaded gives konto time to finish its first load before input: keys
// sent while "loading journal…" shows are (rightly) ignored.
func (w *world) waitLoaded(s *session) {
	s.waitFor(w.suite.timeout, func(screen string) bool { return !strings.Contains(screen, "loading journal…") })
}

// press sends keys and waits for the screen to settle.
func (w *world) press(keys ...string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	w.waitLoaded(s)
	for _, k := range keys {
		b, err := keyBytes(k)
		if err != nil {
			return err
		}
		if err := s.send(b); err != nil {
			return err
		}
		s.settle(w.suite.timeout)
	}
	return nil
}

// dump writes the failed scenario's screen, stderr and hledger calls.
func (w *world) dump(name string) {
	if w.suite.artifacts == "" || w.s == nil {
		return
	}
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == ' ' || r == '"' {
			return '_'
		}
		return r
	}, name)
	calls, _ := os.ReadFile(w.shimLog)
	report := fmt.Sprintf("== screen ==\n%s\n\n== stderr ==\n%s\n\n== hledger calls ==\n%s", w.s.screenText(), w.s.stderr.String(), calls)
	_ = os.MkdirAll(w.suite.artifacts, 0o755)
	_ = os.WriteFile(filepath.Join(w.suite.artifacts, safe+".txt"), []byte(report), 0o644)
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
