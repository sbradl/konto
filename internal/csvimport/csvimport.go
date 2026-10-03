// Package csvimport is the use case behind the Import view: it reads a bank export against the
// journal and tells what is new, what is already there and what still needs a category. hledger
// converts the CSV; konto owns the rest (spec §4.9).
package csvimport

import (
	"bytes"
	"context"
	_ "embed" // the bank format ships inside the binary
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

//go:embed postbank.rules
var postbankRules string

// Uncategorised is the account the bank format gives a row no rule has claimed.
const Uncategorised = "konto:uncategorised"

// FormatPostbank names the one bank format konto understands.
const FormatPostbank = "postbank"

// RuleTag is the posting tag that carries the id of the rule that categorised a row.
const RuleTag = "konto-rule"

// The ways an export can be unusable; both are the file's fault, not hledger's.
var (
	ErrNotAnExport = errors.New("the file is not a recognised Postbank export")
	ErrNoIBAN      = errors.New("the account could not be detected: the export names no IBAN")
)

// ConvertError is hledger failing to read the export; its text is hledger's own.
type ConvertError struct{ Err error }

func (e ConvertError) Error() string { return e.Err.Error() }
func (e ConvertError) Unwrap() error { return e.Err }

type fileSystem interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(dir string) ([]domain.FileEntry, error)
}

type converter interface {
	ConvertCSV(ctx context.Context, csvPath, rules string) ([]domain.Transaction, error)
}

// Service reads exports.
type Service struct {
	files     fileSystem
	converter converter
	rulesPath string // categories.rules, beside the journal
}

// NewService returns a Service that reads files through files, converts through converter and applies
// the categorisation rules in rulesPath.
func NewService(files fileSystem, converter converter, rulesPath string) *Service {
	return &Service{files: files, converter: converter, rulesPath: rulesPath}
}

// Row is one booking of the export.
type Row struct {
	Date        time.Time
	Description string // the payee, else the transaction type
	Purpose     string // becomes the transaction comment
	Amount      domain.MultiAmount
	Type        string // the export's transaction type, e.g. Kartenzahlung
	Payee       string // the export's payee; empty for bookings the bank makes itself
	Target      string // the category or transfer account a rule names; empty while it needs one
	RuleID      string // the rule that named it; empty for a row categorised once
	Once        bool   // the category was chosen for this row alone, and no rule may take it away
}

// NeedsCategory reports a row no rule has claimed yet.
func (r Row) NeedsCategory() bool { return r.Target == "" }

// Staged is an export read against the journal, held in memory until it is written.
type Staged struct {
	Path        string // where the export was read from, to read it again when the journal changes
	File        string // the export's file name
	IBAN        string
	Format      string
	Account     domain.Account // the account the IBAN belongs to; zero for a new one
	NewAccount  bool           // the IBAN is unknown: the account has yet to be created
	Named       string         // the name given to the new account; empty until it has one
	Watermark   time.Time      // the account's latest transaction; zero for a fresh account
	Rows        []Row          // new rows, newest first
	Skipped     []Row          // rows the journal already has, newest first
	Closing     string         // the bank's closing balance as printed, e.g. "14.261,45"
	Rules       []Rule         // the categorisation rules, those staged by this import included
	Known       []domain.Account
	NewAccounts []string           // categories this import declares
	Zero        domain.MultiAmount // a zero in the ledger's currency and style
}

// NeedCategory counts the new rows no rule has claimed.
func (s Staged) NeedCategory() int {
	return len(slices.DeleteFunc(slices.Clone(s.Rows), func(r Row) bool { return !r.NeedsCategory() }))
}

// newAccountName stands in for the account's name until the import creates it.
const newAccountName = "assets:new-account"

// Open reads the export at path against the journal snapshot. A refused account is the error.
func (s *Service) Open(ctx context.Context, path string, ledger domain.Ledger) (Staged, error) {
	data, err := s.files.ReadFile(path)
	if err != nil {
		return Staged{}, fmt.Errorf("reading %s: %w", path, err)
	}
	preamble, err := readPreamble(data)
	if err != nil {
		return Staged{}, err
	}
	staged := Staged{
		Path: path, File: filepath.Base(path), IBAN: preamble.iban, Format: FormatPostbank, Closing: preamble.closing,
		Known: ledger.Accounts, Zero: ledger.Zero(),
	}
	staged.Rules = s.readRules()
	staged.Account, staged.NewAccount = accountOf(ledger, preamble.iban)
	if !staged.NewAccount {
		if err := staged.Account.ImportRefusal(); err != nil {
			return Staged{}, err //nolint:wrapcheck // the refusal is the answer, in the words the user reads
		}
	}
	name := staged.Account.Name
	if staged.NewAccount {
		name = newAccountName
	}
	transactions, err := s.converter.ConvertCSV(ctx, path, s.rules(name))
	if err != nil {
		return Staged{}, ConvertError{Err: err}
	}
	rows := withRawFields(rowsOf(transactions, name), preamble.records)
	staged.Rows, staged.Skipped, staged.Watermark = classify(rows, journalBookings(ledger, name))
	return staged, nil
}

// readRules are the rules of categories.rules; a file that cannot be read has none, and hledger, which
// includes it, says what is wrong with it.
func (s *Service) readRules() []Rule {
	data, err := s.files.ReadFile(s.rulesPath)
	if err != nil {
		return nil
	}
	return ParseRules(string(data))
}

// rules is the bank format, then the account the export belongs to, then the categorisation rules.
func (s *Service) rules(account string) string {
	return postbankRules + "account1 " + account + "\ninclude " + s.rulesPath + "\n"
}

// accountOf is the account the IBAN belongs to; false when no account has it.
func accountOf(ledger domain.Ledger, iban string) (account domain.Account, isNew bool) {
	for _, candidate := range ledger.Accounts {
		if candidate.IBAN != "" && domain.NormalizeIBAN(candidate.IBAN) == iban {
			return candidate, false
		}
	}
	return domain.Account{}, true
}

// preamble is what the export says about itself.
type preamble struct {
	iban, closing string
	records       []record
}

const (
	ibanLine      = 2 // Postbank prints the account on the third line
	ibanColumn    = 2
	balanceColumn = 4
)

// readPreamble recognises a Postbank export and reads its IBAN and the trailer's closing balance.
func readPreamble(data []byte) (preamble, error) {
	text := strings.TrimPrefix(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\xef\xbb\xbf")
	lines := strings.Split(text, "\n")
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "Buchungstag;") }) {
		return preamble{}, ErrNotAnExport
	}
	var found preamble
	if ibanLine < len(lines) {
		if fields := strings.Split(lines[ibanLine], ";"); len(fields) > ibanColumn {
			found.iban = domain.NormalizeIBAN(fields[ibanColumn])
		}
	}
	if found.iban == "" {
		return preamble{}, ErrNoIBAN
	}
	for _, line := range lines {
		if fields := strings.Split(line, ";"); fields[0] == "Kontostand" && len(fields) > balanceColumn {
			found.closing = fields[balanceColumn]
		}
	}
	found.records = recordsOf(lines)
	return found, nil
}

// rowsOf are the converted transactions as rows, seen from the import account: its own posting gives the
// amount, the posting beside it the category. hledger lists them oldest first; the table shows newest first.
func rowsOf(transactions []domain.Transaction, account string) []Row {
	rows := make([]Row, 0, len(transactions))
	for _, transaction := range slices.Backward(transactions) {
		row := Row{Date: transaction.Date, Description: transaction.Description, Purpose: transaction.Comment}
		for _, posting := range transaction.Postings {
			if posting.Account == account {
				row.Amount = posting.Amounts
				continue
			}
			if posting.Account != Uncategorised {
				row.Target, row.RuleID = posting.Account, posting.Tags[RuleTag]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// booking is what the journal knows of one transaction on the account: enough to recognise it again.
type booking struct {
	date        string
	amount      string
	description string
}

// journalBookings are the account's transactions in the journal, as bookings.
func journalBookings(ledger domain.Ledger, account string) []booking {
	var bookings []booking
	for _, transaction := range ledger.Transactions {
		for _, posting := range transaction.Postings {
			if posting.Account == account {
				bookings = append(bookings, booking{
					transaction.Date.Format(time.DateOnly), posting.Amounts.Format(), transaction.Description,
				})
			}
		}
	}
	return bookings
}

// classify splits the rows by the journal as watermark (ADR 0009): D is the account's latest transaction;
// rows before D are skipped, rows after D are new, and rows on D are matched against the journal's
// transactions of that day by date, amount and description, as a multiset.
func classify(rows []Row, bookings []booking) (fresh, skipped []Row, watermark time.Time) {
	var latest string
	for _, b := range bookings {
		latest = max(latest, b.date)
	}
	if latest == "" {
		return rows, nil, time.Time{}
	}
	watermark, _ = time.Parse(time.DateOnly, latest)
	remaining := map[booking]int{}
	for _, b := range bookings {
		if b.date == latest {
			remaining[b]++
		}
	}
	for i := range rows {
		row := &rows[i]
		key := booking{row.Date.Format(time.DateOnly), row.Amount.Format(), row.Description}
		switch {
		case row.Date.Before(watermark):
			skipped = append(skipped, *row)
		case row.Date.After(watermark):
			fresh = append(fresh, *row)
		case remaining[key] > 0:
			remaining[key]--
			skipped = append(skipped, *row)
		default:
			fresh = append(fresh, *row)
		}
	}
	return fresh, skipped, watermark
}

// Browse lists a directory for the file picker: its folders and its .csv files, folders first.
func (s *Service) Browse(dir string) ([]domain.FileEntry, error) {
	entries, err := s.files.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	listed := slices.DeleteFunc(slices.Clone(entries), func(entry domain.FileEntry) bool {
		return !entry.IsDir && !strings.EqualFold(filepath.Ext(entry.Name), ".csv")
	})
	slices.SortFunc(listed, func(a, b domain.FileEntry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return listed, nil
}

// record is what one line of the export says in the columns konto matches rules against.
type record struct{ date, kind, payee, purpose string }

const (
	typeColumn    = 2
	payeeColumn   = 3
	purposeColumn = 4
)

// recordsOf are the bookings below the export's header line, up to the trailer.
func recordsOf(lines []string) []record {
	var records []record
	inBody := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Buchungstag;") {
			inBody = true
			continue
		}
		fields := strings.Split(line, ";")
		if !inBody || fields[0] == "Kontostand" {
			continue
		}
		date, err := time.Parse("2.1.2006", fields[0])
		if err != nil {
			continue
		}
		records = append(records, record{
			date: date.Format(time.DateOnly), kind: column(fields, typeColumn), payee: column(fields, payeeColumn),
			purpose: column(fields, purposeColumn),
		})
	}
	return records
}

// descriptionOf is what the bank format makes of the record: the payee, else the transaction type.
func (r record) descriptionOf() string {
	if r.payee == "" {
		return r.kind
	}
	return r.payee
}

// withRawFields gives each row the transaction type and payee of the export line it came from, found by
// date, description and purpose.
func withRawFields(rows []Row, records []record) []Row {
	for i := range rows {
		row := &rows[i]
		for _, rec := range records {
			if rec.date == row.Date.Format(time.DateOnly) && rec.descriptionOf() == row.Description &&
				rec.purpose == row.Purpose {
				row.Type, row.Payee = rec.kind, rec.payee
				break
			}
		}
	}
	return rows
}

// column is the field of a split line, empty when the line is too short to have it.
func column(fields []string, index int) string {
	if index >= len(fields) {
		return ""
	}
	return fields[index]
}
