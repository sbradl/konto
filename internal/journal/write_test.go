package journal

import (
	"errors"
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

// withYear is a ledger whose 2026 file holds text; the main journal holds only a directive.
func withYear(t *testing.T, text string) File {
	t.Helper()
	file := journalWith(t, "decimal-mark ,\n")
	if err := os.WriteFile(file.yearPath(2026), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
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
	file := withYear(t, ordered)

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
	if got := readFile(t, file.yearPath(2026)); got != want {
		t.Errorf("journal =\n%s\nwant\n%s", got, want)
	}
}

func TestAddTransactionsAppendsWhatIsNotBeforeAnyOther(t *testing.T) {
	file := withYear(t, ordered)

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{
		spend(day(29), "D"), spend(day(30), "E"),
	}})

	got := readFile(t, file.yearPath(2026))
	if want := "    assets:checking\n\n2026-09-29 D\n    expenses:x\n    assets:checking\n\n2026-09-30 E\n"; !strings.Contains(got, want) {
		t.Errorf("journal =\n%s\nwant it to contain\n%s", got, want)
	}
	if !strings.HasSuffix(got, "    assets:checking\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("journal ends %q, want one final newline", got[len(got)-20:])
	}
}

func TestAddTransactionsKeepsTheCommentsOfTheTransactionBelow(t *testing.T) {
	file := withYear(t, "; about C\n2026-09-29 C\n    expenses:x  3,00 EUR\n    assets:checking\n")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got := readFile(t, file.yearPath(2026)); !strings.HasPrefix(got, "2026-09-28 B\n") || !strings.Contains(got, "\n; about C\n2026-09-29 C") {
		t.Errorf("journal =\n%s", got)
	}
}

func TestAddTransactionsStartTheYearFileWhenItIsNotThereYet(t *testing.T) {
	file := journalWith(t, "decimal-mark ,\n")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got, want := readFile(t, file.yearPath(2026)), "2026-09-28 B\n    expenses:x\n    assets:checking\n"; got != want {
		t.Errorf("2026.journal = %q, want %q", got, want)
	}
	if got := readFile(t, file.path); got != "decimal-mark ,\n" {
		t.Errorf("main journal = %q, want it untouched", got)
	}
}

func TestAddTransactionsGoToTheYearFileOfTheirDate(t *testing.T) {
	file := journalWith(t, "x\n")
	december := time.Date(2025, 12, 30, 0, 0, 0, 0, time.UTC)

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{
		spend(december, "Late"), spend(day(28), "B"),
	}})

	if got := readFile(t, file.yearPath(2025)); !strings.HasPrefix(got, "2025-12-30 Late\n") || strings.Contains(got, "B") {
		t.Errorf("2025.journal = %q, want only the December transaction", got)
	}
	if got := readFile(t, file.yearPath(2026)); !strings.HasPrefix(got, "2026-09-28 B\n") || strings.Contains(got, "Late") {
		t.Errorf("2026.journal = %q, want only the September transaction", got)
	}
}

func TestAddTransactionsOlderThanTheWindowGoToTheirOwnYearFile(t *testing.T) {
	file := journalWith(t, "x\n")
	old := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(old, "Old")}})

	if got, want := readFile(t, file.yearPath(2024)), "2024-12-31 Old\n    expenses:x\n    assets:checking\n"; got != want {
		t.Errorf("2024.journal = %q, want %q", got, want)
	}
}

func TestAddTransactionsAddToAnOlderYearFileThatIsOnDisk(t *testing.T) {
	file := journalWith(t, "x\n")
	if err := os.WriteFile(file.yearPath(2023), []byte("2023-01-05 Z\n    expenses:x\n    assets:checking\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{
		spend(time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), "Early"),
	}})

	if got := readFile(t, file.yearPath(2023)); !strings.HasPrefix(got, "2023-01-02 Early\n") || !strings.Contains(got, "2023-01-05 Z") {
		t.Errorf("2023.journal = %q, want the new row above the old one", got)
	}
}

func TestAddTransactionsRefuseNextYear(t *testing.T) {
	file := journalWith(t, "x\n")
	next := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	err := file.Apply(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{Transactions: []domain.NewTransaction{spend(next, "Out")}},
	}})

	var refusal domain.RefusalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "2027") || !strings.Contains(refusal.Reason, "2026") {
		t.Errorf("error = %v, want a refusal naming 2027 and this year", err)
	}
	if isFile(file.yearPath(2027)) {
		t.Error("a file for 2027 was written")
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

	got, err := file.Render(domain.Changeset{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "top\nsub 1\n\nbottom\n"; got != want {
		t.Errorf("rendered =\n%q\nwant\n%q", got, want)
	}
}

// The journal as it would read: the main journal, then the window's year files, the new ones included.
func TestRenderAppendsTheYearFilesAndWritesNothing(t *testing.T) {
	file := withYear(t, "2026-09-10 A\n")
	if err := os.WriteFile(file.yearPath(2025), []byte("2025-01-01 Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := file.Render(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(30), "E")}},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "decimal-mark ,\n2025-01-01 Z\n2026-09-10 A\n\n2026-09-30 E\n    expenses:x\n    assets:checking\n"
	if got != want {
		t.Errorf("rendered =\n%q\nwant\n%q", got, want)
	}
	if readFile(t, file.yearPath(2026)) != "2026-09-10 A\n" {
		t.Error("Render wrote to the year file")
	}
}

func TestRenderReportsAnEditThatDoesNotApply(t *testing.T) {
	file := journalWith(t, "x\n")

	_, err := file.Render(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{Transactions: []domain.NewTransaction{spend(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), "Future")}},
	}})

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

func TestAddTransactionsSeeATransactionWhoseLineIsOnlyADate(t *testing.T) {
	file := withYear(t, "2026-09-29\n    expenses:x  3,00 EUR\n    assets:checking\n")

	apply(t, file, domain.AddTransactions{Transactions: []domain.NewTransaction{spend(day(28), "B")}})

	if got := readFile(t, file.yearPath(2026)); !strings.HasPrefix(got, "2026-09-28 B\n") {
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

const rulesAndTransaction = "account a\n\n~ monthly from 2026-10-01  A\n    expenses:x  1,00 EUR\n    assets:checking\n\n; note\n~ monthly from 2026-10-02  B\n    expenses:x  2,00 EUR\n    assets:checking\n\n2026-09-01 T\n    expenses:x  3,00 EUR\n    assets:checking\n"

func TestAddRuleAppendsTheBlockAtTheEndOfTheMainJournal(t *testing.T) {
	file := journalWith(t, rulesAndTransaction)

	apply(t, file, domain.AddRule{Lines: []string{"~ yearly from 2026-11-01  C", "    expenses:x  4,00 EUR", "    assets:checking"}})

	if got := readFile(t, file.path); !strings.HasSuffix(got, "    assets:checking\n\n~ yearly from 2026-11-01  C\n    expenses:x  4,00 EUR\n    assets:checking\n") {
		t.Errorf("journal = %q", got)
	}
}

func TestSetCategoryRuleReplacesABlockWhoseMarkerIsTheFirstLine(t *testing.T) {
	file := journalWith(t, "x\n")
	rules := filepath.Join(filepath.Dir(file.path), "categories.rules")
	if err := os.WriteFile(rules, []byte("comment2 konto-rule:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apply(t, file, domain.SetCategoryRule{ID: "1", Text: "comment2 konto-rule:1\naccount2 b"})

	if got, want := readFile(t, rules), "comment2 konto-rule:1\naccount2 b\n"; got != want {
		t.Errorf("rules = %q, want %q", got, want)
	}
}

func TestReplaceRuleSwapsTheBlockInPlaceAndKeepsTheCommentAbove(t *testing.T) {
	file := journalWith(t, rulesAndTransaction)

	apply(t, file, domain.ReplaceRule{
		Pos:   domain.SourcePos{File: file.path, Line: 8},
		Lines: []string{"~ weekly from 2026-10-09  B2", "    expenses:y  9,00 EUR", "    assets:checking"},
	})

	want := strings.Replace(rulesAndTransaction, "~ monthly from 2026-10-02  B\n    expenses:x  2,00 EUR\n    assets:checking",
		"~ weekly from 2026-10-09  B2\n    expenses:y  9,00 EUR\n    assets:checking", 1)
	if got := readFile(t, file.path); got != want {
		t.Errorf("journal =\n%s\nwant\n%s", got, want)
	}
}

func TestReplaceRuleRefusesALineThatIsNoRuleAnymore(t *testing.T) {
	file := journalWith(t, rulesAndTransaction)

	for _, pos := range []domain.SourcePos{{File: file.path, Line: 1}, {File: "/elsewhere", Line: 3}} {
		err := file.Apply(domain.Changeset{Edits: []domain.Edit{domain.ReplaceRule{Pos: pos}}})

		if err == nil {
			t.Errorf("%+v: expected an error", pos)
		}
	}
}

func TestRenderReportsAnUnreadableYearFile(t *testing.T) {
	file := withYear(t, "2026-09-10 A\n")
	if err := os.Chmod(file.yearPath(2026), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file.yearPath(2026), 0o644) })

	if _, err := file.Render(domain.Changeset{}); err == nil {
		t.Error("expected an error for a year file that cannot be read")
	}
}

// An older year's file is part of the text, so the carry into the next year can be added up from it.
func TestRenderIncludesTheOlderYearFileTheEditsTouch(t *testing.T) {
	file := withYear(t, "2026-09-10 A\n")
	if err := os.WriteFile(file.yearPath(2020), []byte("2020-01-01 Far\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.yearPath(2021), []byte("2021-01-01 Untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := file.Render(domain.Changeset{Edits: []domain.Edit{
		domain.AddTransactions{Transactions: []domain.NewTransaction{spend(time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC), "E")}},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "decimal-mark ,\n2020-01-01 Far\n\n2020-02-01 E\n    expenses:x\n    assets:checking\n2026-09-10 A\n"
	if got != want {
		t.Errorf("rendered =\n%q\nwant\n%q", got, want)
	}
}
