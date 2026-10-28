package csvimport

import (
	"bytes"
	_ "embed" // the bank format ships inside the binary
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

//go:embed postbank.rules
var postbankRules string

// FormatPostbank names the one bank format konto understands.
const FormatPostbank = "postbank"

// bankFormat is how one bank lays out its export: the hledger rules that convert the bookings, and where konto
// itself finds the IBAN, the closing balance and the raw columns rules are matched against.
type bankFormat struct {
	name          string
	rules         string // the hledger rules preamble; its field names are the ones a Condition tests
	header        string // the start of the line the bookings follow
	trailer       string // the first field of the line with the closing balance
	dateLayout    string
	ibanLine      int
	ibanColumn    int
	balanceColumn int
	typeColumn    int
	payeeColumn   int
	purposeColumn int
}

// postbank is the Postbank layout: the IBAN on the third line, the closing balance on the trailer.
//
//nolint:mnd // the numbers are the layout
func postbank() bankFormat {
	return bankFormat{
		name: FormatPostbank, rules: postbankRules, header: "Buchungstag;", trailer: "Kontostand",
		dateLayout: "2.1.2006", ibanLine: 2, ibanColumn: 2, balanceColumn: 4,
		typeColumn: 2, payeeColumn: 3, purposeColumn: 4,
	}
}

const separator = ";"

// preamble is what the export says about itself.
type preamble struct {
	iban, closing string
	records       []record
}

// readPreamble recognises an export of the format and reads its IBAN and the trailer's closing balance.
func (b bankFormat) readPreamble(data []byte) (preamble, error) {
	text := strings.TrimPrefix(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\xef\xbb\xbf")
	lines := strings.Split(text, "\n")
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, b.header) }) {
		return preamble{}, ErrNotAnExport
	}
	var found preamble
	if b.ibanLine < len(lines) {
		found.iban = domain.NormalizeIBAN(column(strings.Split(lines[b.ibanLine], separator), b.ibanColumn))
	}
	if found.iban == "" {
		return preamble{}, ErrNoIBAN
	}
	for _, line := range lines {
		if fields := strings.Split(line, separator); fields[0] == b.trailer && len(fields) > b.balanceColumn {
			found.closing = fields[b.balanceColumn]
		}
	}
	found.records = b.recordsOf(lines)
	return found, nil
}

// record is what one line of the export says in the columns konto matches rules against.
type record struct{ date, kind, payee, purpose string }

// recordsOf are the bookings below the export's header line, up to the trailer.
func (b bankFormat) recordsOf(lines []string) []record {
	var records []record
	inBody := false
	for _, line := range lines {
		if strings.HasPrefix(line, b.header) {
			inBody = true
			continue
		}
		fields := strings.Split(line, separator)
		if !inBody || fields[0] == b.trailer {
			continue
		}
		date, err := time.Parse(b.dateLayout, fields[0])
		if err != nil {
			continue
		}
		records = append(records, record{
			date: date.Format(time.DateOnly), kind: column(fields, b.typeColumn), payee: column(fields, b.payeeColumn),
			purpose: column(fields, b.purposeColumn),
		})
	}
	return records
}

// descriptionOf is what the rules make of the record (`if %payee ^$`): the payee, else the transaction type.
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
