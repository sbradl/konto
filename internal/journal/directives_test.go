package journal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

func journalWith(t *testing.T, text string) File {
	t.Helper()
	return writeJournal(t, t.TempDir(), "main.journal", text)
}

func writeJournal(t *testing.T, dir, name, text string) File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewFile(path)
}

const directivesJournal = `decimal-mark ,
account equity:opening-balances
account assets:transfers  ; konto:source:clearing
account assets:checking  ; konto:source:import, konto:iban:DE89370400440532013000, konto:format:postbank
account assets:old-account  ; konto:source:import, konto:closed:2026-03-01
account assets:cash  ; konto:source:manual
account assets:odd name  ; konto:closed:someday, other:tag
account expenses:rent
  account indented:not-a-directive
2026-01-01 account in a description
    assets:checking  1,00 EUR
`

func TestAccountDeclarationsReadsTheTags(t *testing.T) {
	got, err := journalWith(t, directivesJournal).AccountDeclarations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []domain.Declaration{
		{Name: "equity:opening-balances"},
		{Name: "assets:transfers", Source: domain.SourceClearing},
		{Name: "assets:checking", Source: domain.SourceImport, IBAN: "DE89370400440532013000"},
		{Name: "assets:old-account", Source: domain.SourceImport, Closed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "assets:cash", Source: domain.SourceManual},
		{Name: "assets:odd name"},
		{Name: "expenses:rent"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("declarations =\n%+v\nwant\n%+v", got, want)
	}
}

func TestAccountDeclarationsReportsAnUnreadableFile(t *testing.T) {
	if _, err := NewFile(filepath.Join(t.TempDir(), "missing.journal")).AccountDeclarations(); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestAccountDeclarationsFollowIncludes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.journal", "account assets:a  ; konto:source:manual\ninclude sub/*.journal\ninclude "+filepath.Join(dir, "abs.journal")+"\ninclude main.journal\ninclude nothing-here.journal\n")
	write("abs.journal", "account assets:abs\n")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("sub/one.journal", "account assets:one  ; konto:source:import\n")
	write("sub/two.journal", "account assets:two\n")

	got, err := NewFile(filepath.Join(dir, "main.journal")).AccountDeclarations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make([]string, 0, len(got))
	for _, declaration := range got {
		names = append(names, declaration.Name)
	}
	if want := []string{"assets:a", "assets:one", "assets:two", "assets:abs"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestAccountDeclarationsReportsABadIncludePattern(t *testing.T) {
	if _, err := journalWith(t, "include [\n").AccountDeclarations(); err == nil {
		t.Error("expected an error for a malformed include pattern")
	}
}

func TestAccountDeclarationsReportsAnUnreadableInclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dir.journal"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.journal")
	if err := os.WriteFile(main, []byte("include dir.journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFile(main).AccountDeclarations(); err == nil {
		t.Error("expected an error for an include that cannot be read")
	}
}
