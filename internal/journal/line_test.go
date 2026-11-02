package journal

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

// hledger separates a directive's keyword from what follows by a tab as well as by spaces.
func TestDirectivesTakeATabAfterTheKeyword(t *testing.T) {
	dir := t.TempDir()
	writeJournal(t, dir, "sub.journal", "account\tassets:giro\t; konto:source:import\n")
	file := writeJournal(t, dir, "main.journal", "include\tsub.journal\naccounts:other\nincludes sub.journal\n")

	got, err := file.AccountDeclarations()

	if err != nil || len(got) != 1 || got[0].Name != "assets:giro" || got[0].Source != domain.SourceImport {
		t.Errorf("declarations = %+v, %v; want only assets:giro, import-fed", got, err)
	}
}

// Render splices in a file included after a tab.
func TestRenderSplicesAnIncludeAfterATab(t *testing.T) {
	dir := t.TempDir()
	writeJournal(t, dir, "sub.journal", "sub\n")
	file := writeJournal(t, dir, "main.journal", "include\tsub.journal\n")

	if got, err := file.Render(domain.Changeset{}); err != nil || got != "sub\n\n" {
		t.Errorf("rendered = %q, %v; want the included file spliced in", got, err)
	}
}

// A posting's status mark may be followed by a tab, and a tab may end its account.
func TestRulesReferencingReadsPostingsSeparatedByTabs(t *testing.T) {
	file := journalWith(t, "~\tmonthly  Rent\n\t*\texpenses:rent\t900,00 EUR\n\t!\t(assets:giro)\n")

	for account, want := range map[string][]string{"expenses:rent": {"Rent"}, "assets:giro": {"Rent"}} {
		if got, err := file.RulesReferencing(account); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("rules posting to %s = %v, %v; want %v", account, got, err, want)
		}
	}
}

// A year file the main journal includes itself is read once: hledger must not see it twice.
func TestWindowLeavesOutAYearFileTheMainJournalIncludes(t *testing.T) {
	dir := windowDir(t, "2025.journal", "2026.journal")
	main := filepath.Join(dir, "main.journal")
	if err := os.WriteFile(main, []byte("include 2026.journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, want := NewFile(main, septemberOf(2026)).Window(), []string{main, filepath.Join(dir, "2025.journal")}; !slices.Equal(got, want) {
		t.Errorf("Window = %v, want %v", got, want)
	}
}

// The rendered journal holds a year file the main journal includes once, where it is included.
func TestRenderSplicesAYearFileTheMainJournalIncludesOnce(t *testing.T) {
	file := withYear(t, "2026-09-10 A\n")
	if err := os.WriteFile(file.path, []byte("include 2026.journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := file.Render(domain.Changeset{}); err != nil || got != "2026-09-10 A\n\n" {
		t.Errorf("rendered = %q, %v; want the year file once", got, err)
	}
}
