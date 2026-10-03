package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const directive = "account assets:spare  ; konto:source:import, konto:iban:DE12, konto:format:postbank"

func closeEdit() domain.Changeset {
	return domain.Changeset{Edits: []domain.Edit{domain.SetAccountTag{Account: "assets:spare", Key: "konto:closed", Value: "2026-09-30"}}}
}

func reopenEdit() domain.Changeset {
	return domain.Changeset{Edits: []domain.Edit{domain.RemoveAccountTag{Account: "assets:spare", Key: "konto:closed"}}}
}

func TestApplySetsATagAndTouchesOnlyThatLine(t *testing.T) {
	file := journalWith(t, "a\n"+directive+"\nb\n")

	if err := file.Apply(closeEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "a\n" + directive + ", konto:closed:2026-09-30\nb\n"
	if got := readFile(t, file.path); got != want {
		t.Errorf("journal =\n%q\nwant\n%q", got, want)
	}
}

func TestApplyReplacesATagWithTheSameKey(t *testing.T) {
	file := journalWith(t, directive+", konto:closed:2026-01-01\n")

	if err := file.Apply(closeEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := readFile(t, file.path), directive+", konto:closed:2026-09-30\n"; got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

func TestClosingThenReopeningRestoresTheJournalByteForByte(t *testing.T) {
	original := "x\n" + directive + "\ny\n"
	file := journalWith(t, original)

	for _, changeset := range []domain.Changeset{closeEdit(), reopenEdit()} {
		if err := file.Apply(changeset); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := readFile(t, file.path); got != original {
		t.Errorf("journal = %q, want %q", got, original)
	}
}

func TestRemovingTheLastTagLeavesABareDirective(t *testing.T) {
	file := journalWith(t, "account assets:spare  ; konto:closed:2026-09-30\n")

	if err := file.Apply(reopenEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, file.path); got != "account assets:spare\n" {
		t.Errorf("journal = %q", got)
	}
}

func TestApplyEditsTheDirectiveInTheFileThatHoldsIt(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.journal")
	if err := os.WriteFile(mainPath, []byte("include accounts.journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(dir, "accounts.journal")
	if err := os.WriteFile(included, []byte(directive+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := NewFile(mainPath).Apply(closeEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, included); !strings.Contains(got, "konto:closed:2026-09-30") {
		t.Errorf("included file = %q, want the closed tag", got)
	}
	if got := readFile(t, mainPath); got != "include accounts.journal\n" {
		t.Errorf("main journal = %q, want it untouched", got)
	}
}

func TestApplyDeclaresAnUndeclaredAccountInTheMainJournal(t *testing.T) {
	for name, tc := range map[string]struct{ before, after string }{
		"final newline": {"x\n", "x\naccount assets:spare  ; konto:closed:2026-09-30\n"},
		"no newline":    {"x", "x\naccount assets:spare  ; konto:closed:2026-09-30"},
	} {
		file := journalWith(t, tc.before)

		if err := file.Apply(closeEdit()); err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}

		if got := readFile(t, file.path); got != tc.after {
			t.Errorf("%s: journal = %q, want %q", name, got, tc.after)
		}
	}
}

func TestRemovingATagFromAnUndeclaredAccountChangesNothing(t *testing.T) {
	file := journalWith(t, "x\n")

	if err := file.Apply(reopenEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, file.path); got != "x\n" {
		t.Errorf("journal = %q, want it untouched", got)
	}
}

func TestApplyKeepsTheFileMode(t *testing.T) {
	file := journalWith(t, directive+"\n")
	if err := os.Chmod(file.path, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := file.Apply(closeEdit()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(file.path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(file.path)); len(entries) != 1 {
		t.Errorf("directory holds %d files, want the journal alone (no temp file left)", len(entries))
	}
}

func TestApplyReportsAnUnreadableJournal(t *testing.T) {
	if err := NewFile(filepath.Join(t.TempDir(), "missing.journal")).Apply(closeEdit()); err == nil {
		t.Error("expected an error for a missing journal")
	}
}

func TestApplyReportsAFailedWrite(t *testing.T) {
	dir := t.TempDir()
	file := NewFile(filepath.Join(dir, "main.journal"))
	if err := os.WriteFile(file.path, []byte(directive+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := file.Apply(closeEdit()); err == nil {
		t.Skip("running as a user that ignores directory permissions")
	}
	if got := readFile(t, file.path); got != directive+"\n" {
		t.Errorf("journal = %q, want it untouched after the failed write", got)
	}
}

func TestReplaceFileReportsAMissingFile(t *testing.T) {
	if err := replaceFile(filepath.Join(t.TempDir(), "missing"), "x"); err == nil {
		t.Error("expected an error replacing a file that is not there")
	}
}

func TestReplaceFileLeavesNoTempFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil { // a directory cannot be replaced by a file
		t.Fatal(err)
	}

	if err := replaceFile(target, "x"); err == nil {
		t.Error("expected an error renaming a file over a directory")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("directory holds %d entries, want only the target", len(entries))
	}
}

func TestApplyWritesEveryFileItTouches(t *testing.T) {
	dir := t.TempDir()
	main := writeJournal(t, dir, "main.journal", "include more.journal\naccount assets:a  ; konto:source:import\n")
	writeJournal(t, dir, "more.journal", "account assets:b  ; konto:source:import\n")

	err := main.Apply(domain.Changeset{Edits: []domain.Edit{
		domain.SetAccountTag{Account: "assets:a", Key: "konto:closed", Value: "2026-09-30"},
		domain.SetAccountTag{Account: "assets:b", Key: "konto:closed", Value: "2026-09-30"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"main.journal", "more.journal"} {
		if got := readFile(t, filepath.Join(dir, name)); !strings.Contains(got, "konto:closed:2026-09-30") {
			t.Errorf("%s = %q, want the closed tag", name, got)
		}
	}
}
