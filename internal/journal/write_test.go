package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

func day(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }

func spend(date time.Time, description string) domain.NewTransaction {
	return domain.NewTransaction{
		Date: date, Description: description,
		Postings: []domain.NewPosting{{Account: "expenses:x"}, {Account: "assets:checking"}},
	}
}

func apply(t *testing.T, file File, edits ...domain.Edit) {
	t.Helper()
	if err := file.Apply(domain.Changeset{Edits: edits}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeclareAddsADirectiveToTheMainJournalOnce(t *testing.T) {
	file := journalWith(t, "x\n")
	edit := domain.Declare{Directive: "account", Name: "expenses:cafe", Comment: "konto:source:import"}

	apply(t, file, edit, edit, domain.Declare{Directive: "payee", Name: "Kaffeehaus"})

	want := "x\naccount expenses:cafe  ; konto:source:import\npayee Kaffeehaus\n"
	if got := readFile(t, file.path); got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

func TestDeclareLeavesWhatAnyFileAlreadyDeclares(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sub.journal"), []byte("payee REWE  ; known\naccount expenses:cafe  ; a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := writeJournal(t, dir, "main.journal", "include sub.journal\npayee REWE Markt\n")

	apply(t, file,
		domain.Declare{Directive: "payee", Name: "REWE"},
		domain.Declare{Directive: "account", Name: "expenses:cafe"},
		domain.Declare{Directive: "payee", Name: "REWE Markt"},
		domain.Declare{Directive: "tag", Name: "REWE"},
	)

	if got, want := readFile(t, file.path), "include sub.journal\npayee REWE Markt\ntag REWE\n"; got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

func TestDeclareIgnoresTheLongerNameAndOtherDirectives(t *testing.T) {
	file := journalWith(t, "payee REWE Markt\n")

	apply(t, file, domain.Declare{Directive: "payee", Name: "REWE"})

	if got, want := readFile(t, file.path), "payee REWE Markt\npayee REWE\n"; got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

const ordered = `2026-09-10 A
    expenses:x  1,00 EUR
    assets:checking

2026-09-29 C
    expenses:x  3,00 EUR
    assets:checking
`

func TestAddTransactionsInsertsAtTheDatePosition(t *testing.T) {
	file := journalWith(t, ordered)

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	want := `2026-09-10 A
    expenses:x  1,00 EUR
    assets:checking

2026-09-28 B
    expenses:x
    assets:checking

2026-09-29 C
    expenses:x  3,00 EUR
    assets:checking
`
	if got := readFile(t, file.path); got != want {
		t.Errorf("journal =\n%s\nwant\n%s", got, want)
	}
}

func TestAddTransactionsAppendsWhatIsNotBeforeAnyOther(t *testing.T) {
	file := journalWith(t, ordered)

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{
		spend(day(29), "D"), spend(day(30), "E"),
	}})

	got := readFile(t, file.path)
	if want := "    assets:checking\n\n2026-09-29 D\n    expenses:x\n    assets:checking\n\n2026-09-30 E\n"; !strings.Contains(got, want) {
		t.Errorf("journal =\n%s\nwant it to contain\n%s", got, want)
	}
	if !strings.HasSuffix(got, "    assets:checking\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("journal ends %q, want one final newline", got[len(got)-20:])
	}
}

func TestAddTransactionsKeepsTheCommentsOfTheTransactionBelow(t *testing.T) {
	file := journalWith(t, "; about C\n2026-09-29 C\n    expenses:x  3,00 EUR\n    assets:checking\n")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got := readFile(t, file.path); !strings.HasPrefix(got, "2026-09-28 B\n") || !strings.Contains(got, "\n; about C\n2026-09-29 C") {
		t.Errorf("journal =\n%s", got)
	}
}

func TestAddTransactionsIntoAnEmptyFileStartsIt(t *testing.T) {
	file := journalWith(t, "")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got, want := readFile(t, file.path), "2026-09-28 B\n    expenses:x\n    assets:checking\n"; got != want {
		t.Errorf("journal = %q, want %q", got, want)
	}
}

func TestAddTransactionsGoIntoTheNamedFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub.journal")
	if err := os.WriteFile(sub, []byte(ordered), 0o644); err != nil {
		t.Fatal(err)
	}
	file := writeJournal(t, dir, "main.journal", "include sub.journal\n")

	apply(t, file, domain.AddTransactions{File: sub, Transactions: []domain.NewTransaction{spend(day(30), "E")}})

	if got := readFile(t, sub); !strings.Contains(got, "2026-09-30 E") {
		t.Errorf("sub.journal = %q, want the transaction", got)
	}
	if got := readFile(t, file.path); got != "include sub.journal\n" {
		t.Errorf("main journal = %q, want it untouched", got)
	}
}

func TestAddTransactionsRefuseAFileOutsideTheJournal(t *testing.T) {
	file := journalWith(t, "x\n")

	err := file.Apply(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{File: "/elsewhere.journal", Transactions: []domain.NewTransaction{spend(day(30), "E")}},
	}})

	if err == nil {
		t.Error("expected an error for a file the journal does not include")
	}
}

func TestSetCategoryRuleAppendsABlockAfterABlankLine(t *testing.T) {
	file := journalWith(t, "x\n")
	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	if err := os.WriteFile(rules, []byte("if %payee A\n    account2 a\n    comment2 konto-rule:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apply(t, file, domain.SetCategoryRule{ID: "2", Text: "if %payee B\n    account2 b\n    comment2 konto-rule:2"})

	want := "if %payee A\n    account2 a\n    comment2 konto-rule:1\n\nif %payee B\n    account2 b\n    comment2 konto-rule:2\n"
	if got := readFile(t, rules); got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}

func TestSetCategoryRuleCreatesTheFileWhenThereIsNone(t *testing.T) {
	file := journalWith(t, "x\n")

	apply(t, file, domain.SetCategoryRule{ID: "1", Text: "if %payee A\n    account2 a\n    comment2 konto-rule:1"})

	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	if got, want := readFile(t, rules), "if %payee A\n    account2 a\n    comment2 konto-rule:1\n"; got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}

func TestSetCategoryRuleReplacesTheBlockWithTheSameId(t *testing.T) {
	file := journalWith(t, "x\n")
	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	before := "if %payee A\n    account2 a\n    comment2 konto-rule:1\n\nif %payee B\n    comment2 konto-rule:2\n    account2 b\n\nif %payee C\n    account2 c\n    comment2 konto-rule:3\n"
	if err := os.WriteFile(rules, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	apply(t, file, domain.SetCategoryRule{ID: "2", Text: "if %payee B\nif %payee B2\n    account2 b\n    comment2 konto-rule:2"})

	want := "if %payee A\n    account2 a\n    comment2 konto-rule:1\n\nif %payee B\nif %payee B2\n    account2 b\n    comment2 konto-rule:2\n\nif %payee C\n    account2 c\n    comment2 konto-rule:3\n"
	if got := readFile(t, rules); got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}

func TestSetCategoryRuleReportsAnUnreadableRulesFile(t *testing.T) {
	file := journalWith(t, "x\n")
	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	if err := os.Mkdir(rules, 0o755); err != nil { // a directory cannot be read as a file
		t.Fatal(err)
	}

	err := file.Apply(domain.Changeset{Edits: []domain.Edit{domain.SetCategoryRule{ID: "1", Text: "if %payee A"}}})

	if err == nil {
		t.Error("expected an error for an unreadable rules file")
	}
}

func TestRenderSplicesIncludedFilesInAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub.journal")
	if err := os.WriteFile(sub, []byte("sub 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := writeJournal(t, dir, "main.journal", "top\ninclude sub.journal\ninclude none-*.journal\nbottom\n")

	got, err := file.Render(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{File: sub, Transactions: []domain.NewTransaction{spend(day(30), "E")}},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "top\nsub 1\n\n2026-09-30 E\n    expenses:x\n    assets:checking\n\nbottom\n"
	if got != want {
		t.Errorf("rendered =\n%q\nwant\n%q", got, want)
	}
	if readFile(t, sub) != "sub 1\n" {
		t.Error("Render wrote to the journal")
	}
}

func TestRenderReportsAnEditThatDoesNotApply(t *testing.T) {
	file := journalWith(t, "x\n")

	_, err := file.Render(domain.Changeset{Edits: []domain.Edit{domain.AddTransactions{File: "/nope"}}})

	if err == nil {
		t.Error("expected an error")
	}
}

func TestRenderReportsAnUnreadableJournal(t *testing.T) {
	if _, err := NewFile(filepath.Join(t.TempDir(), "missing")).Render(domain.Changeset{}); err == nil {
		t.Error("expected an error")
	}
}

func TestRenderDoesNotSpliceAFileTwice(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sub.journal"), []byte("sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := writeJournal(t, dir, "main.journal", "include sub.journal\ninclude sub.journal\n")

	got, err := file.Render(domain.Changeset{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Count(got, "sub\n") != 1 {
		t.Errorf("rendered = %q, want the included file once", got)
	}
}

func TestAddTransactionsFindTheFileByAnyOfItsSpellings(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub.journal")
	if err := os.WriteFile(sub, []byte(ordered), 0o644); err != nil {
		t.Fatal(err)
	}
	file := writeJournal(t, dir, "main.journal", "include sub.journal\n")

	apply(t, file, domain.AddTransactions{
		File: filepath.Join(dir, ".", "other", "..", "sub.journal"), Transactions: []domain.NewTransaction{spend(day(30), "E")},
	})

	if got := readFile(t, sub); !strings.Contains(got, "2026-09-30 E") {
		t.Errorf("sub.journal = %q, want the transaction", got)
	}
}

func TestAddTransactionsNamingTheMainJournalWriteThere(t *testing.T) {
	file := journalWith(t, ordered)

	apply(t, file, domain.AddTransactions{File: file.path, Transactions: []domain.NewTransaction{spend(day(30), "E")}})

	if got := readFile(t, file.path); !strings.Contains(got, "2026-09-30 E") {
		t.Errorf("journal = %q, want the transaction", got)
	}
}

func TestAddTransactionsSeeATransactionWhoseLineIsOnlyADate(t *testing.T) {
	file := journalWith(t, "2026-09-29\n    expenses:x  3,00 EUR\n    assets:checking\n")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got := readFile(t, file.path); !strings.HasPrefix(got, "2026-09-28 B\n") {
		t.Errorf("journal = %q, want B above the bare-date transaction", got)
	}
}

func TestSetCategoryRuleReplacesABlockThatIsTheWholeFile(t *testing.T) {
	for name, before := range map[string]string{
		"with a final newline": "if %payee A\n    account2 a\n    comment2 konto-rule:1\n",
		"without":              "if %payee A\n    account2 a\n    comment2 konto-rule:1",
	} {
		file := journalWith(t, "x\n")
		rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
		if err := os.WriteFile(rules, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}

		apply(t, file, domain.SetCategoryRule{ID: "1", Text: "if %payee B\n    account2 b\n    comment2 konto-rule:1"})

		want := "if %payee B\n    account2 b\n    comment2 konto-rule:1" + before[len(strings.TrimRight(before, "\n")):]
		if got := readFile(t, rules); got != want {
			t.Errorf("%s: rules = %q, want %q", name, got, want)
		}
	}
}

func TestSetCategoryRuleReplacesTheFirstAndTheLastBlockOfSeveral(t *testing.T) {
	file := journalWith(t, "x\n")
	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	before := "if %payee A\n    comment2 konto-rule:1\n\nif %payee B\n    comment2 konto-rule:2"
	if err := os.WriteFile(rules, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	apply(t, file,
		domain.SetCategoryRule{ID: "1", Text: "if %payee A2\n    comment2 konto-rule:1"},
		domain.SetCategoryRule{ID: "2", Text: "if %payee B2\n    comment2 konto-rule:2"})

	want := "if %payee A2\n    comment2 konto-rule:1\n\nif %payee B2\n    comment2 konto-rule:2"
	if got := readFile(t, rules); got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}
