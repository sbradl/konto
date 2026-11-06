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

// formatNamed is the bank format of the name, as Staged.Format and an account's konto:format tag name it.
func formatNamed(name string) (bankFormat, bool) {
	formats := []bankFormat{postbank()}

	at := slices.IndexFunc(formats, func(format bankFormat) bool { return format.name == name })
	if at < 0 {
		return bankFormat{}, false
	}

	return formats[at], true
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
		"comment %" + fieldPurpose, "amount %" + fieldAmount + " " + b.currency, "account2 " + unclaimedAccount, "",
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
	lines := exportLines(data)
	if !slices.ContainsFunc(lines, b.isHeader) {
		return preamble{}, ErrNotAnExport
	}

	iban := b.ibanOf(lines)
	if iban == "" {
		return preamble{}, ErrNoIBAN
	}

	return preamble{iban: iban, closing: b.closingOf(lines), records: b.recordsOf(lines)}, nil
}

// exportLines are the export's lines, without a byte order mark and Windows line ends.
func exportLines(data []byte) []string {
	text := strings.TrimPrefix(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\xef\xbb\xbf")

	return strings.Split(text, "\n")
}

// isHeader reports the line the bookings follow.
func (b bankFormat) isHeader(line string) bool { return strings.HasPrefix(line, b.header) }

// ibanOf is the IBAN the preamble names; empty when it names none.
func (b bankFormat) ibanOf(lines []string) string {
	if b.ibanLine >= len(lines) {
		return ""
	}

	return domain.NormalizeIBAN(column(strings.Split(lines[b.ibanLine], separator), b.ibanColumn))
}

// closingOf is the closing balance the trailer prints, as printed.
func (b bankFormat) closingOf(lines []string) string {
	var closing string
	for _, line := range lines {
		if fields := strings.Split(line, separator); fields[0] == b.trailer && len(fields) > b.balanceColumn {
			closing = fields[b.balanceColumn]
		}
	}

	return closing
}

// record is what one line of the export says in the columns konto matches rules against, and what tells it
// from the other lines.
type record struct {
	key         rowKey
	kind, payee string
}

// recordsOf are the bookings below the export's header line, up to the trailer.
func (b bankFormat) recordsOf(lines []string) []record {
	var records []record
	inBody := false
	for _, line := range lines {
		if b.isHeader(line) {
			inBody = true

			continue
		}

		fields := strings.Split(line, separator)
		if !inBody || fields[0] == b.trailer {
			continue
		}

		if rec, ok := b.recordOf(fields); ok {
			records = append(records, rec)
		}
	}

	return records
}

// recordOf reads the fields of one booking line; false when its date does not read, as on a line that is no
// booking.
func (b bankFormat) recordOf(fields []string) (record, bool) {
	date, err := time.Parse(b.dateLayout, column(fields, b.dateColumn))
	if err != nil {
		return record{}, false
	}
	kind, payee := column(fields, b.typeColumn), column(fields, b.payeeColumn)
	seen := booking{date.Format(time.DateOnly), b.value(column(fields, b.amountColumn)), descriptionOf(kind, payee)}

	return record{key: rowKey{seen, column(fields, b.purposeColumn)}, kind: kind, payee: payee}, true
}

// value is the amount column as MultiAmount.Value has it, to compare with a converted row; empty when it is
// no number, which no converted row matches.
func (b bankFormat) value(text string) string {
	amount, err := b.amount(domain.DefaultAmount(b.currency), text)
	if err != nil {
		return ""
	}

	return domain.MultiAmount{amount}.Value()
}

// descriptionOf is what the rules make of a line (`if %payee ^$`): the payee, else the transaction type.
func descriptionOf(kind, payee string) string {
	if payee == "" {
		return kind
	}

	return payee
}

// errUnmatchedRow is konto reading a line of the export other than hledger did, so the rules konto drafts
// would test fields the row does not have.
var errUnmatchedRow = errors.New("a converted row matches no line of the export")

// withRawFields gives each row the transaction type and payee of the export line it came from, found by its
// key; each line gives its fields to one row only, so same-day bookings alike but for their type keep theirs.
func withRawFields(rows []Row, records []record) ([]Row, error) {
	unused := map[rowKey][]record{}
	for _, rec := range records {
		unused[rec.key] = append(unused[rec.key], rec)
	}
	for i := range rows {
		row := &rows[i]

		found := unused[row.key()]
		if len(found) == 0 {
			return nil, fmt.Errorf("%w: %s %s", errUnmatchedRow, row.Date.Format(time.DateOnly), row.Description)
		}
		row.Type, row.Payee = found[0].kind, found[0].payee
		unused[row.key()] = found[1:]
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
