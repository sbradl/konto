package ui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

var regexpPosition = regexp.MustCompile(`\b\d+/\d+\b`)

type fakeImporter struct {
	browsed []string
	opened  []string
	entries []domain.FileEntry
	staged  csvimport.Staged
	err     error
	listErr error
}

func (f *fakeImporter) Browse(dir string) ([]domain.FileEntry, error) {
	f.browsed = append(f.browsed, dir)

	return f.entries, f.listErr
}

func (f *fakeImporter) Open(_ context.Context, path string, _ domain.Ledger) (csvimport.Staged, error) {
	f.opened = append(f.opened, path)

	return f.staged, f.err
}

func pressCode(m Model, code rune) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyPressMsg{Code: code})

	return updated.(Model), cmd
}

func startedWithImporter(t *testing.T, importer *fakeImporter) Model {
	t.Helper()
	model := New(journalPath, Ports{
		Loader: fakeLoader{}, Checker: fakeChecker{}, Creator: &fakeCreator{}, Editor: recordingEditor(nil),
		Importer: importer, DownloadDir: "/home/me/Downloads",
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ = updated.Update(model.load())
	imports, _ := press(updated.(Model), "2")

	return imports
}

func sampleStaged() csvimport.Staged {
	return csvimport.Staged{
		File: "sept.csv", Format: "postbank", Account: domain.Account{Name: "assets:checking"},
		Rows: []csvimport.Row{{Description: "Kiosk"}, {Description: "REWE"}},
	}
}

func TestImportStartsEmptyWithAHintAndNIsInTheHint(t *testing.T) {
	model := startedWithImporter(t, &fakeImporter{})

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "press n for a new file") {
		t.Errorf("empty Import:\n%s", content)
	}

	if got := lastLine(model); !strings.Contains(got, "n new file") || regexpPosition.MatchString(got) {
		t.Errorf("status bar = %q", got)
	}
}

func TestNBrowsesTheDownloadDirectoryAndEnterOpensTheFile(t *testing.T) {
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: sampleStaged()}
	model := startedWithImporter(t, importer)

	model, cmd := press(model, "n")

	model = deliver(model, cmd)
	if len(importer.browsed) != 1 || importer.browsed[0] != "/home/me/Downloads" || !strings.Contains(uitest.Plain(model.View().Content), "sept.csv") {
		t.Fatalf("browsed %v; view:\n%s", importer.browsed, uitest.Plain(model.View().Content))
	}

	model, _ = pressCode(model, tea.KeyDown)
	model, cmd = pressCode(model, tea.KeyEnter)
	model = deliver(model, cmd)

	if len(importer.opened) != 1 || importer.opened[0] != "/home/me/Downloads/sept.csv" {
		t.Errorf("opened %v, want the picked file", importer.opened)
	}

	if got := lastLine(model); !strings.Contains(got, "1/2") {
		t.Errorf("status bar = %q, want the row position of the staged import", got)
	}
}

func TestAnUnusableFileIsAMessageAndNothingIsStaged(t *testing.T) {
	for failure, message := range map[error]string{
		csvimport.ErrNotAnExport: csvimport.ErrNotAnExport.Error(), csvimport.ErrNoIBAN: csvimport.ErrNoIBAN.Error(),
		domain.ErrAccountClosed: "account closed — reopen with o",
	} {
		importer := &fakeImporter{err: failure, entries: []domain.FileEntry{{Name: "x.csv"}}}
		model := startedWithImporter(t, importer)
		model, cmd := press(model, "n")
		model = deliver(model, cmd)
		model, _ = pressCode(model, tea.KeyDown)
		model, cmd = pressCode(model, tea.KeyEnter)
		model = deliver(model, cmd)

		content := uitest.Plain(model.View().Content)
		if !strings.Contains(content, message) || strings.Contains(content, "ROWS") || strings.Contains(lastLine(model), "✗") {
			t.Errorf("%v: the message should show and nothing be staged:\n%s", failure, content)
		}
	}
}

func TestGlobalKeysNeverReachTheView(t *testing.T) {
	importer := &fakeImporter{err: csvimport.ErrNoIBAN, entries: []domain.FileEntry{{Name: "x.csv"}}}
	model := startedWithImporter(t, importer)
	model, cmd := press(model, "n")
	model = deliver(model, cmd)
	model, _ = pressCode(model, tea.KeyDown)
	model, cmd = pressCode(model, tea.KeyEnter)
	model = deliver(model, cmd)

	for _, key := range []string{"?", "!", "1"} {
		model, _ = press(model, key)
		model = pressEsc(model)
		model, _ = press(model, "2")
	}

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, csvimport.ErrNoIBAN.Error()) {
		t.Errorf("a global key reached the Import view and cleared its message:\n%s", content)
	}
}

func TestHledgerFailingOnTheConversionFlagsTheErrorAndFillsTheChecksPanel(t *testing.T) {
	importer := &fakeImporter{err: csvimport.ConvertError{Err: errors.New("hledger: boom\nsecond")}, entries: []domain.FileEntry{{Name: "x.csv"}}}
	model := startedWithImporter(t, importer)
	model, cmd := press(model, "n")
	model = deliver(model, cmd)
	model, _ = pressCode(model, tea.KeyDown)
	model, cmd = pressCode(model, tea.KeyEnter)
	failed := deliver(model, cmd)

	content := uitest.Plain(failed.View().Content)
	if !strings.Contains(content, "unreadable row or file") || !strings.Contains(lastLine(failed), "✗ error") {
		t.Errorf("the failure shows as a message and an indicator:\n%s", content)
	}

	opened, _ := press(failed, "!")
	if !strings.Contains(uitest.Plain(opened.View().Content), "hledger: boom") {
		t.Errorf("the ! panel carries hledger's words")
	}
}

func TestAFailureToListTheFolderIsShownInThePicker(t *testing.T) {
	importer := &fakeImporter{listErr: errors.New("listing /home/me/Downloads: no such file or directory")}
	model, cmd := press(startedWithImporter(t, importer), "n")
	model = deliver(model, cmd)

	if content := uitest.Plain(model.View().Content); !strings.Contains(content, "no such file or directory") {
		t.Errorf("picker:\n%s", content)
	}
}

func TestEscLeavesThePickerAndOtherViewsKeepTheirKeys(t *testing.T) {
	model, cmd := press(startedWithImporter(t, &fakeImporter{}), "n")
	model = deliver(model, cmd)

	closed, _ := pressCode(model, tea.KeyEscape)

	if content := uitest.Plain(closed.View().Content); !strings.Contains(content, "press n for a new file") {
		t.Errorf("Esc returns to the empty Import:\n%s", content)
	}

	dash, _ := press(closed, "1")
	if got, cmd := press(dash, "n"); cmd != nil || strings.Contains(uitest.Plain(got.View().Content), "OPEN A CSV") {
		t.Errorf("n belongs to Import only")
	}
}

func TestImportKeysDoNothingWithoutAnImporterAndStatsStayConsistent(t *testing.T) {
	model, _ := press(started(t, fakeLoader{}), "2")

	after, cmd := press(model, "n")

	if cmd != nil || uitest.Plain(after.View().Content) != uitest.Plain(model.View().Content) {
		t.Errorf("without an importer n has nothing to open")
	}
}

func TestImportFollowsTheTerminalSize(t *testing.T) {
	model, _ := press(startedWithImporter(t, &fakeImporter{staged: sampleStaged()}), "2")

	rows := strings.Split(uitest.Plain(resized(model, 160, 50).View().Content), "\n")

	if len(rows) != 50 || !strings.HasPrefix(rows[len(rows)-2], "╰") {
		t.Errorf("view has %d rows; the panels should end just above the status bar, got %q", len(rows), rows[len(rows)-2])
	}
}

func categorisingModel(t *testing.T) Model {
	t.Helper()
	staged := sampleStaged()
	staged.Known = []domain.Account{{Name: "expenses:books"}}
	importer := &fakeImporter{entries: []domain.FileEntry{{Name: "sept.csv"}}, staged: staged}
	model, cmd := press(startedWithImporter(t, importer), "n")
	updated, _ := model.Update(cmd())
	model = updated.(Model)
	model, _ = pressCode(model, tea.KeyDown) // past the parent folder
	model, cmd = pressCode(model, tea.KeyEnter)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	model, _ = pressCode(model, tea.KeyEnter) // the categorise popup

	return model
}

func TestAnOpenDialogTakesEveryKeyAsTextSoViewKeysAndQuitDoNotLeakThrough(t *testing.T) {
	model := categorisingModel(t)
	if !model.capturing() {
		t.Fatalf("the dialog is not capturing:\n%s", uitest.Plain(model.View().Content))
	}
	for _, key := range []string{"q", "1", "r", "?", "!", " "} {
		var cmd tea.Cmd

		model, cmd = press(model, key)
		if cmd != nil || model.active != importTab || model.popup != popupNone || model.busy() {
			t.Fatalf("%q leaked out of the dialog: active %d popup %d", key, model.active, model.popup)
		}
	}

	if got := uitest.Plain(model.View().Content); !strings.Contains(got, "─ categorise ") {
		t.Errorf("the dialog closed:\n%s", got)
	}
}

func TestCtrlCWhileADialogIsOpenAsksBeforeDiscardingTheStagedImport(t *testing.T) {
	model := categorisingModel(t)

	asked, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if cmd != nil || asked.(Model).prompt == nil || !asked.(Model).prompt.quit {
		t.Errorf("ctrl+c over a dialog should ask, got cmd %v and prompt %v", cmd, asked.(Model).prompt)
	}
}

func TestEscClosesTheDialogAndViewKeysWorkAgain(t *testing.T) {
	model := pressEsc(categorisingModel(t))
	if model.capturing() {
		t.Fatal("Esc left the dialog open")
	}

	if model, _ = press(model, "1"); model.active != 0 {
		t.Errorf("active = %d, want the dashboard", model.active)
	}
}

func TestSpaceIsNamedAsATextCharacter(t *testing.T) {
	if got := keyName(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}); got != " " {
		t.Errorf("keyName(space) = %q", got)
	}

	if got := keyName(tea.KeyPressMsg{Code: tea.KeyEnter}); got != "enter" {
		t.Errorf("keyName(enter) = %q", got)
	}
}
