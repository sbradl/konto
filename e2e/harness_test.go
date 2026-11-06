//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/e2e/shimconf"
)

// These tests pin the harness itself, so a green feature run means konto
// behaved, not that the harness looked away.

func TestKeyBytes(t *testing.T) {
	cases := map[string]string{
		"q": "q", "?": "?", "↓": "\x1b[B", "Enter": "\r", "Esc": "\x1b", "Shift+Tab": "\x1b[Z",
		"PgDn": "\x1b[6~", "F5": "\x1b[15~", "Insert": "\x1b[2~",
		"Ctrl+C": "\x03", "Ctrl+c": "\x03", "Ctrl+X": "\x18", "Ctrl+r": "\x12",
		"Alt+1": "\x1b1", "Shift+q": "Q",
	}
	for name, want := range cases {
		if got, err := keyBytes(name); err != nil || got != want {
			t.Errorf("keyBytes(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	for _, bad := range []string{"Hyper+x", "Ctrl+1", "Ctrl+ab", "Banana"} {
		if _, err := keyBytes(bad); err == nil {
			t.Errorf("keyBytes(%q) should fail", bad)
		}
	}
}

func TestContainsBlock(t *testing.T) {
	text := "2026-09-29 Kiosk  ; Zeitung\n    assets:checking   -1,00 EUR\n    expenses:news\n"
	if !containsBlock(text, "\n2026-09-29 Kiosk ; Zeitung\n  assets:checking -1,00 EUR\n") {
		t.Error("whitespace-normalised consecutive lines should match")
	}

	if containsBlock(text, "2026-09-29 Kiosk ; Zeitung\nexpenses:news") {
		t.Error("non-consecutive lines must not match")
	}

	if containsBlock(text, "assets:checking -2,00 EUR") {
		t.Error("different content must not match")
	}
}

func TestPostbankExportLayout(t *testing.T) {
	e := postbankExport{IBAN: "DE89370400440532013000", ClosingBalance: "14.261,45", Rows: []postbankRow{
		{Date: "30.9.2026", Type: "Kartenzahlung", Payee: "Kiosk", Purpose: "Zeitung", Amount: "-2,00"},
		{Date: "29.9.2026", Type: "Gutschrift", Payee: "Oma", Purpose: "Geburtstag", Amount: "1.023,45"},
	}}

	b, err := e.render()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(b, []byte("\xef\xbb\xbfUmsätze\n")) {
		t.Errorf("export must start with a UTF-8 BOM and the title, got %q", b[:12])
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	want := map[int]string{
		2:  "Postbank Giro plus;0123456789;DE89370400440532013000;EUR",
		3:  "",
		4:  "29.9.2026 - 30.9.2026",
		5:  "Letzter Kontostand;;;;14.261,45;EUR",
		7:  postbankHeader,
		8:  "30.9.2026;30.9.2026;Kartenzahlung;Kiosk;Zeitung;DE00000000000000000001;ABCDEFGH;ref1;;;;-2,00;;;;-2,00;;EUR",
		9:  "29.9.2026;29.9.2026;Gutschrift;Oma;Geburtstag;DE00000000000000000002;ABCDEFGH;ref2;;;;1.023,45;;;;;1.023,45;EUR",
		10: "Kontostand;30.9.2026;;;14.261,45;EUR",
	}

	if len(lines) != 11 {
		t.Fatalf("want 11 lines, got %d:\n%s", len(lines), b)
	}
	for i, l := range want {
		if lines[i] != l {
			t.Errorf("line %d = %q, want %q", i+1, lines[i], l)
		}
	}
	for _, r := range [][]string{strings.Split(lines[8], ";"), strings.Split(lines[7], ";")} {
		if len(r) != 18 {
			t.Errorf("rows and header have 18 columns, got %d", len(r))
		}
	}
}

func TestPostbankExportVariants(t *testing.T) {
	row := postbankRow{Date: "29.9.2026", Type: "T", Payee: "P", Purpose: "x", Amount: "-1,00"}

	crlf, _ := postbankExport{IBAN: "DE1", ClosingBalance: "1,00", Rows: []postbankRow{row}, CRLF: true}.render()
	if bytes.Count(crlf, []byte("\r\n")) != bytes.Count(crlf, []byte("\n")) {
		t.Error("CRLF export must end every line with CRLF")
	}

	blank, _ := postbankExport{IBAN: "DE1", ClosingBalance: "1,00", Rows: []postbankRow{row}, TrailingBlank: true}.render()
	if !bytes.HasSuffix(blank, []byte("EUR\n\n")) {
		t.Errorf("trailing blank line missing: %q", blank[len(blank)-10:])
	}

	broken, _ := postbankExport{IBAN: "DE1", ClosingBalance: "1,00", Rows: []postbankRow{row}, BrokenRow: "29.9.2026"}.render()
	if !bytes.Contains(broken, []byte("EUR\n29.9.2026\nKontostand;")) {
		t.Error("broken row must sit between the rows and the trailer")
	}

	if _, err := (postbankExport{}).render(); err == nil {
		t.Error("an export without rows must be refused")
	}

	if _, err := (postbankExport{Rows: []postbankRow{{Date: "2026-09-29"}}}).render(); err == nil {
		t.Error("a non-German date must be refused")
	}
}

// startScript runs a shell script on a 60×12 pty for the session tests.
func startScript(t *testing.T, script string) *session {
	t.Helper()

	s, err := startSession("/bin/sh", []string{"-c", script}, os.Environ(), t.TempDir(), 60, 12)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.stop)

	return s
}

func waitForText(t *testing.T, s *session, text string) {
	t.Helper()

	if !s.waitFor(3*time.Second, func(sc string) bool { return strings.Contains(sc, text) }) {
		t.Fatalf("%q never appeared on screen:\n%s", text, s.screenText())
	}
}

func TestSessionReadsTextAndANSIColour(t *testing.T) {
	s := startScript(t, `printf '\033[31mred\033[0m plain\n'; sleep 5`)
	waitForText(t, s, "plain")

	if x, y, _ := s.find("red"); s.cellColour(x, y) != 1 {
		t.Errorf("\"red\" should be ANSI 1, got %d", s.cellColour(x, y))
	}

	if x, y, _ := s.find("plain"); s.cellColour(x, y) != -1 {
		t.Error("unstyled text should report no ANSI colour")
	}

	if !bytes.Contains(s.rawOutput(), []byte("\x1b[31m")) {
		t.Error("raw output should keep escape sequences")
	}
}

func TestSessionSendsInput(t *testing.T) {
	s := startScript(t, `read line; echo "got:$line"; sleep 5`)
	if err := s.send("hello\r"); err != nil {
		t.Fatal(err)
	}
	waitForText(t, s, "got:hello")
}

// settle waits for the idle signal to count the keys sent (x and Enter); handled does not wait for idle.
func TestSessionSettlesOnTheIdleSignal(t *testing.T) {
	s := startScript(t, `printf '\033]2;konto idle 0\a'; read line; printf '\033]2;konto busy 2\a'; sleep 0.3; printf 'done\033]2;konto idle 2\a'; sleep 5`)
	if !s.settle(3 * time.Second) {
		t.Fatal("never idle at start")
	}

	if err := s.send("x\r"); err != nil {
		t.Fatal(err)
	}

	if !s.handled(3*time.Second) || strings.Contains(s.screenText(), "done") {
		t.Errorf("handled should return once the key is taken in, before the work ends:\n%s", s.screenText())
	}

	if !s.settle(3*time.Second) || !strings.Contains(s.screenText(), "done") {
		t.Errorf("settle should return once idle again, after the work:\n%s", s.screenText())
	}
}

func TestSessionStartsAtSizeAndResizes(t *testing.T) {
	s := startScript(t, `stty size; trap 'stty size; exit 7' WINCH; while :; do sleep 0.05; done`)
	waitForText(t, s, "12 60")

	if err := s.resize(80, 20); err != nil {
		t.Fatal(err)
	}

	if code, err := s.waitExit(3 * time.Second); err != nil || code != 7 {
		t.Fatalf("exit = %d, %v; want 7", code, err)
	}
	waitForText(t, s, "20 80")
}

func TestSessionCapturesStderrApartFromScreen(t *testing.T) {
	s := startScript(t, `echo shown; echo oops >&2`)
	if _, err := s.waitExit(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	waitForText(t, s, "shown")

	if !strings.Contains(s.stderr.String(), "oops") || strings.Contains(s.screenText(), "oops") {
		t.Errorf("stderr %q must be captured apart from the screen", s.stderr.String())
	}
}

// shimWorld gives a scenario sandbox with the built shim as "hledger".
func shimWorld(t *testing.T) *world {
	t.Helper()

	s, err := newSuite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(s.buildDir()) })

	w, err := newWorld(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.close)

	return w
}

func runShim(t *testing.T, w *world, cfg shimconf.Config, args ...string) (string, error) {
	t.Helper()

	return runShimFed(t, w, cfg, "", args...)
}

// runShimFed runs the shim with stdin on its standard input.
func runShimFed(t *testing.T, w *world, cfg shimconf.Config, stdin string, args ...string) (string, error) {
	t.Helper()

	w.shim = cfg
	if err := w.writeShim(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), filepath.Join(w.binDir, "hledger"), args...)
	cmd.Env, cmd.Stdin = w.environ(), strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()

	return string(out), err
}

func TestShimPassesThroughToRealHledger(t *testing.T) {
	w := shimWorld(t)
	if out, err := runShim(t, w, shimconf.Config{}, "--version"); err != nil || !strings.HasPrefix(out, "hledger 1.") {
		t.Errorf("passthrough --version = %q, %v", out, err)
	}
}

func TestShimFakesVersionAndFailures(t *testing.T) {
	w := shimWorld(t)

	cfg := shimconf.Config{Version: "1.31", Fail: "boom"}
	if out, _ := runShim(t, w, cfg, "--version"); !strings.HasPrefix(out, "hledger 1.31,") {
		t.Errorf("faked --version = %q", out)
	}

	if out, err := runShim(t, w, cfg, "-f", "x", "bal"); err == nil || !strings.Contains(out, "boom") {
		t.Errorf("failing shim = %q, %v", out, err)
	}

	if out, err := runShim(t, w, shimconf.Config{GarbageFor: []string{"bal"}}, "-f", "x", "bal"); err != nil ||
		!strings.Contains(out, "not json") {
		t.Errorf("garbage shim = %q, %v", out, err)
	}
}

// A config field the shim does not know fails the call instead of being ignored.
func TestShimRefusesAnUnknownConfigField(t *testing.T) {
	w := shimWorld(t)
	if err := os.WriteFile(w.shimConf, []byte(`{"delay": 5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), filepath.Join(w.binDir, "hledger"), "--version")

	cmd.Env = w.environ()
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "unknown field") {
		t.Errorf("shim with an unknown config field = %q, %v", out, err)
	}
}

func TestShimRecordsCallsWithoutVersionProbes(t *testing.T) {
	w := shimWorld(t)
	cfg := shimconf.Config{Version: "1.40", GarbageFor: []string{"bal"}}
	for _, args := range [][]string{{"--version"}, {"-f", "x", "bal"}} {
		if _, err := runShim(t, w, cfg, args...); err != nil {
			t.Fatal(err)
		}
	}

	calls, err := w.hledgerCalls()
	if err != nil || len(calls) != 1 || strings.Join(calls[0], " ") != "-f x bal" {
		t.Errorf("recorded calls = %v, %v", calls, err)
	}

	if err := w.expectEveryCallNamesTheJournal(); err == nil {
		t.Error("a call without the journal path must be reported")
	}
}

// A `-f -` call is logged with its stdin, which the real hledger still gets.
func TestShimRecordsAndPassesOnStdin(t *testing.T) {
	w := shimWorld(t)
	if err := os.WriteFile(w.journal, []byte("2026-01-01 x\n    a    1\n    b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := "include " + w.journal + "\n"

	out, err := runShimFed(t, w, shimconf.Config{}, input, "-f", "-", "print")

	if err != nil || !strings.Contains(out, "2026-01-01 x") {
		t.Errorf("fed shim = %q, %v; want hledger to read the journal through stdin", out, err)
	}

	if err := w.expectEveryCallNamesTheJournal(); err != nil {
		t.Error(err)
	}

	if err := w.expectEveryCallIncludes("2026"); err == nil {
		t.Error("a call without the year file must be reported")
	}

	if out, err := runShimFed(t, w, shimconf.Config{}, "include /nowhere.journal\n", "-f", "-", "print"); err == nil {
		t.Errorf("fed shim = %q, want hledger's failure passed on", out)
	}
}

// fixtureWorld is a world with only what useJournal needs.
func fixtureWorld(t *testing.T) *world {
	t.Helper()

	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	w := &world{suite: &suite{root: root}, dir: t.TempDir()}
	w.journal, w.rules = filepath.Join(w.dir, "main.journal"), filepath.Join(w.dir, "categories.rules")

	return w
}

func realHledger(t *testing.T) string {
	t.Helper()

	path, err := exec.LookPath("hledger")
	if err != nil {
		t.Fatal("the e2e suite needs hledger on PATH")
	}

	return path
}

func TestFixtureRulesPairing(t *testing.T) {
	w := fixtureWorld(t)
	for fixture, wantRules := range map[string]bool{"minimal": false, "household": true} {
		if err := w.useJournal(fixture, ""); err != nil {
			t.Fatal(err)
		}

		if b, _ := os.ReadFile(w.rules); (len(b) > 0) != wantRules {
			t.Errorf("%s: categories.rules non-empty = %v, want %v", fixture, len(b) > 0, wantRules)
		}
	}

	if err := w.useJournal("no-such-fixture", ""); err == nil {
		t.Error("an unknown fixture must be reported")
	}
}

func TestFixtureWithAppendedText(t *testing.T) {
	w := fixtureWorld(t)
	if err := w.useJournal("household", "\n; appended\n"); err != nil {
		t.Fatal(err)
	}

	if err := expectFileBlock(w.journal, "; appended"); err != nil {
		t.Error(err)
	}
}

func TestChecksPassOnFixturesAndFailOnBrokenJournal(t *testing.T) {
	w := fixtureWorld(t)
	if err := w.useJournal("household", ""); err != nil {
		t.Fatal(err)
	}

	if err := runChecks(realHledger(t), w.journal, true); err != nil {
		t.Errorf("household fixture must pass every check: %v", err)
	}

	if err := os.WriteFile(w.journal, []byte("2026-01-01 x\n    a  1 EUR\n    b  2 EUR\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runChecks(realHledger(t), w.journal, false); err == nil {
		t.Error("an unbalanced journal must fail the checks")
	}
}
