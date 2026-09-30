package journal

import (
	"os"
	"path/filepath"
	"testing"
)

// A journal file that is there exists.
func TestExistsForAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.journal")
	if err := os.WriteFile(path, []byte("; hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	exists, err := NewFile(path).Exists()

	if err != nil || !exists {
		t.Errorf("Exists = %v, %v; want true, nil", exists, err)
	}
}

// A journal file that is not there does not exist, and that is not an error.
func TestDoesNotExistForAMissingFile(t *testing.T) {
	exists, err := NewFile(filepath.Join(t.TempDir(), "nope.journal")).Exists()

	if err != nil || exists {
		t.Errorf("Exists = %v, %v; want false, nil", exists, err)
	}
}

// A path that cannot even be inspected is an error, not "missing".
func TestExistsReportsUnreadablePath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := NewFile(filepath.Join(blocker, "main.journal")).Exists()

	if err == nil {
		t.Error("expected an error for a path below a regular file, got nil")
	}
}

const wantSkeleton = `decimal-mark ,
commodity 1.000,00 EUR
account equity:opening-balances
account assets:transfers  ; konto:source:clearing
tag konto
tag konto-rule
`

// Create writes the skeleton journal, parent directories included, plus an
// empty categories.rules beside it.
func TestCreateWritesSkeletonAndEmptyRules(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")

	if err := NewFile(filepath.Join(dir, "main.journal")).Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "main.journal"))
	if err != nil || string(got) != wantSkeleton {
		t.Errorf("journal = %q, %v; want the skeleton", got, err)
	}
	rules, err := os.ReadFile(filepath.Join(dir, "categories.rules"))
	if err != nil || len(rules) != 0 {
		t.Errorf("categories.rules = %q, %v; want an empty file", rules, err)
	}
}

// Create never overwrites a journal that is already there.
func TestCreateRefusesToOverwriteExistingJournal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.journal")
	if err := os.WriteFile(path, []byte("; mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewFile(path).Create(); err == nil {
		t.Error("Create over an existing journal should fail, got nil")
	}

	if got, _ := os.ReadFile(path); string(got) != "; mine\n" {
		t.Errorf("journal was changed to %q", got)
	}
}

// An existing categories.rules beside the new journal is kept.
func TestCreateKeepsExistingRules(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "categories.rules")
	if err := os.WriteFile(rulesPath, []byte("if x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewFile(filepath.Join(dir, "main.journal")).Create(); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if got, _ := os.ReadFile(rulesPath); string(got) != "if x\n" {
		t.Errorf("categories.rules = %q, want it kept", got)
	}
}

// Create reports a directory it cannot make.
func TestCreateFailsWhenDirectoryCannotBeMade(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewFile(filepath.Join(blocker, "sub", "main.journal")).Create(); err == nil {
		t.Error("expected an error for a directory below a regular file, got nil")
	}
}

// Create reports a rules file it cannot write.
func TestCreateFailsWhenRulesCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "categories.rules"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := NewFile(filepath.Join(dir, "main.journal")).Create(); err == nil {
		t.Error("expected an error when categories.rules is a directory, got nil")
	}
}
