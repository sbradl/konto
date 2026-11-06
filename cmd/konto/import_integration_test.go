//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/files"
	"github.com/sbradl/konto/internal/hledger"
)

// A rule konto writes for a row without payee, which matches the transaction type, categorises that row
// when hledger reads the export through the bank format.
func TestWrittenRuleForARowWithoutPayeeCategorisesItInHledger(t *testing.T) {
	dir := t.TempDir()
	rule := csvimport.Staged{Rows: []csvimport.Row{{Type: "Auszahlung"}}}.RuleDraftFor(0).Rule("1", "expenses:cash")
	export := "Umsätze\nKonto;Kontonummer;IBAN;Währung\nGiro;0123456789;DE89370400440532013000;EUR\n;\n" +
		"1.9.2026 - 30.9.2026\nLetzter Kontostand;;;;0,00;EUR\nVorgemerkte\n" +
		"Buchungstag;Wert;Umsatzart;Begünstigter / Auftraggeber;Verwendungszweck\n" +
		"29.9.2026;29.9.2026;Auszahlung;;Geldautomat;;;;;;;-1.100,00;;;;;;EUR\nKontostand;29.9.2026;;;0,00;EUR\n"
	writeFile(t, filepath.Join(dir, "categories.rules"), rule.Text()+"\n")
	writeFile(t, filepath.Join(dir, "export.csv"), export)
	service := csvimport.NewService(files.System{}, hledger.New("hledger", nil), filepath.Join(dir, "categories.rules"))
	account := domain.Account{Name: "assets:giro", Source: domain.SourceImport, IBAN: "DE89370400440532013000"}

	staged, err := service.Open(context.Background(), filepath.Join(dir, "export.csv"), domain.Ledger{
		Accounts: []domain.Account{account},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(staged.Rows) != 1 || staged.Rows[0].Target != "expenses:cash" || staged.Rows[0].RuleID != "1" {
		t.Errorf("rows = %+v, want the row categorised by rule 1", staged.Rows)
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
