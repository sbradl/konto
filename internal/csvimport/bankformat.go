package csvimport

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// FormatPostbank names the one bank format konto understands.
const FormatPostbank = "postbank"

// The fields of a booking as the hledger rules name them; a Condition tests type, payee and purpose.
const (
	fieldType    = "type"
	fieldPayee   = "payee"
	fieldPurpose = "purpose"
	fieldAmount  = "betrag"
)

// bankFormat is how one bank lays out its export: konto finds the IBAN, the closing balance and the raw columns
// rules are matched against in it, and the hledger rules that convert the bookings are written from it, so the
// two readings never disagree.
type bankFormat struct {
	name          string
	currency      string // the commodity of every amount in the export
	skip          int    // the preamble lines above the header
	header        string // the start of the line the bookings follow
	trailer       string // the first field of the line with the closing balance
	dateLayout    string // the booking date, as Go reads it
	dateFormat    string // the same, as hledger reads it
	decimalMark   string
	groupMark     string
	ibanLine      int
	ibanColumn    int
	balanceColumn int
	dateColumn    int
	typeColumn    int
	payeeColumn   int
	purposeColumn int
	amountColumn  int
}

// postbank is the Postbank layout: the IBAN on the third line, the closing balance on the trailer.
//
//nolint:mnd // the numbers are the layout
func postbank() bankFormat {
	return bankFormat{
		name: FormatPostbank, currency: "EUR", skip: 7, header: "Buchungstag;", trailer: "Kontostand",
		dateLayout: "2.1.2006", dateFormat: "%-d.%-m.%Y", decimalMark: ",", groupMark: ".",
		ibanLine: 2, ibanColumn: 2, balanceColumn: 4,
		dateColumn: 0, typeColumn: 2, payeeColumn: 3, purposeColumn: 4, amountColumn: 11,
	}
}

// rules is the hledger rules preamble that converts the bookings: the header and the trailer are skipped,
// the payee, else the type, is the description, the purpose the comment, and no row has a category yet.
func (b bankFormat) rules() string {
	fields := make([]string, b.amountColumn+1)
	fields[b.dateColumn], fields[b.typeColumn], fields[b.payeeColumn] = "date", fieldType, fieldPayee
	fields[b.purposeColumn], fields[b.amountColumn] = fieldPurpose, fieldAmount
	return strings.Join([]string{
		"skip " + strconv.Itoa(b.skip), "separator " + separator, "decimal-mark " + b.decimalMark,
		"date-format " + b.dateFormat, "newest-first", "fields " + strings.Join(fields, ", "),
		"if ^" + strings.TrimSuffix(b.header, separator), " skip", "if ^" + b.trailer, " skip",
		"description %" + fieldPayee, "if %" + fieldPayee + " ^$", " description %" + fieldType,
		"comment %" + fieldPurpose, "amount %" + fieldAmount + " " + b.currency, "account2 " + Uncategorised, "",
	}, "\n")
}

// amount reads a number as the export prints it (13.182,03) in the commodity and style of like.
func (b bankFormat) amount(like domain.Amount, text string) (domain.Amount, error) {
	mantissa, err := domain.ParseNumber(text, b.decimalMark, b.groupMark, like.Places)
	if err != nil {
		return domain.Amount{}, err //nolint:wrapcheck // the number's own words are the answer
	}
	like.Mantissa = mantissa
	return like, nil
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
		date, err := time.Parse(b.dateLayout, column(fields, b.dateColumn))
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

// errUnmatchedRow is konto reading a line of the export other than hledger did, so the rules konto drafts
// would test fields the row does not have.
var errUnmatchedRow = errors.New("a converted row matches no line of the export")

// withRawFields gives each row the transaction type and payee of the export line it came from, found by
// date, description and purpose.
func withRawFields(rows []Row, records []record) ([]Row, error) {
	for i := range rows {
		row := &rows[i]
		at := slices.IndexFunc(records, func(rec record) bool {
			return rec.date == row.Date.Format(time.DateOnly) && rec.descriptionOf() == row.Description &&
				rec.purpose == row.Purpose
		})
		if at < 0 {
			return nil, fmt.Errorf("%w: %s %s", errUnmatchedRow, row.Date.Format(time.DateOnly), row.Description)
		}
		row.Type, row.Payee = records[at].kind, records[at].payee
	}
	return rows, nil
}

// column is the field of a split line, empty when the line is too short to have it, trimmed as hledger trims it.
func column(fields []string, index int) string {
	if index >= len(fields) {
		return ""
	}
	return strings.TrimSpace(fields[index])
}
