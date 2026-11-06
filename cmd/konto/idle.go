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

		return m.settle(msg.msg)
	case tea.KeyPressMsg, tea.WindowSizeMsg:
		m.inputs++
	}

	return m.forward(msg)
}

// settle handles what a finished command returned: a batch is tracked command by command, a runtime message
// goes back to the runtime, anything else reaches the UI.
func (m *idleSignal) settle(result tea.Msg) (tea.Model, tea.Cmd) {
	switch result := result.(type) {
	case nil:
		return m, nil
	case tea.BatchMsg:
		cmd := m.trackBatch(result)

		return m, cmd
	}
	// shortcut: a sequence or an exec runs untracked: no load or write uses them (e2e editor steps poll).
	if fromRuntime(result) {
		return m, func() tea.Msg { return result }
	}

	return m.forward(result)
}

func (m *idleSignal) trackBatch(batch tea.BatchMsg) tea.Cmd {
	cmds := make([]tea.Cmd, len(batch))
	for i, cmd := range batch {
		cmds[i] = m.track(cmd)
	}

	return tea.Batch(cmds...)
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
	if _, err := t.File.Write(titleBehindFrame(frame)); err != nil {
		return 0, fmt.Errorf("write frame: %w", err)
	}

	return len(frame), nil
}

// titleBehindFrame moves frame's window title sequence, if it has a complete one, to the end of the frame.
func titleBehindFrame(frame []byte) []byte {
	before, rest, found := bytes.Cut(frame, []byte(setTitle))
	if !found {
		return frame
	}

	title, after, closed := bytes.Cut(rest, []byte("\a"))
	if !closed {
		return frame
	}

	return slices.Concat(before, after, []byte(setTitle), title, []byte("\a"))
}

// WriteString writes through Write, so a frame written as a string has its title moved too.
func (t titleLast) WriteString(frame string) (int, error) {
	return t.Write([]byte(frame)) //nolint:gocritic // WriteString would call itself
}
