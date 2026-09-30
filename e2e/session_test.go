//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// session is one running program on a pseudo-terminal, rendered through a
// virtual terminal so steps can read the screen the way a user sees it.
type session struct {
	cmd *exec.Cmd
	pty *os.File

	mu         sync.Mutex
	term       *vt.Emulator
	raw        bytes.Buffer // every byte the program wrote to the terminal
	lastOutput time.Time
	lastSend   time.Time

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
	s := &session{cmd: cmd, term: vt.NewEmulator(w, h), done: make(chan struct{}), lastOutput: time.Now()}
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
			s.lastOutput = time.Now()
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

func (s *session) send(b string) error {
	s.mu.Lock()
	s.lastSend = time.Now()
	s.mu.Unlock()
	_, err := s.pty.WriteString(b)
	return err
}

func (s *session) resize(w, h int) error {
	s.mu.Lock()
	s.term.Resize(w, h)
	s.mu.Unlock()
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

// settle waits until the program has been quiet for settleQuiet, or exited.
// After input it also gives the program reactWindow to start reacting, so a
// slow reaction is not mistaken for a program that ignored the key.
func (s *session) settle(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && !s.exited() {
		s.mu.Lock()
		idle := time.Since(s.lastOutput)
		reacted := s.lastOutput.After(s.lastSend) || time.Since(s.lastSend) >= reactWindow
		s.mu.Unlock()
		if idle >= settleQuiet && reacted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
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
	w, h := s.term.Width(), s.term.Height()
	lines := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			c := s.term.CellAt(x, y)
			switch {
			case c == nil || c.Content == "" && c.Width == 0:
				// continuation of a wide character
			case c.Content == "":
				b.WriteByte(' ')
			default:
				b.WriteString(c.Content)
			}
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return lines
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
	if bc, ok := c.Style.Fg.(ansi.BasicColor); ok {
		return int(bc)
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
	if bc, ok := c.Style.Bg.(ansi.BasicColor); ok {
		return int(bc)
	}
	return -1
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
			if c == nil || c.Style.Bg == nil {
				continue
			}
			if bc, ok := c.Style.Bg.(ansi.BasicColor); ok {
				seen[int(bc)] = true
			} else {
				seen[-1] = true
			}
		}
	}
	return seen
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
