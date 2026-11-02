package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// fakeUI records the messages it takes in and answers each with its next command.
type fakeUI struct {
	got  *[]tea.Msg
	init tea.Cmd
	next tea.Cmd
}

func (f fakeUI) Init() tea.Cmd { return f.init }

func (f fakeUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	*f.got = append(*f.got, msg)
	return f, f.next
}

func (fakeUI) View() tea.View { return tea.NewView("ui") }

type loadedMsg struct{}

func loaded() tea.Msg { return loadedMsg{} }

func signalled(t *testing.T, ui fakeUI) *idleSignal {
	t.Helper()
	model, opts := withIdleSignal(ui, func(key string) string {
		if key == "KONTO_E2E" {
			return "1"
		}
		return ""
	}, os.Stdout)
	if len(opts) != 1 {
		t.Fatalf("want the output option, got %d options", len(opts))
	}
	return model.(*idleSignal)
}

func title(m tea.Model) string { return m.View().WindowTitle }

func TestWithoutKontoE2ETheUIRunsUnwrapped(t *testing.T) {
	ui := fakeUI{got: new([]tea.Msg)}
	model, opts := withIdleSignal(ui, func(string) string { return "" }, os.Stdout)
	if _, ok := model.(fakeUI); !ok || opts != nil {
		t.Errorf("got %T with %d options, want the UI itself and none", model, len(opts))
	}
}

func TestTheSignalIsBusyUntilTheCommandsLand(t *testing.T) {
	got := new([]tea.Msg)
	m := signalled(t, fakeUI{got: got, init: loaded})

	cmd := m.Init()
	if title(m) != "konto busy 0" {
		t.Errorf("title with the first load in flight = %q", title(m))
	}
	_, follow := m.Update(cmd())

	if title(m) != "konto idle 0" || follow != nil {
		t.Errorf("title once it landed = %q, follow-up %v", title(m), follow)
	}
	if len(*got) != 1 || (*got)[0] != (loadedMsg{}) {
		t.Errorf("the UI took in %v, want the load's result", *got)
	}
	if view := m.View(); view.Content != "ui" {
		t.Errorf("view content = %q, want the UI's", view.Content)
	}
}

func TestKeysAndResizesAreCountedOtherMessagesNot(t *testing.T) {
	m := signalled(t, fakeUI{got: new([]tea.Msg)})
	m.Init()

	m.Update(tea.KeyPressMsg{Code: 'q'})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(loadedMsg{})

	if title(m) != "konto idle 2" {
		t.Errorf("title = %q, want two inputs", title(m))
	}
}

func TestACommandWithoutResultEndsUnseen(t *testing.T) {
	got := new([]tea.Msg)
	m := signalled(t, fakeUI{got: got, init: func() tea.Msg { return nil }})

	m.Update(m.Init()())

	if title(m) != "konto idle 0" || len(*got) != 0 {
		t.Errorf("title = %q, UI took in %v; want idle and nothing", title(m), *got)
	}
}

func TestABatchIsTrackedCommandByCommand(t *testing.T) {
	got := new([]tea.Msg)
	m := signalled(t, fakeUI{got: got, init: tea.Batch(loaded, nil, loaded)})

	_, batch := m.Update(m.Init()())
	if title(m) != "konto busy 0" {
		t.Errorf("title with the batch's commands in flight = %q", title(m))
	}
	cmds, ok := batch().(tea.BatchMsg)
	if !ok || len(cmds) != 2 {
		t.Fatalf("batch = %v, want its two commands", cmds)
	}
	m.Update(cmds[0]())
	if title(m) != "konto busy 0" {
		t.Errorf("title with one command left = %q", title(m))
	}
	m.Update(cmds[1]())
	if title(m) != "konto idle 0" || len(*got) != 2 {
		t.Errorf("title = %q, UI took in %v; want idle after both", title(m), *got)
	}
}

func TestTheRuntimesOwnMessagesGoBackToIt(t *testing.T) {
	got := new([]tea.Msg)
	m := signalled(t, fakeUI{got: got, init: tea.Quit})

	_, relay := m.Update(m.Init()())

	if relay == nil || relay() != (tea.QuitMsg{}) || len(*got) != 0 {
		t.Errorf("quit went to the UI (%v) instead of back to the runtime", *got)
	}
	if title(m) != "konto idle 0" {
		t.Errorf("title = %q, want idle", title(m))
	}
}

func TestTheUIsFollowUpCommandsAreTracked(t *testing.T) {
	m := signalled(t, fakeUI{got: new([]tea.Msg), next: loaded})
	m.Init()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})

	if title(m) != "konto busy 1" || cmd == nil {
		t.Errorf("title = %q, want the reload the key started in flight", title(m))
	}
}

func written(t *testing.T, frame string) (out string, n int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err = titleLast{f}.WriteString(frame)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), n
}

func TestTheTitleMovesBehindTheFrame(t *testing.T) {
	cases := map[string]string{
		"a\x1b]2;konto idle 3\ab": "ab\x1b]2;konto idle 3\a",
		"no title":                "no title",
		"a\x1b]2;unclosed":        "a\x1b]2;unclosed",
	}
	for frame, want := range cases {
		if got, n := written(t, frame); got != want || n != len(frame) {
			t.Errorf("Write(%q) wrote %q (n=%d), want %q (n=%d)", frame, got, n, want, len(frame))
		}
	}
}

func TestAFailedFrameWriteIsReported(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if n, err := (titleLast{f}).WriteString("frame"); err == nil || n != 0 {
		t.Errorf("Write on a closed file = %d, %v; want 0 and the error", n, err)
	}
}
