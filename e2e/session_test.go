//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// session is one running program on a pseudo-terminal, rendered through a
// virtual terminal so steps can read the screen the way a user sees it.
type session struct {
	cmd *exec.Cmd
	pty *os.File

	mu     sync.Mutex
	term   *vt.Emulator
	raw    bytes.Buffer // every byte the program wrote to the terminal
	title  string       // the window title: konto's idle signal, "konto idle N" or "konto busy N"
	expect int          // the inputs konto must have taken in once the last key or resize is handled

	stderr   lockedBuffer
	done     chan struct{}
	exitCode int
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// startSession runs bin on a w×h pty. Stdout and stdin are the pty; stderr is
// captured separately so steps can assert on preflight messages.
func startSession(bin string, args, env []string, dir string, w, h int) (*session, error) {
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	s := &session{cmd: cmd, term: vt.NewEmulator(w, h), done: make(chan struct{})}
	s.term.SetCallbacks(vt.Callbacks{Title: func(title string) { s.title = title }}) // called under s.mu, by pump
	cmd.Stderr = &s.stderr

	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
	if err != nil {
		return nil, err
	}
	s.pty = f
	go s.pump()
	go s.answer()
	go s.wait()

	return s, nil
}

// pump feeds program output into the emulator.
func (s *session) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.raw.Write(buf[:n])
			_, _ = s.term.Write(buf[:n])
			s.mu.Unlock()
		}

		if err != nil {
			return
		}
	}
}

// answer relays the emulator's replies to terminal queries (cursor position,
// device attributes, colours) back to the program, as a real terminal would.
func (s *session) answer() {
	_, _ = io.Copy(s.pty, s.term)
}

func (s *session) wait() {
	err := s.cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		s.exitCode = 0
	case errors.As(err, &exit):
		s.exitCode = exit.ExitCode()
	default:
		s.exitCode = -1
	}
	close(s.done)
}

// send writes keys: one escape sequence or control key, or text, each rune of which konto takes in as a key.
func (s *session) send(b string) error {
	keys := utf8.RuneCountInString(b)
	if strings.HasPrefix(b, "\x1b") {
		keys = 1
	}
	s.expectInputs(keys)
	_, err := s.pty.WriteString(b)

	return err
}

// resize sets the terminal size; only a new size reaches the program (as SIGWINCH), and so its idle signal.
func (s *session) resize(w, h int) error {
	s.mu.Lock()
	same := s.term.Width() == w && s.term.Height() == h
	s.term.Resize(w, h)
	s.mu.Unlock()

	if same {
		return nil
	}
	s.expectInputs(1)

	return pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
}

func (s *session) exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// waitExit waits for the program to end and returns its exit code.
func (s *session) waitExit(timeout time.Duration) (int, error) {
	select {
	case <-s.done:
		return s.exitCode, nil
	case <-time.After(timeout):
		return 0, fmt.Errorf("program still running after %s\n%s", timeout, s.screenText())
	}
}

// expectInputs notes n more inputs konto has to take in: keys or a resize.
func (s *session) expectInputs(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, inputs := s.signal()
	s.expect = max(s.expect, inputs) + n
}

// signal reads konto's idle signal (KONTO_E2E): whether nothing it started is still running, and how many
// inputs it has taken in; -1 inputs before the first frame. Callers hold s.mu.
func (s *session) signal() (idle bool, inputs int) {
	var state string
	if _, err := fmt.Sscanf(s.title, "konto %s %d", &state, &inputs); err != nil {
		return false, -1
	}

	return state == "idle", inputs
}

// settle waits until konto has taken in every key and resize sent and finished what they started: no load,
// write or plan in flight. It returns false on timeout; a program that exited is settled.
func (s *session) settle(timeout time.Duration) bool { return s.awaitSignal(timeout, true) }

// handled waits until konto has taken in every key and resize sent, the work they started maybe still running.
func (s *session) handled(timeout time.Duration) bool { return s.awaitSignal(timeout, false) }

func (s *session) awaitSignal(timeout time.Duration, idle bool) bool {
	deadline := time.Now().Add(timeout)
	for !s.exited() {
		done, late := s.checkSignal(idle, deadline, timeout)
		if done {
			return true
		}

		if late {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}

	return true
}

// checkSignal reads the idle signal once: done when every input is taken in (and, if idle is asked for,
// konto is idle), late when not done by the deadline.
func (s *session) checkSignal(idle bool, deadline time.Time, timeout time.Duration) (done, late bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	quiet, inputs := s.signal()
	done = inputs >= s.expect && (quiet || !idle)

	late = !done && time.Now().After(deadline)
	if late { // said, and the count taken as it is, so one lost input does not stall every later wait
		fmt.Fprintf(os.Stderr, "e2e: no idle signal for %d inputs after %s, title %q\n", s.expect, timeout, s.title)
		s.expect = min(s.expect, max(inputs, 0))
	}

	return done, late
}

// waitFor polls the screen until cond holds.
func (s *session) waitFor(timeout time.Duration, cond func(screen string) bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond(s.screenText()) {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// screenLines returns the visible screen, one string per row, trailing
// spaces trimmed. Wide characters occupy one entry.
func (s *session) screenLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := make([]string, s.term.Height())
	for y := range lines {
		lines[y] = strings.TrimRight(s.rowText(y), " ")
	}

	return lines
}

// rowText is row y's text. Callers hold s.mu.
func (s *session) rowText(y int) string {
	var b strings.Builder
	for x := range s.term.Width() {
		b.WriteString(s.cellText(x, y))
	}

	return b.String()
}

// cellText is what the cell at x, y prints: nothing for the trailing half of a wide character, a space for
// an empty cell.
func (s *session) cellText(x, y int) string {
	c := s.term.CellAt(x, y)
	if c == nil || c.Content == "" && c.Width == 0 {
		return ""
	}

	if c.Content == "" {
		return " "
	}

	return c.Content
}

func (s *session) screenText() string {
	return strings.Join(s.screenLines(), "\n")
}

// cellColour is the foreground of the cell at column x (in cells) of row y as
// an ANSI-16 index, or -1 when the cell has no ANSI-16 foreground.
func (s *session) cellColour(x, y int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.term.CellAt(x, y)
	if c == nil {
		return -1
	}

	return ansi16(c.Style.Fg)
}

// ansi16 is the colour as an ANSI-16 index, or -1 when it is none (default or any other kind of colour).
func ansi16(colour color.Color) int {
	if basic, ok := colour.(ansi.BasicColor); ok {
		return int(basic)
	}

	return -1
}

// cellBackground is the background of the cell at column x of row y as an
// ANSI-16 index, or -1 when the cell has no ANSI-16 background.
func (s *session) cellBackground(x, y int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.term.CellAt(x, y)
	if c == nil {
		return -1
	}

	return ansi16(c.Style.Bg)
}

// attrBold is the bold bit of a cell style (ultraviolet's AttrBold, which this harness need not import).
const attrBold = 1

// cellBold reports whether the cell is drawn bold.
func (s *session) cellBold(x, y int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.term.CellAt(x, y)

	return c != nil && c.Style.Attrs&attrBold != 0
}

// backgrounds returns the distinct non-default background colours on screen,
// as ANSI-16 indices, or -1 for any other kind of colour.
func (s *session) backgrounds() map[int]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[int]bool{}
	for y := range s.term.Height() {
		for x := range s.term.Width() {
			c := s.term.CellAt(x, y)
			if c != nil && c.Style.Bg != nil {
				seen[ansi16(c.Style.Bg)] = true
			}
		}
	}

	return seen
}

// colours are the foreground colours of every occurrence of text on screen (-1 where not ANSI-16).
func (s *session) colours(text string) []int {
	var seen []int
	for y, line := range s.screenLines() {
		for _, x := range columnsOf(line, text) {
			seen = append(seen, s.cellColour(x, y))
		}
	}

	return seen
}

// columnsOf is the cell column of every occurrence of text in line.
func columnsOf(line, text string) []int {
	var columns []int
	for offset := 0; ; {
		before, _, found := strings.Cut(line[offset:], text)
		if !found {
			return columns
		}
		columns = append(columns, ansi.StringWidth(line[:offset+len(before)]))
		offset += len(before) + len(text)
	}
}

// find returns the cell position of the first occurrence of text on screen.
func (s *session) find(text string) (x, y int, ok bool) {
	for y, line := range s.screenLines() {
		if before, _, found := strings.Cut(line, text); found {
			return ansi.StringWidth(before), y, true
		}
	}

	return 0, 0, false
}

func (s *session) rawOutput() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return bytes.Clone(s.raw.Bytes())
}

// stop ends the program if it still runs and releases the pty.
func (s *session) stop() {
	if !s.exited() {
		_ = s.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			_ = s.cmd.Process.Kill()
			<-s.done
		}
	}
	_ = s.pty.Close()
	s.mu.Lock()
	_ = s.term.Close()
	s.mu.Unlock()
}
