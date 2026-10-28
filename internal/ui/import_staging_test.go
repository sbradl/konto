package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
)

// stagedAt is writable() read from a path, with its first row chosen once.
func stagedAt() csvimport.Staged {
	staged := writable()
	staged.Path = "/dl/sept.csv"
	return staged
}

// reread is the export as hledger gives it again: nothing categorised yet but what the disk's rules say.
func reread() csvimport.Staged {
	staged := stagedAt()
	staged.Rows = []csvimport.Row{staged.Rows[0], staged.Rows[1]}
	staged.Rows[1].Target, staged.Rows[1].Once = "", false
	return staged
}

type rereadingImporter struct {
	fakeImporter
	ledgers []domain.Ledger
	reread  csvimport.Staged
	err     error
}

func (r *rereadingImporter) Open(_ context.Context, path string, ledger domain.Ledger) (csvimport.Staged, error) {
	r.opened = append(r.opened, path)
	r.ledgers = append(r.ledgers, ledger)
	if len(r.opened) == 1 {
		return stagedAt(), nil
	}
	return r.reread, r.err
}

// listing is the download folder with the one export in it.
func listing() *rereadingImporter {
	importer := &rereadingImporter{}
	importer.entries = []domain.FileEntry{{Name: "sept.csv"}}
	return importer
}

func stagedModel(t *testing.T, importer csvImporter) Model {
	t.Helper()
	model := New(journalPath, Ports{
		Loader: fakeLoader{}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Importer: importer, Writer: &fakeWriter{}, DownloadDir: "/dl",
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())
	opened, _ := press(updated.(Model), "2")
	opened, cmd := press(opened, "n")
	opened = deliver(opened, cmd)
	opened, _ = pressCode(opened, tea.KeyDown)
	opened, cmd = pressCode(opened, tea.KeyEnter)
	return deliver(opened, cmd)
}

func TestRefreshReadsTheStagedExportAgainAndKeepsWhatWasChosen(t *testing.T) {
	importer := listing()
	importer.reread = reread()
	model := stagedModel(t, importer)

	model, cmd := press(model, "r")
	model = deliver(model, cmd)

	if len(importer.opened) != 2 || importer.opened[1] != "/dl/sept.csv" {
		t.Fatalf("opened %v, want the staged export read again", importer.opened)
	}
	staged, _ := model.imports.Staged()
	if row := staged.Rows[1]; row.Target != "expenses:books" || !row.Once {
		t.Errorf("row = %+v, want the choice kept", row)
	}
	if got := lastLine(model); !strings.Contains(got, "1/2") {
		t.Errorf("status bar = %q, want the cursor kept", got)
	}
}

func TestRefreshWithoutAStagedImportReadsNothing(t *testing.T) {
	importer := listing()
	model := startedWithImporter(t, &importer.fakeImporter)

	_, cmd := press(model, "r")
	deliver(model, cmd)

	if len(importer.opened) != 0 {
		t.Errorf("opened %v", importer.opened)
	}
}

func TestAStagedImportOfUnknownPathIsNotReadAgain(t *testing.T) {
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: writable()}
	model := stagedModel(t, importer)

	model, cmd := press(model, "r")
	deliver(model, cmd)

	if len(importer.opened) != 1 {
		t.Errorf("opened %v, want the one open", importer.opened)
	}
}

func TestAFailedRereadKeepsTheStagedImportAndFlagsTheError(t *testing.T) {
	importer := listing()
	importer.err = errors.New("gone")
	model := stagedModel(t, importer)

	model, cmd := press(model, "r")
	model = deliver(model, cmd)

	staged, ok := model.imports.Staged()
	if !ok || staged.Rows[1].Target != "expenses:books" || model.failure == nil {
		t.Errorf("staged %v, error %v; want the import kept and the error flagged", ok, model.failure)
	}
}

func TestAnExportReadAgainAfterTheImportWasWrittenIsDropped(t *testing.T) {
	importer := listing()
	importer.reread = reread()
	model := stagedModel(t, importer)
	earlier, _ := model.imports.Staged()
	model.imports = model.imports.Cleared()

	updated, _ := model.Update(openedMsg{staged: reread(), earlier: &earlier})

	if _, ok := updated.(Model).imports.Staged(); ok {
		t.Error("a stale re-read brought the import back")
	}
}

func quitCommand(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("command returned %T, want a quit", cmd())
	}
}

func askedToDiscard(t *testing.T, model Model) {
	t.Helper()
	if got := lastLine(model); !strings.Contains(got, "discard the staged import? (y/n)") {
		t.Errorf("status bar = %q, want the discard question", got)
	}
}

func TestQAsksBeforeDiscardingAStagedImport(t *testing.T) {
	model, cmd := press(stagedModel(t, listing()), "q")

	if cmd != nil {
		t.Error("q quit without asking")
	}
	askedToDiscard(t, model)
}

func TestNAndEscKeepTheStagedImport(t *testing.T) {
	asked, _ := press(stagedModel(t, listing()), "q")
	for name, answer := range map[string]func(Model) Model{
		"n":   func(m Model) Model { m, _ = press(m, "n"); return m },
		"esc": pressEsc,
	} {
		kept := answer(asked)

		if _, staged := kept.imports.Staged(); !staged || kept.prompt != nil {
			t.Errorf("%s: staged %v, prompt %v", name, staged, kept.prompt)
		}
	}
}

func TestYAfterQQuitsWithoutWriting(t *testing.T) {
	importer := listing()
	asked, _ := press(stagedModel(t, importer), "q")

	_, cmd := press(asked, "y")

	quitCommand(t, cmd)
}

func TestQOnAnotherViewStillAsksWhileWorkIsStaged(t *testing.T) {
	model, _ := press(stagedModel(t, listing()), "1")

	model, cmd := press(model, "q")

	if cmd != nil {
		t.Error("q quit without asking")
	}
	askedToDiscard(t, model)
}

func TestCtrlCAsksLikeQ(t *testing.T) {
	updated, cmd := stagedModel(t, listing()).Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if cmd != nil {
		t.Error("ctrl+c quit without asking")
	}
	askedToDiscard(t, updated.(Model))
}

func TestQQuitsAtOnceWithNothingStagedOrOnceWritten(t *testing.T) {
	_, cmd := press(startedWithImporter(t, &fakeImporter{}), "q")
	quitCommand(t, cmd)

	rig := stagedForWrite(t, writable())
	written, _ := press(rig.model, "w")
	written, cmd = press(written, "y")
	updated, _ := written.Update(cmd())
	_, cmd = press(updated.(Model), "q")
	quitCommand(t, cmd)
}

func TestQQuitsAtOnceWhenTheTerminalIsTooSmallToAnswer(t *testing.T) {
	model := stagedModel(t, listing())
	small, _ := model.Update(tea.WindowSizeMsg{Width: 50, Height: 10})

	_, cmd := press(small.(Model), "q")

	quitCommand(t, cmd)
}

func TestQAgainWhileTheDiscardQuestionIsOpenQuits(t *testing.T) {
	asked, _ := press(stagedModel(t, listing()), "q")

	_, cmd := press(asked, "q")

	quitCommand(t, cmd)
}
