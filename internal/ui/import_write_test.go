package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
)

func euros(mantissa int64) domain.MultiAmount {
	style := domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}}
}

func dated(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

// writable is a categorised import of two rows that brings checking from 13.240,00 to the bank's balance.
func writable() csvimport.Staged {
	return csvimport.Staged{
		File: "sept.csv", Format: "postbank", Closing: "13.195,83", Zero: euros(0),
		Account: domain.Account{Name: "assets:checking", Balance: euros(1324000)},
		Rows: []csvimport.Row{
			{Date: dated("2026-09-30"), Description: "REWE Markt", Amount: euros(-4217), Target: "expenses:groceries", RuleID: "2"},
			{Date: dated("2026-09-26"), Description: "Kiosk", Amount: euros(-200), Target: "expenses:books", Once: true},
		},
	}
}

type fakeVerifier struct {
	checked []domain.Changeset
	err     error
}

func (f *fakeVerifier) Verify(_ context.Context, changeset domain.Changeset) error {
	f.checked = append(f.checked, changeset)
	return f.err
}

type writeRig struct {
	model    Model
	writer   *fakeWriter
	verifier *fakeVerifier
}

// stagedForWrite opens the staged import in the Import view of a model that can write.
func stagedForWrite(t *testing.T, staged csvimport.Staged) writeRig {
	t.Helper()
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: staged}
	writer, verifier := &fakeWriter{}, &fakeVerifier{}
	model := New(journalPath, Ports{
		Loader: fakeLoader{}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Importer: importer, Writer: writer, Verifier: verifier, DownloadDir: "/dl",
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())
	opened, _ := press(updated.(Model), "2")
	opened, cmd := press(opened, "n")
	opened = deliver(opened, cmd)
	opened, _ = pressCode(opened, tea.KeyDown)
	opened, cmd = pressCode(opened, tea.KeyEnter)
	return writeRig{model: deliver(opened, cmd), writer: writer, verifier: verifier}
}

func TestWBlocksWhileRowsNeedACategoryAndWritesNothing(t *testing.T) {
	staged := writable()
	staged.Rows[1].Target = ""
	rig := stagedForWrite(t, staged)

	model, _ := press(rig.model, "w")

	if content := plain(model.View().Content); !strings.Contains(content, "1 row needs a category") ||
		model.prompt != nil || len(rig.writer.applied) != 0 {
		t.Errorf("prompt = %v, applied = %v; content:\n%s", model.prompt, rig.writer.applied, content)
	}
}

func TestWAsksYNAndShowsWhatWillBeWritten(t *testing.T) {
	model, _ := press(stagedForWrite(t, writable()).model, "w")

	content := plain(model.View().Content)
	for _, want := range []string{"─ write ", "2026-09-30 REWE Markt", "expenses:groceries  42,17 EUR  ; konto-rule:2", "╭─ ROWS "} {
		if !strings.Contains(content, want) {
			t.Errorf("screen lacks %q:\n%s", want, content)
		}
	}
	if got := lastLine(model); !strings.Contains(got, "write 2 transactions into assets:checking? (y/n)") {
		t.Errorf("status bar = %q", got)
	}
}

func TestOneTransactionIsSingular(t *testing.T) {
	staged := writable()
	staged.Rows, staged.Closing = staged.Rows[:1], "13.197,83"

	model, _ := press(stagedForWrite(t, staged).model, "w")

	if got := lastLine(model); !strings.Contains(got, "write 1 transaction into") {
		t.Errorf("status bar = %q", got)
	}
}

func TestNAndEscCancelTheWriteAndTheStagedImportSurvives(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		rig := stagedForWrite(t, writable())
		model, _ := press(rig.model, "w")

		if key == "esc" {
			model, _ = pressCode(model, tea.KeyEscape)
		} else {
			model, _ = press(model, key)
		}

		if content := plain(model.View().Content); model.prompt != nil || !strings.Contains(content, "╭─ ROWS ") ||
			strings.Contains(content, "─ write ") || len(rig.writer.applied) != 0 {
			t.Errorf("%s: prompt = %v; content:\n%s", key, model.prompt, content)
		}
	}
}

func TestYChecksThenWritesAndThenConfirmsAndEmptiesTheImport(t *testing.T) {
	rig := stagedForWrite(t, writable())
	model, _ := press(rig.model, "w")

	model, cmd := press(model, "y")
	written := cmd()

	if len(rig.verifier.checked) != 1 || len(rig.writer.applied) != 1 ||
		len(rig.verifier.checked[0].Edits) != len(rig.writer.applied[0].Edits) {
		t.Fatalf("checked %d, applied %d; want the same changeset checked and written once", len(rig.verifier.checked), len(rig.writer.applied))
	}
	updated, reload := model.Update(written)
	done := updated.(Model)
	if !done.refreshing || reload == nil {
		t.Error("a successful write refreshes the views")
	}
	if got := lastLine(done); !strings.Contains(got, "imported 2 transactions into assets:checking") {
		t.Errorf("status bar = %q", got)
	}
	if content := plain(done.View().Content); !strings.Contains(content, "press n for a new file") {
		t.Errorf("the import is not emptied:\n%s", content)
	}
}

func TestAFailedPreCheckWritesNothingAndKeepsTheImport(t *testing.T) {
	rig := stagedForWrite(t, writable())
	rig.verifier.err = errors.New("the journal would fail its checks: balance assertion")
	model, _ := press(rig.model, "w")
	model, cmd := press(model, "y")

	updated, _ := model.Update(cmd())

	done := updated.(Model)
	if len(rig.writer.applied) != 0 || !strings.Contains(lastLine(done), "write failed: the journal would fail its checks") ||
		!strings.Contains(plain(done.View().Content), "╭─ ROWS ") {
		t.Errorf("applied = %v, status = %q", rig.writer.applied, lastLine(done))
	}
}

func TestAFailedWriteKeepsTheImport(t *testing.T) {
	rig := stagedForWrite(t, writable())
	rig.writer.err = errors.New("disk full")
	model, _ := press(rig.model, "w")
	model, cmd := press(model, "y")

	updated, _ := model.Update(cmd())

	done := updated.(Model)
	if !strings.Contains(lastLine(done), "write failed: disk full") || !strings.Contains(plain(done.View().Content), "╭─ ROWS ") {
		t.Errorf("status = %q", lastLine(done))
	}
}

func TestAnExportThatMissesTheBanksBalanceIsBlockedWithTheFigures(t *testing.T) {
	staged := writable()
	staged.Closing = "13.182,03"
	rig := stagedForWrite(t, staged)

	model, _ := press(rig.model, "w")

	content := plain(model.View().Content)
	if !strings.Contains(content, "bank 13.182,03 EUR · journal 13.195,83 EUR · difference -13,80 EUR") ||
		model.prompt != nil || len(rig.writer.applied) != 0 || !strings.Contains(content, "╭─ ROWS ") {
		t.Errorf("prompt = %v; content:\n%s", model.prompt, content)
	}
}

func TestAnImportWithNothingNewHasNothingToWrite(t *testing.T) {
	staged := writable()
	staged.Rows = nil

	model, _ := press(stagedForWrite(t, staged).model, "w")

	if content := plain(model.View().Content); !strings.Contains(content, "nothing new to write") || model.prompt != nil {
		t.Errorf("content:\n%s", content)
	}
}

func TestWOnTheEmptyImportAsksForNothing(t *testing.T) {
	model := startedWithImporter(t, &fakeImporter{})

	model, _ = press(model, "w")

	if model.prompt != nil {
		t.Error("a prompt opened with nothing staged")
	}
}

func manyRows(n int) csvimport.Staged {
	staged := writable()
	staged.Rows = nil
	for range n {
		staged.Rows = append(staged.Rows, csvimport.Row{
			Date: dated("2026-09-30"), Description: "Kiosk", Amount: euros(-100), Target: "expenses:books", Once: true,
		})
	}
	staged.Account.Balance = euros(int64(1324000))
	staged.Closing = "13.230,00"
	staged.Rows[0].Description = "Newest"
	return staged
}

func TestALongPreviewScrollsWithTheArrowKeys(t *testing.T) {
	model, _ := press(stagedForWrite(t, manyRows(10)).model, "w")
	if content := plain(model.View().Content); !strings.Contains(content, "Newest") || strings.Contains(content, "payee Newest") {
		t.Fatalf("the preview starts with the newest transaction:\n%s", content)
	}

	for range 40 {
		model, _ = pressCode(model, tea.KeyDown)
	}
	scrolled := plain(model.View().Content)
	if strings.Contains(scrolled, "2026-09-30 Newest") || !strings.Contains(scrolled, "payee Newest") {
		t.Errorf("the preview did not scroll to its end:\n%s", scrolled)
	}
	for range 80 {
		model, _ = pressCode(model, tea.KeyUp)
	}
	if content := plain(model.View().Content); !strings.Contains(content, "2026-09-30 Newest") {
		t.Errorf("the preview did not scroll back:\n%s", content)
	}
}

func TestOpeningAFileWhileTheJournalReloadsAsksToTryAgain(t *testing.T) {
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: sampleStaged()}
	model, cmd := press(startedWithImporter(t, importer), "n")
	model = deliver(model, cmd)
	model.refreshing = true

	model, _ = pressCode(model, tea.KeyDown)
	model, cmd = pressCode(model, tea.KeyEnter)

	if content := plain(model.View().Content); cmd != nil || len(importer.opened) != 0 ||
		!strings.Contains(content, "still reloading the journal") {
		t.Errorf("opened %v; content:\n%s", importer.opened, content)
	}
}

func TestAPreviewThatFitsShowsEveryLineOfItIncludingTheLast(t *testing.T) {
	one := manyRows(1)
	one.Closing = "13.239,00"
	model, _ := press(stagedForWrite(t, one).model, "w")

	content := plain(model.View().Content)
	if model.prompt == nil {
		t.Fatalf("no prompt:\n%s", content)
	}
	lines := model.prompt.preview

	if len(lines) < 2 || !strings.Contains(content, strings.TrimSpace(lines[len(lines)-1])) ||
		!strings.Contains(content, strings.TrimSpace(lines[0])) {
		t.Errorf("a preview of %d lines should show in full:\n%s", len(lines), content)
	}
}
