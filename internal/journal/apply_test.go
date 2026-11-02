package journal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

func TestApplyRefusesAnEditItDoesNotKnowAndWritesNothing(t *testing.T) {
	file := journalWith(t, directive+"\n")
	changeset := closeEdit()
	changeset.Edits = append(changeset.Edits, struct{ domain.Edit }{})

	if err := file.Apply(changeset); err == nil || !strings.Contains(err.Error(), "unsupported edit") {
		t.Errorf("err = %v, want the unsupported edit", err)
	}
	if got := readFile(t, file.path); got != directive+"\n" {
		t.Errorf("journal = %q, want it untouched", got)
	}
}

// editKinds is a zero value of every domain.Edit implementation; TestApplyHandlesEveryKindOfEdit keeps it
// complete against the domain's source.
var editKinds = []domain.Edit{
	domain.SetAccountTag{},
	domain.RemoveAccountTag{},
	domain.Declare{},
	domain.AddTransactions{},
	domain.SetCategoryRule{},
	domain.SetCarry{},
	domain.DeleteRule{},
	domain.AddRule{},
	domain.ReplaceRule{},
}

// The domain's edits are a closed set (the unexported isEdit marker), so apply can switch over all of them:
// a new kind of edit fails here until apply has a case for it.
func TestApplyHandlesEveryKindOfEdit(t *testing.T) {
	kinds := make([]string, 0, len(editKinds))
	for _, edit := range editKinds {
		kinds = append(kinds, reflect.TypeOf(edit).Name())
		if err := journalWith(t, "x\n").Apply(domain.Changeset{Edits: []domain.Edit{edit}}); err != nil &&
			strings.Contains(err.Error(), "unsupported edit") {
			t.Errorf("apply does not handle %T", edit)
		}
	}
	if marked := editImplementations(t); !slices.Equal(marked, slices.Sorted(slices.Values(kinds))) {
		t.Errorf("domain.Edit implementations = %v, editKinds = %v; add the new kind to both", marked, kinds)
	}
}

// editImplementations are the types the domain package gives the isEdit marker, sorted by name.
func editImplementations(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "domain", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "isEdit" && fn.Recv != nil {
				names = append(names, fmt.Sprint(fn.Recv.List[0].Type))
			}
		}
	}
	slices.Sort(names)
	return names
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

	if err := NewFile(mainPath, septemberOf(2026)).Apply(closeEdit()); err != nil {
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
	if err := NewFile(filepath.Join(t.TempDir(), "missing.journal"), septemberOf(2026)).Apply(closeEdit()); err == nil {
		t.Error("expected an error for a missing journal")
	}
}

func TestApplyReportsAFailedWrite(t *testing.T) {
	dir := t.TempDir()
	file := NewFile(filepath.Join(dir, "main.journal"), septemberOf(2026))
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

func TestStageReportsAPathThatCannotBeLookedAt(t *testing.T) {
	notADirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADirectory, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := stage(&source{path: filepath.Join(notADirectory, "x")}); err == nil {
		t.Error("expected an error for a path under a file")
	}
}

func TestCommitAllLeavesNoTempFileWhenARenameFails(t *testing.T) {
	dir := t.TempDir()
	first, target := filepath.Join(dir, "first"), filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil { // a directory cannot be replaced by a file
		t.Fatal(err)
	}

	err := commitAll([]*source{{path: first, lines: []string{"x"}}, {path: target, lines: []string{"x"}}})

	if err == nil {
		t.Error("expected an error renaming a file over a directory")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("directory holds %d entries, want the first file and the target only", len(entries))
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

const ruleJournal = `; keep me
~ monthly from 2026-10-20  Gym
    expenses:rent  25,00 EUR  ; posting note
    ; own line
    assets:checking

~ monthly from 2026-11-01  Rent
    expenses:rent  900,00 EUR
    assets:checking
account assets:checking
`

func deleteAt(file File, line int) error {
	return file.Apply(domain.PeriodicRule{Pos: domain.SourcePos{File: file.path, Line: line}}.Delete())
}

func TestApplyDeletesTheWholeBlockAndKeepsCommentsAboveIt(t *testing.T) {
	file := journalWith(t, ruleJournal)

	if err := deleteAt(file, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "; keep me\n\n~ monthly from 2026-11-01  Rent\n    expenses:rent  900,00 EUR\n    assets:checking\naccount assets:checking\n"
	if got := readFile(t, file.path); got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

func TestApplyDeletesTheLastBlockOfAFileWithoutLeavingBlankLines(t *testing.T) {
	file := journalWith(t, "a\n\n~ monthly  X\n    p  1,00 EUR\n    q\n")

	if err := deleteAt(file, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, file.path); got != "a\n" {
		t.Errorf("journal = %q, want a\\n", got)
	}
}

func TestApplyDeletesBetweenBlankLinesLeavingOne(t *testing.T) {
	file := journalWith(t, "a\n\n~ monthly  X\n    p  1,00 EUR\n    q\n\nb\n")

	if err := deleteAt(file, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, file.path); got != "a\n\nb\n" {
		t.Errorf("journal = %q", got)
	}
}

func TestApplyDeletesTheFirstLineOfAFile(t *testing.T) {
	file := journalWith(t, "~ monthly  X\n    p  1,00 EUR\n    q\nb\n")

	if err := deleteAt(file, 1); err != nil || readFile(t, file.path) != "b\n" {
		t.Errorf("journal = %q, %v", readFile(t, file.path), err)
	}
}

func TestApplyDeletesARuleInAnIncludedFileAndLeavesTheMainOneAlone(t *testing.T) {
	dir := t.TempDir()
	main := writeJournal(t, dir, "main.journal", "include rules.journal\n")
	rules := filepath.Join(dir, "rules.journal")
	writeJournal(t, dir, "rules.journal", "~ monthly  X\n    p  1,00 EUR\n    q\n")

	err := main.Apply(domain.PeriodicRule{Pos: domain.SourcePos{File: rules, Line: 1}}.Delete())

	if err != nil || readFile(t, rules) != "" || readFile(t, main.path) != "include rules.journal\n" {
		t.Errorf("rules = %q, main = %q, %v", readFile(t, rules), readFile(t, main.path), err)
	}
}

func TestApplyRefusesToDeleteWhatIsNoLongerARule(t *testing.T) {
	file := journalWith(t, ruleJournal)

	for name, pos := range map[string]domain.SourcePos{
		"a line that moved":   {File: file.path, Line: 4},
		"before the file":     {File: file.path, Line: 0},
		"past the file":       {File: file.path, Line: 99},
		"a file not included": {File: "/elsewhere.journal", Line: 1},
	} {
		if err := file.Apply(domain.PeriodicRule{Pos: pos}.Delete()); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if got := readFile(t, file.path); got != ruleJournal {
		t.Errorf("a refused delete must not touch the journal, got %q", got)
	}
}

func TestApplyDeletesABlockThatEndsTheFileWithoutANewline(t *testing.T) {
	file := journalWith(t, "a\n~ monthly  X\n    p  1,00 EUR\n    q")

	if err := deleteAt(file, 2); err != nil || readFile(t, file.path) != "a" {
		t.Errorf("journal = %q, %v", readFile(t, file.path), err)
	}
}

func TestApplyDeletesAfterABlankLineAtTheEndOfAFileWithoutANewline(t *testing.T) {
	file := journalWith(t, "a\n\n~ monthly  X\n    p  1,00 EUR\n    q")

	if err := deleteAt(file, 3); err != nil || readFile(t, file.path) != "a\n" {
		t.Errorf("journal = %q, %v", readFile(t, file.path), err)
	}
}

func TestApplyRefusesTheLineJustPastTheEnd(t *testing.T) {
	file := journalWith(t, "a\n~ monthly  X\n    p\n")

	if err := deleteAt(file, 5); err == nil { // lines: a, ~, p, "" — index 4 is past them
		t.Error("expected an error")
	}
}

func TestApplyRefusesAReadOnlyJournalAndLeavesItAlone(t *testing.T) {
	file := journalWith(t, directive+"\n")
	if err := os.Chmod(file.path, 0o444); err != nil {
		t.Fatal(err)
	}

	err := file.Apply(closeEdit())

	if err == nil || !strings.Contains(err.Error(), file.path) {
		if err == nil {
			t.Skip("running as a user that ignores file permissions")
		}
		t.Errorf("error = %v, want it to name the file", err)
	}
	if got := readFile(t, file.path); got != directive+"\n" {
		t.Errorf("journal = %q, want it untouched", got)
	}
}
