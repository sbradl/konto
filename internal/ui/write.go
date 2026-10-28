package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
)

// prompt is a y/n question about a planned write; y writes it. An import's prompt also shows what will be
// written.
type prompt struct {
	question string
	write    pendingWrite
	preview  []string
	top      int  // the first line of the preview on screen
	quit     bool // y leaves konto: the question is about discarding the staged import
}

// pendingWrite is a changeset on its way to the journal and what follows once it is there.
type pendingWrite struct {
	changeset domain.Changeset
	verify    bool              // checked first, so a journal that would fail its checks is never written
	notice    string            // what the status bar says once it is written
	written   func(Model) Model // what else changes on screen once it is written; nil for nothing
}

// onPromptKey answers the open y/n prompt; every other key leaves it open.
func (m Model) onPromptKey(key string) (Model, tea.Cmd) {
	switch key {
	case "y":
		pending := *m.prompt
		m.prompt = nil
		if pending.quit {
			return m, tea.Quit
		}
		return m.write(pending.write)
	case "n", "esc":
		m.prompt = nil
	case "up":
		m.prompt = m.prompt.scrolled(-1, m.lastPreviewTop())
	case "down":
		m.prompt = m.prompt.scrolled(1, m.lastPreviewTop())
	}
	return m, nil
}

// scrolled is the prompt with its preview moved by lines, between the first line and last, the last line
// the preview can start at; a new prompt, since the one open is shared with earlier models.
func (p *prompt) scrolled(lines, last int) *prompt {
	next := *p
	next.top = max(0, min(p.top+lines, last))
	return &next
}

// write writes the changeset, checking it first if it asks to be. The spinner shows from now until the
// reload after it lands.
func (m Model) write(pending pendingWrite) (Model, tea.Cmd) {
	m.refreshing = true
	return m, func() tea.Msg {
		if pending.verify && m.verifier != nil {
			if err := m.verifier.Verify(context.Background(), pending.changeset); err != nil {
				return writtenMsg{err: err}
			}
		}
		return writtenMsg{err: m.writer.Apply(pending.changeset), pending: pending}
	}
}

// onWritten refreshes after a successful write, saying so and carrying out what follows it, and reports a
// failed one.
func (m Model) onWritten(written writtenMsg) (Model, tea.Cmd) {
	if written.err != nil {
		m.notice, m.refreshing = "write failed: "+written.err.Error(), false
		return m, nil
	}
	if written.pending.notice != "" {
		m.notice = written.pending.notice
	}
	if written.pending.written != nil {
		m = written.pending.written(m)
	}
	return m.reload()
}

// clearImport empties the Import view of the import just written.
func clearImport(m Model) Model {
	m.imports = m.imports.Cleared()
	return m
}
