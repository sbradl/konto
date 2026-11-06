package imports

import (
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
)

func freshAccount() csvimport.Staged {
	s := categorisable()
	s.NewAccount, s.Account = true, domain.Account{}
	s.IBAN = "DE27100777770209299700"

	return s
}

func naming() Model { return New("/dl").Resize(180, 50).WithStaged(freshAccount()) }

func TestWKeyAsksTheRootToWrite(t *testing.T) {
	_, action := opened().Key("w")

	if action != (Write{}) {
		t.Errorf("action = %+v, want only Write", action)
	}
}

func TestWKeyWithNothingStagedOrWhilePickingAsksForNothing(t *testing.T) {
	empty, action := New("/dl").Resize(180, 40).Key("w")
	if action != nil {
		t.Error("w with nothing staged asks to write")
	}

	picking, _ := empty.Key("n")
	if _, action := picking.Key("w"); action != nil {
		t.Error("w in the picker asks to write")
	}
}

func TestANewIBANOpensTheAccountNameDialogWithASuggestion(t *testing.T) {
	m := naming()

	view := screen(m)
	for _, want := range []string{"─ create account ", "assets:account-9700", "DE27100777770209299700"} {
		if !strings.Contains(view, want) {
			t.Errorf("dialog lacks %q:\n%s", want, view)
		}
	}

	if !m.Capturing() {
		t.Error("the dialog does not capture the keyboard")
	}
}

func TestEnterAcceptsTheSuggestedNameAndStagesTheAccount(t *testing.T) {
	m := keys(naming(), "enter")

	if m.Capturing() {
		t.Error("the dialog stays open")
	}

	staged, _ := m.Staged()
	if staged.Named != "assets:account-9700" {
		t.Errorf("named = %q", staged.Named)
	}

	view := screen(m)
	if !strings.Contains(view, "assets:account-9700 (new)") || !strings.Contains(view, "account assets:account-9700") {
		t.Errorf("the info block does not show the new account:\n%s", view)
	}
}

func TestTypingReplacesTheNameAndARefusalShowsUntilTheNextKey(t *testing.T) {
	m := keys(naming(), "ctrl+u")
	m = keys(typed(m, "assets:checking"), "enter")

	if view := screen(m); !strings.Contains(view, "refused: the name checking is already used by assets:checking") {
		t.Errorf("no refusal:\n%s", view)
	}

	m = keys(m, "backspace")
	if view := screen(m); strings.Contains(view, "refused:") {
		t.Errorf("the refusal stays after typing:\n%s", view)
	}
	m = keys(m, "ctrl+u")

	m = keys(typed(m, "assets:holiday"), "enter")
	if staged, _ := m.Staged(); m.Capturing() || staged.Named != "assets:holiday" {
		t.Errorf("a fresh name is not accepted: named %q", staged.Named)
	}
}

func TestAnEmptyNameIsRefused(t *testing.T) {
	m := keys(naming(), "ctrl+u", "enter")

	if view := screen(m); !strings.Contains(view, "refused: the account needs a name") || !m.Capturing() {
		t.Errorf("an empty name goes through:\n%s", view)
	}
}

func TestEscAbandonsAccountCreation(t *testing.T) {
	m := keys(naming(), "esc")

	if _, staged := m.Staged(); staged || m.Capturing() {
		t.Error("Esc leaves the import staged")
	}

	if view := screen(m); !strings.Contains(view, "press n for a new file") {
		t.Errorf("not the empty Import:\n%s", view)
	}
}

func TestClearedLeavesTheEmptyImport(t *testing.T) {
	m := opened().Cleared()

	if _, staged := m.Staged(); staged {
		t.Error("an import is still staged")
	}
}

func TestAnExistingAccountOpensNoDialog(t *testing.T) {
	if opened().Capturing() {
		t.Error("a dialog opened for an existing account")
	}
}

func TestWithRefreshedKeepsTheCursorAndClosesADialogOverARow(t *testing.T) {
	m := onRow(categorising(), 1)
	if !m.Capturing() {
		t.Fatal("no dialog is open")
	}
	fresh := categorisable()

	refreshed := m.WithRefreshed(fresh)

	if refreshed.Capturing() {
		t.Error("the dialog over a row stays open")
	}

	if row, total := refreshed.Position(); row != 2 || total != 3 {
		t.Errorf("position = %d/%d, want the cursor where it was", row, total)
	}
}

func TestWithRefreshedClampsTheCursorToTheNewRows(t *testing.T) {
	m := keys(categorising(), "down", "down")
	fresh := categorisable()
	fresh.Rows = fresh.Rows[:1]

	if row, total := m.WithRefreshed(fresh).Position(); row != 1 || total != 1 {
		t.Errorf("position = %d/%d", row, total)
	}
}

func TestWithRefreshedKeepsTheAccountNameDialog(t *testing.T) {
	if !naming().WithRefreshed(freshAccount()).Capturing() {
		t.Error("the name being typed is lost")
	}
}
