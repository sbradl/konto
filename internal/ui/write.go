package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
)

// prompt is a y/n question about a planned write; y writes it. An import's prompt also shows what will be
// written.
type prompt struct {
	question string
	write    pendingWrite
	preview  []string
	scroll   layout.Cursor // the preview's first line on screen
	quit     bool          // y leaves konto: the question is about discarding the staged import
}

// pendingWrite is a changeset on its way to the journal and what follows once it is there. Every key that
// plans one first asks busy, and waits while it is: a plan of the snapshot on screen is stale once a write or
// reload is in flight.
type pendingWrite struct {
	changeset domain.Changeset
	verify    bool              // checked first, so a journal that would fail its checks is never written
	notice    string            // what the status bar says once it is written
	written   func(Model) Model // what else changes on screen once it is written; nil for nothing
}

// reloadingNotice asks for a key refused while the model is busy to be pressed again.
const reloadingNotice = "the journal is reloading — try again in a moment"

// busy reports a write or reload in flight: the snapshot on screen is about to change.
func (m Model) busy() bool { return m.writing || m.reloading }

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
	case "n", layout.KeyEsc:
		m.prompt = nil
	default:
		m.prompt = m.prompt.scrolled(key, m.lastPreviewTop(), m.previewCapacity())
	}
	return m, nil
}

// scrolled is the prompt with its preview scrolled by a cursor key, between the first line and last, the last
// line the preview can start at; a new prompt, since the one open is shared with earlier models.
func (p *prompt) scrolled(key string, last, page int) *prompt {
	next := *p
	next.scroll = p.scroll.Scroll(key, last, page)
	return &next
}

// write writes the changeset, checking it first if it asks to be. The spinner shows from now until the
// reload after it lands.
func (m Model) write(pending pendingWrite) (Model, tea.Cmd) {
	m.writing = true
	return m, func() tea.Msg {
		ctx := context.Background()
		changeset, err := m.writer.Prepare(ctx, pending.changeset) // checked and written as prepared, never redone
		if err != nil {
			return writtenMsg{err: err}
		}
		if pending.verify && m.verifier != nil {
			if err := m.verifier.Verify(ctx, changeset); err != nil {
				return writtenMsg{err: err}
			}
		}
		return writtenMsg{err: m.writer.Apply(changeset), pending: pending}
	}
}

// onWritten refreshes after a successful write, saying so and carrying out what follows it, and reports a
// failed one.
func (m Model) onWritten(written writtenMsg) (Model, tea.Cmd) {
	m.writing = false
	if written.err != nil {
		m.notice = "write failed: " + written.err.Error()
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
