package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"slices"

	tea "charm.land/bubbletea/v2"
)

// withIdleSignal is the UI as the e2e harness runs it ($KONTO_E2E set): the window title reports, as
// "konto idle N" or "konto busy N", whether a command the UI started is still running, N the inputs (keys,
// resizes) taken in so far. The harness waits on it instead of on quiet output. Otherwise the UI is unwrapped.
func withIdleSignal(model tea.Model, getenv func(string) string, out *os.File) (tea.Model, []tea.ProgramOption) {
	if getenv("KONTO_E2E") == "" {
		return model, nil
	}
	return &idleSignal{inner: model}, []tea.ProgramOption{tea.WithOutput(titleLast{out})}
}

// idleSignal counts the UI's commands in flight: each is wrapped to report back as a commandDone.
type idleSignal struct {
	inner   tea.Model
	pending int
	inputs  int
}

// commandDone carries what a tracked command returned.
type commandDone struct{ msg tea.Msg }

func (m *idleSignal) Init() tea.Cmd { return m.track(m.inner.Init()) }

func (m *idleSignal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commandDone:
		m.pending--
		switch done := msg.msg.(type) {
		case nil:
			return m, nil
		case tea.BatchMsg:
			cmds := make([]tea.Cmd, len(done))
			for i, cmd := range done {
				cmds[i] = m.track(cmd)
			}
			return m, tea.Batch(cmds...)
		}
		// shortcut: a sequence or an exec runs untracked; the UI uses neither for loads or writes.
		if fromRuntime(msg.msg) {
			return m, func() tea.Msg { return msg.msg }
		}
		return m.forward(msg.msg)
	case tea.KeyPressMsg, tea.WindowSizeMsg:
		m.inputs++
	}
	return m.forward(msg)
}

// fromRuntime reports a message of the runtime's own (quit, exec, …): it goes back to the runtime untracked.
func fromRuntime(msg tea.Msg) bool {
	return reflect.TypeOf(msg).PkgPath() == reflect.TypeFor[tea.QuitMsg]().PkgPath()
}

func (m *idleSignal) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.inner.Update(msg)
	m.inner = next
	cmd = m.track(cmd)
	return m, cmd
}

func (m *idleSignal) track(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	m.pending++
	return func() tea.Msg { return commandDone{cmd()} }
}

func (m *idleSignal) View() tea.View {
	view := m.inner.View()
	state := "idle"
	if m.pending > 0 {
		state = "busy"
	}
	view.WindowTitle = fmt.Sprintf("konto %s %d", state, m.inputs)
	return view
}

// titleLast is the terminal with each frame's window title moved behind the frame, so the harness reads a
// frame's title only once the whole frame has landed. It stays an *os.File for the runtime's size and colour
// probes.
type titleLast struct{ *os.File }

// setTitle starts the window title sequence; BEL ends it.
const setTitle = "\x1b]2;"

func (t titleLast) Write(frame []byte) (int, error) {
	out := frame
	if before, rest, found := bytes.Cut(frame, []byte(setTitle)); found {
		if title, after, closed := bytes.Cut(rest, []byte("\a")); closed {
			out = slices.Concat(before, after, []byte(setTitle), title, []byte("\a"))
		}
	}
	if _, err := t.File.Write(out); err != nil {
		return 0, fmt.Errorf("write frame: %w", err)
	}
	return len(frame), nil
}

// WriteString writes through Write, so a frame written as a string has its title moved too.
func (t titleLast) WriteString(frame string) (int, error) {
	return t.Write([]byte(frame)) //nolint:gocritic // WriteString would call itself
}
