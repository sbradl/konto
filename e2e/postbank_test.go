//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// postbankRow is one booking in a generated Postbank export.
type postbankRow struct {
	Date    string // d.m.yyyy as in the export
	Type    string // Umsatzart
	Payee   string // Begünstigter / Auftraggeber
	Purpose string // Verwendungszweck
	Amount  string // German, signed: -1.023,45
}

type postbankExport struct {
	IBAN           string
	ClosingBalance string // German, e.g. 13.240,00
	Rows           []postbankRow
	BrokenRow      string // written verbatim after the rows, if set
	TrailingBlank  bool
	CRLF           bool
}

const postbankHeader = "Buchungstag;Wert;Umsatzart;Begünstigter / Auftraggeber;Verwendungszweck;" +
	"IBAN / Kontonummer;BIC;Kundenreferenz;Mandatsreferenz;Gläubiger ID;Fremde Gebühren;Betrag;" +
	"Abweichender Empfänger;Anzahl der Aufträge;Anzahl der Schecks;Soll;Haben;Währung"

// render produces the export bytes: UTF-8 BOM, 7-line preamble plus a blank
// line, header, rows as given (the bank lists newest first), Kontostand
// trailer dated at the newest row. Layout per ticket 08; values synthetic.
func (e postbankExport) render() ([]byte, error) {
	from, to, err := e.period()
	if err != nil {
		return nil, err
	}
	lines := []string{
		"Umsätze",
		"Konto;Kontonummer;IBAN;Währung",
		"Postbank Giro plus;0123456789;" + e.IBAN + ";EUR",
		"",
		from + " - " + to,
		"Letzter Kontostand;;;;" + e.ClosingBalance + ";EUR",
		"Vorgemerkte und noch nicht gebuchte Umsätze sind nicht Bestandteil dieser Übersicht.",
		postbankHeader,
	}
	for i, r := range e.Rows {
		soll, haben := r.Amount, ""
		if !strings.HasPrefix(r.Amount, "-") {
			soll, haben = "", r.Amount
		}
		lines = append(lines, strings.Join([]string{
			r.Date, r.Date, r.Type, r.Payee, r.Purpose,
			fmt.Sprintf("DE%020d", i+1), "ABCDEFGH", fmt.Sprintf("ref%d", i+1), "", "", "",
			r.Amount, "", "", "", soll, haben, "EUR",
		}, ";"))
	}

	if e.BrokenRow != "" {
		lines = append(lines, e.BrokenRow)
	}

	lines = append(lines, "Kontostand;"+to+";;;"+e.ClosingBalance+";EUR")
	if e.TrailingBlank {
		lines = append(lines, "")
	}

	eol := "\n"
	if e.CRLF {
		eol = "\r\n"
	}

	return []byte("\xef\xbb\xbf" + strings.Join(lines, eol) + eol), nil
}

// period is the oldest and newest booking date, in the export's own format.
func (e postbankExport) period() (from, to string, err error) {
	if len(e.Rows) == 0 {
		return "", "", errors.New("a Postbank export needs at least one row")
	}
	var oldest, newest time.Time
	for i, r := range e.Rows {
		d, err := time.Parse("2.1.2006", r.Date)
		if err != nil {
			return "", "", fmt.Errorf("row %d: date %q is not d.m.yyyy", i+1, r.Date)
		}

		if i == 0 || d.Before(oldest) {
			oldest = d
		}

		if i == 0 || d.After(newest) {
			newest = d
		}
	}

	return oldest.Format("2.1.2006"), newest.Format("2.1.2006"), nil
}
