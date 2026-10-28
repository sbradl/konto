package csvimport

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

var euro = domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}

func eur(mantissa int64) domain.MultiAmount {
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: euro}}
}

func day(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

type fakeFiles struct {
	data    map[string][]byte
	entries map[string][]domain.FileEntry
	err     error
}

func (f fakeFiles) ReadFile(path string) ([]byte, error) { return f.data[path], f.err }

func (f fakeFiles) ReadDir(dir string) ([]domain.FileEntry, error) { return f.entries[dir], f.err }

type fakeConverter struct {
	transactions []domain.Transaction
	err          error
	path, rules  string
}

func (f *fakeConverter) ConvertCSV(_ context.Context, csvPath, rules string) ([]domain.Transaction, error) {
	f.path, f.rules = csvPath, rules
	return f.transactions, f.err
}

const preambleText = "\xef\xbb\xbfUmsätze\nKonto;Kontonummer;IBAN;Währung\nPostbank Giro plus;0123456789;%s;EUR\n\n1.9.2026 - 30.9.2026\n" +
	"Letzter Kontostand;;;;99.999,99;EUR\nVorgemerkte und noch nicht gebuchte Umsätze sind nicht Bestandteil dieser Übersicht.\n" +
	"Buchungstag;Wert;Umsatzart;Begünstigter / Auftraggeber;Verwendungszweck;IBAN;BIC;Kunde;Mandat;Gläubiger;Gebühren;Betrag\n" +
	"30.9.2026;30.9.2026;Kartenzahlung;Kiosk;Zeitung;;;;;;;-2,00\nKontostand;30.9.2026;;;14.261,45;EUR\n"

func export(iban string) []byte { return []byte(strings.Replace(preambleText, "%s", iban, 1)) }

const checking = "assets:checking"

func ledgerWith(accounts ...domain.Account) domain.Ledger { return domain.Ledger{Accounts: accounts} }

func importAccount() domain.Account {
	return domain.Account{Name: checking, Source: domain.SourceImport, IBAN: "DE89370400440532013000"}
}

func service(files fakeFiles, conv *fakeConverter) *Service {
	return NewService(files, conv, "/data/categories.rules")
}

func open(t *testing.T, iban string, ledger domain.Ledger, transactions ...domain.Transaction) (Staged, *fakeConverter, error) {
	t.Helper()
	conv := &fakeConverter{transactions: transactions}
	staged, err := service(fakeFiles{data: map[string][]byte{"/dl/sept.csv": export(iban)}}, conv).Open(context.Background(), "/dl/sept.csv", ledger)
	return staged, conv, err
}

// booking builds a converted transaction seen from checking, categorised as target (empty: not yet).
func converted(date, description, comment string, mantissa int64, target, rule string) domain.Transaction {
	counter := domain.Posting{Account: Uncategorised, Amounts: eur(-mantissa)}
	if target != "" {
		counter = domain.Posting{Account: target, Amounts: eur(-mantissa), Tags: map[string]string{domain.RuleTag: rule}}
	}
	return domain.Transaction{Date: day(date), Description: description, Comment: comment, Postings: []domain.Posting{
		{Account: checking, Amounts: eur(mantissa)}, counter,
	}}
}

func journalTxn(date, description string, mantissa int64) domain.Transaction {
	return domain.Transaction{Date: day(date), Description: description, Postings: []domain.Posting{
		{Account: checking, Amounts: eur(mantissa)}, {Account: "expenses:x", Amounts: eur(-mantissa)},
	}}
}

func TestOpenFindsTheAccountByIBANWhateverTheSpacing(t *testing.T) {
	for _, iban := range []string{"DE89370400440532013000", "DE89 3704 0044 0532 0130 00"} {
		staged, _, err := open(t, iban, ledgerWith(importAccount()))

		if err != nil || staged.Account.Name != checking || staged.NewAccount || staged.File != "sept.csv" || staged.Format != "postbank" {
			t.Errorf("IBAN %q: staged = %+v, err = %v", iban, staged, err)
		}
		if staged.IBAN != "DE89370400440532013000" || staged.Closing != "14.261,45" {
			t.Errorf("IBAN %q: staged = %+v, want the normalised IBAN and the trailer's balance", iban, staged)
		}
	}
}

func TestOpenGivesHledgerTheFormatThenTheAccountThenTheRules(t *testing.T) {
	_, conv, err := open(t, "DE89370400440532013000", ledgerWith(importAccount()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conv.path != "/dl/sept.csv" || !strings.HasPrefix(conv.rules, "skip 7\n") ||
		!strings.HasSuffix(conv.rules, "account1 assets:checking\ninclude /data/categories.rules\n") {
		t.Errorf("converted %q with rules %q", conv.path, conv.rules)
	}
}

func TestOpenOfAnUnknownIBANStagesANewAccount(t *testing.T) {
	staged, conv, err := open(t, "DE27100777770209299700", ledgerWith(importAccount()))

	if err != nil || !staged.NewAccount || staged.IBAN != "DE27100777770209299700" || staged.Account.Name != "" {
		t.Errorf("staged = %+v, err = %v; want a new account", staged, err)
	}
	if !strings.Contains(conv.rules, "account1 "+newAccountName+"\n") {
		t.Errorf("rules = %q, want the stand-in account", conv.rules)
	}
}

func TestOpenRefusesAccountsThatCannotBeImportedInto(t *testing.T) {
	iban := "DE89370400440532013000"
	closed := importAccount()
	closed.Closed = day("2026-03-01")
	manual := domain.Account{Name: "assets:cash", Source: domain.SourceManual, IBAN: iban}
	unclassified := domain.Account{Name: checking, IBAN: iban}
	clearing := domain.Account{Name: "assets:transfers", Source: domain.SourceClearing, IBAN: iban}

	for name, tc := range map[string]struct {
		account domain.Account
		want    string
	}{
		"closed": {closed, "account closed — reopen with o"}, "manual": {manual, "manual"},
		"unclassified": {unclassified, "unclassified"}, "clearing": {clearing, "assets:transfers"},
	} {
		_, conv, err := open(t, iban, ledgerWith(tc.account))

		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to say %q", name, err, tc.want)
		}
		if conv.path != "" {
			t.Errorf("%s: a refused account must not be converted", name)
		}
	}
}

func TestOpenRefusesFilesThatAreNotAPostbankExport(t *testing.T) {
	for name, content := range map[string]string{
		"empty": "", "other csv": "just,some,other,csv\n1,2,3\n", "text": "not a csv at all", "binary": "\x00\x01\x02\xff\xfe",
	} {
		_, err := service(fakeFiles{data: map[string][]byte{"/x.csv": []byte(content)}}, &fakeConverter{}).Open(context.Background(), "/x.csv", domain.Ledger{})

		if !errors.Is(err, ErrNotAnExport) {
			t.Errorf("%s: err = %v, want ErrNotAnExport", name, err)
		}
	}
}

func TestOpenRefusesAnExportWithoutAnIBAN(t *testing.T) {
	for name, content := range map[string]string{
		"no preamble line": "Buchungstag;Wert\n", "empty IBAN": "a\nb\nPostbank;123;;EUR\nBuchungstag;Wert\n",
		"short line": "a\nb\nPostbank\nBuchungstag;Wert\n",
	} {
		_, err := service(fakeFiles{data: map[string][]byte{"/x.csv": []byte(content)}}, &fakeConverter{}).Open(context.Background(), "/x.csv", domain.Ledger{})

		if !errors.Is(err, ErrNoIBAN) {
			t.Errorf("%s: err = %v, want ErrNoIBAN", name, err)
		}
	}
}

func TestOpenToleratesWindowsLineEndings(t *testing.T) {
	crlf := strings.ReplaceAll(string(export("DE89370400440532013000")), "\n", "\r\n")
	conv := &fakeConverter{}

	staged, err := service(fakeFiles{data: map[string][]byte{"/x.csv": []byte(crlf)}}, conv).Open(context.Background(), "/x.csv", ledgerWith(importAccount()))

	if err != nil || staged.IBAN != "DE89370400440532013000" || staged.Closing != "14.261,45" {
		t.Errorf("staged = %+v, err = %v", staged, err)
	}
}

func TestOpenReportsAnUnreadableFileAndHledgersFailure(t *testing.T) {
	_, err := service(fakeFiles{err: errors.New("permission denied")}, &fakeConverter{}).Open(context.Background(), "/x.csv", domain.Ledger{})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("err = %v, want the cause", err)
	}

	conv := &fakeConverter{err: errors.New("hledger: bad row")}
	_, err = service(fakeFiles{data: map[string][]byte{"/x.csv": export("DE89370400440532013000")}}, conv).Open(context.Background(), "/x.csv", ledgerWith(importAccount()))
	var convertErr ConvertError
	if !errors.As(err, &convertErr) || convertErr.Error() != "hledger: bad row" || convertErr.Unwrap() == nil {
		t.Errorf("err = %v, want hledger's failure as a ConvertError", err)
	}
}

func TestRowsAreNewestFirstWithTheirCategoryRuleAndPurpose(t *testing.T) {
	staged, _, err := open(t, "DE89370400440532013000", ledgerWith(importAccount()),
		converted("2026-09-29", "Bäckerei", "Brötchen", -650, "", ""),
		converted("2026-09-30", "REWE Markt", "Einkauf", -4217, "expenses:groceries", "2"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []Row{
		{Date: day("2026-09-30"), Description: "REWE Markt", Purpose: "Einkauf", Amount: eur(-4217), Target: "expenses:groceries", RuleID: "2"},
		{Date: day("2026-09-29"), Description: "Bäckerei", Purpose: "Brötchen", Amount: eur(-650)},
	}
	if !reflect.DeepEqual(staged.Rows, want) {
		t.Errorf("rows = %+v, want %+v", staged.Rows, want)
	}
	if staged.NeedCategory() != 1 || staged.Rows[0].NeedsCategory() || !staged.Rows[1].NeedsCategory() {
		t.Errorf("one row (the bakery) needs a category, got %d", staged.NeedCategory())
	}
}

func sept() domain.Ledger {
	ledger := ledgerWith(importAccount())
	ledger.Transactions = []domain.Transaction{
		journalTxn("2026-09-02", "Vermieter Mueller", -90000), journalTxn("2026-09-25", "Acme GmbH", 300000),
		journalTxn("2026-09-26", "Sparen", -50000),
	}
	return ledger
}

func TestRowsBeforeTheWatermarkAreSkippedAndAfterItNew(t *testing.T) {
	staged, _, _ := open(t, "DE89370400440532013000", sept(),
		converted("2026-09-02", "Vermieter Mueller", "Miete", -90000, "", ""),
		converted("2026-09-25", "Acme GmbH", "Gehalt", 300000, "", ""),
		converted("2026-09-26", "Sparen", "Sparen", -50000, "", ""),
		converted("2026-09-26", "Kiosk", "Zeitung", -200, "", ""),
		converted("2026-09-30", "REWE", "Einkauf", -4217, "", ""),
	)

	if staged.Watermark != day("2026-09-26") {
		t.Errorf("watermark = %v, want 2026-09-26", staged.Watermark)
	}
	skipped, fresh := make([]string, 0, len(staged.Skipped)), make([]string, 0, len(staged.Rows))
	for _, row := range staged.Skipped {
		skipped = append(skipped, row.Description)
	}
	for _, row := range staged.Rows {
		fresh = append(fresh, row.Description)
	}
	if !reflect.DeepEqual(skipped, []string{"Sparen", "Acme GmbH", "Vermieter Mueller"}) || !reflect.DeepEqual(fresh, []string{"REWE", "Kiosk"}) {
		t.Errorf("skipped = %v, new = %v", skipped, fresh)
	}
}

func TestRowsOnTheWatermarkDifferInDescriptionOrAmountAreNew(t *testing.T) {
	staged, _, _ := open(t, "DE89370400440532013000", sept(),
		converted("2026-09-26", "Sparkonto", "x", -50000, "", ""),
		converted("2026-09-26", "Sparen", "x", -25000, "", ""),
		converted("2026-09-26", "Sparen", "x", -50000, "", ""),
	)

	if len(staged.Skipped) != 1 || staged.Skipped[0].Description != "Sparen" || staged.Skipped[0].Amount[0].Mantissa != -50000 || len(staged.Rows) != 2 {
		t.Errorf("skipped = %+v, new = %+v", staged.Skipped, staged.Rows)
	}
}

func TestRowsOnTheWatermarkAreMatchedAsAMultiset(t *testing.T) {
	ledger := sept()
	ledger.Transactions = []domain.Transaction{journalTxn("2026-09-26", "Kaffeehaus", -320)}

	staged, _, _ := open(t, "DE89370400440532013000", ledger,
		converted("2026-09-26", "Kaffeehaus", "Kaffee", -320, "", ""),
		converted("2026-09-26", "Kaffeehaus", "Kaffee", -320, "", ""),
	)

	if len(staged.Skipped) != 1 || len(staged.Rows) != 1 {
		t.Errorf("two identical rows against one in the journal: skipped = %d, new = %d, want 1 and 1", len(staged.Skipped), len(staged.Rows))
	}
}

func TestRowsOnTheWatermarkMatchTheJournalByValueNotByStyle(t *testing.T) {
	ledger := sept()
	ledger.Transactions = []domain.Transaction{journalTxn("2026-09-26", "Vermieter", -123456)}
	row := converted("2026-09-26", "Vermieter", "Miete", -123456, "", "")
	row.Postings[0].Amounts[0].Style = domain.AmountStyle{Spaced: true, DecimalMark: ","}

	staged, _, _ := open(t, "DE89370400440532013000", ledger, row)

	if len(staged.Skipped) != 1 || len(staged.Rows) != 0 {
		t.Errorf("1.234,56 against 1234,56: skipped = %d, new = %d, want the row skipped", len(staged.Skipped), len(staged.Rows))
	}
}

func TestIdenticalRowsWithNothingInTheJournalAreBothNew(t *testing.T) {
	staged, _, _ := open(t, "DE89370400440532013000", ledgerWith(importAccount()),
		converted("2026-09-28", "Kaffeehaus", "Kaffee", -320, "", ""),
		converted("2026-09-28", "Kaffeehaus", "Kaffee", -320, "", ""),
	)

	if len(staged.Rows) != 2 || len(staged.Skipped) != 0 || !staged.Watermark.IsZero() {
		t.Errorf("rows = %d, skipped = %d, watermark = %v; want a fresh account with both new", len(staged.Rows), len(staged.Skipped), staged.Watermark)
	}
}

func TestTheWatermarkIgnoresOtherAccountsTransactions(t *testing.T) {
	ledger := sept()
	ledger.Transactions = append(ledger.Transactions, domain.Transaction{
		Date: day("2026-09-29"), Description: "later", Postings: []domain.Posting{{Account: "assets:savings", Amounts: eur(1)}},
	})

	staged, _, _ := open(t, "DE89370400440532013000", ledger)

	if staged.Watermark != day("2026-09-26") {
		t.Errorf("watermark = %v, want 2026-09-26", staged.Watermark)
	}
}

func TestBrowseListsFoldersThenCsvFilesAndNothingElse(t *testing.T) {
	files := fakeFiles{entries: map[string][]domain.FileEntry{"/dl": {
		{Name: "sept.csv"}, {Name: "notes.txt"}, {Name: "subdir", IsDir: true}, {Name: "report.pdf"}, {Name: "ALL.CSV"}, {Name: "a-dir", IsDir: true},
	}}}

	got, err := service(files, &fakeConverter{}).Browse("/dl")

	want := []domain.FileEntry{{Name: "a-dir", IsDir: true}, {Name: "subdir", IsDir: true}, {Name: "ALL.CSV"}, {Name: "sept.csv"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("browse = %+v, %v; want %+v", got, err, want)
	}
}

func TestBrowseReportsAMissingDirectory(t *testing.T) {
	if _, err := service(fakeFiles{err: errors.New("no such file or directory")}, &fakeConverter{}).Browse("/nope"); err == nil || !strings.Contains(err.Error(), "/nope") {
		t.Errorf("err = %v, want it to name the directory", err)
	}
}

func TestOpenReadsTheIBANFromTheFirstFieldsOfAShortLine(t *testing.T) {
	header := "Buchungstag;Wert\n"
	for name, content := range map[string]string{
		"two fields":    "a\nb\nPostbank;123\n" + header,
		"three fields":  "a\nb\nPostbank;123;DE89 3704 0044 0532 0130 00\n" + header,
		"no third line": header,
	} {
		_, err := service(fakeFiles{data: map[string][]byte{"/x.csv": []byte(content)}}, &fakeConverter{}).Open(context.Background(), "/x.csv", domain.Ledger{})

		if name == "three fields" && err != nil {
			t.Errorf("%s: err = %v, want the IBAN read", name, err)
		}
		if name != "three fields" && !errors.Is(err, ErrNoIBAN) {
			t.Errorf("%s: err = %v, want ErrNoIBAN", name, err)
		}
	}
}

func TestOpenToleratesAShortClosingBalanceLine(t *testing.T) {
	for _, line := range []string{"Kontostand;30.9.2026;;", "Kontostand;30.9.2026;;;", "Kontostand"} {
		content := strings.Replace(string(export("DE89370400440532013000")), "Kontostand;30.9.2026;;;14.261,45;EUR", line, 1)

		_, err := service(fakeFiles{data: map[string][]byte{"/x.csv": []byte(content)}}, &fakeConverter{}).Open(context.Background(), "/x.csv", domain.Ledger{})
		if err != nil {
			t.Errorf("%q: err = %v", line, err)
		}
	}
}

func TestColumnIsEmptyBeyondTheLastField(t *testing.T) {
	fields := []string{"a", "b"}
	if got := column(fields, 1); got != "b" {
		t.Errorf("column 1 = %q", got)
	}
	if got := column(fields, 2); got != "" {
		t.Errorf("column 2 = %q, want empty", got)
	}
}
