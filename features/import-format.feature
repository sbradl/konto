@spec-4.9
Feature: Import — Postbank CSV format mechanics

  The bank format is built into konto. These scenarios feed real, raw Postbank-shaped bytes
  (synthetic values) so the parsing rules themselves are pinned down. Dates are d.m.yyyy
  without zero padding, amounts German, UTF-8 with BOM, `;`-separated.

  Background:
    Given the journal "household"
    And today is 2026-09-30
    And the terminal is 180×60

  Rule: Raw file layout

    Background:
      Given the download directory contains "raw.csv" with these exact bytes (UTF-8 BOM first, LF line ends):
        """
        Umsätze
        Konto;Kontonummer;IBAN;Währung
        Postbank Giro plus;0123456789;DE89370400440532013000;EUR

        1.9.2026 - 30.9.2026
        Letzter Kontostand;;;;99.999,99;EUR
        Vorgemerkte und noch nicht gebuchte Umsätze sind nicht Bestandteil dieser Übersicht.
        Buchungstag;Wert;Umsatzart;Begünstigter / Auftraggeber;Verwendungszweck;IBAN / Kontonummer;BIC;Kundenreferenz;Mandatsreferenz;Gläubiger ID;Fremde Gebühren;Betrag;Abweichender Empfänger;Anzahl der Aufträge;Anzahl der Schecks;Soll;Haben;Währung
        30.9.2026;1.10.2026;Kartenzahlung;Kiosk;Zeitung;DE00000000000000000001;ABCDEFGH;ref1;;;;-2,00;;;;-2,00;;EUR
        29.9.2026;29.9.2026;Gutschrift;Oma;Geburtstag;DE00000000000000000002;ABCDEFGH;ref2;;;;1.023,45;;;;;1.023,45;EUR
        Kontostand;30.9.2026;;;14.261,45;EUR
        """
    Scenario: preamble, trailer and value dates are ignored; booking date and trailer balance are used
      Given I start konto
      When I press "2"
      And I open "raw.csv" in Import
      Then the row table has exactly 2 rows
      And the row "Kiosk" is dated "2026-09-30" and shows "-2,00 EUR"
      And the row "Oma" is dated "2026-09-29" and shows "1.023,45 EUR"
      And the info block shows account "assets:checking"
      And no row is dated "2026-10-01"
      And no row is named "Kontostand"
    Scenario: the trailer balance, not "Letzter Kontostand", becomes the assertion
      Given I start konto
      When I import "raw.csv" categorising every row
      Then the newest imported transaction has the assertion "= 14.261,45 EUR" on "assets:checking"
      And no assertion in the journal mentions "99.999,99"
    Scenario Outline: German amounts
      Given a Postbank CSV "a.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date      | type          | payee | purpose | amount   |
        | 29.9.2026 | Kartenzahlung | Kiosk | x       | <amount> |
      And I start konto
      When I open "a.csv" in Import
      Then the table row "Kiosk" shows "<shown>"

      Examples:
        | amount      | shown          |
        | -6,50       | -6,50 EUR      |
        | -1.023,45   | -1.023,45 EUR  |
        | 1.000.000,00| 1.000.000,00 EUR |
        | 0,01        | 0,01 EUR       |
        | -0,01       | -0,01 EUR      |
    Scenario Outline: dates without zero padding
      Given a Postbank CSV "d.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date   | type          | payee | purpose | amount |
        | <date> | Kartenzahlung | Kiosk | x       | -1,00  |
      And today is 2027-01-15
      And I start konto
      When I open "d.csv" in Import
      Then the row "Kiosk" is dated "<iso>"

      Examples:
        | date       | iso        |
        | 1.10.2026  | 2026-10-01 |
        | 9.11.2026  | 2026-11-09 |
        | 31.12.2026 | 2026-12-31 |
        | 1.1.2027   | 2027-01-01 |
        | 29.2.2028  | 2028-02-29 |
    Scenario: umlauts and symbols survive
      Given a Postbank CSV "u.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee              | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Bäckerei Müller & Söhne | Brötchen für 5 € | -1,00 |
      And I start konto
      When I open "u.csv" in Import
      Then the row "Bäckerei Müller & Söhne" is shown unmangled
    Scenario: the purpose is the transaction comment
      Given a Postbank CSV "p.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee | purpose      | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung vom Montag | -1,00 |
      And I start konto
      When I import "p.csv" categorising every row
      Then the journal contains
        """
        2026-09-29 Kiosk  ; Zeitung vom Montag
        """
    Scenario: an empty payee falls back to the transaction type as description
      Given a Postbank CSV "e.csv" for IBAN "DE89370400440532013000" with closing balance 13.140,00 EUR:
        | date      | type       | payee | purpose     | amount  |
        | 29.9.2026 | Auszahlung |       | Geldautomat | -100,00 |
      And I start konto
      When I import "e.csv" categorising every row
      Then the journal contains
        """
        2026-09-29 Auszahlung  ; Geldautomat
        """
    Scenario: identical same-day rows are both imported
      Given a Postbank CSV "i.csv" for IBAN "DE89370400440532013000" with closing balance 13.233,60 EUR:
        | date      | type          | payee      | purpose | amount |
        | 28.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
        | 28.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
      And I start konto
      When I import "i.csv" categorising every row
      Then the journal has exactly 2 "Kaffeehaus" transactions on "2026-09-28"
    Scenario: rows are written oldest-first although the CSV is newest-first
      Given a Postbank CSV "o.csv" for IBAN "DE89370400440532013000" with closing balance 13.237,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 30.9.2026 | Kartenzahlung | B     | x       | -1,00  |
        | 29.9.2026 | Kartenzahlung | A     | x       | -2,00  |
      And I start konto
      When I import "o.csv" categorising every row
      Then in the journal "A" on "2026-09-29" is above "B" on "2026-09-30"
    Scenario: two exports with an overlapping period import cleanly one after the other
      Given a Postbank CSV "first.csv" for IBAN "DE89370400440532013000" with closing balance 13.238,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 28.9.2026 | Kartenzahlung | A     | x       | -2,00  |
      And a Postbank CSV "second.csv" for IBAN "DE89370400440532013000" with closing balance 13.236,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | B     | x       | -2,00  |
        | 28.9.2026 | Kartenzahlung | A     | x       | -2,00  |
      And I start konto
      When I import "first.csv" categorising every row
      And I import "second.csv" categorising every row
      Then the journal has exactly 1 "A" transaction on "2026-09-28"
      And the journal has exactly 1 "B" transaction on "2026-09-29"
      And the journal balance of "assets:checking" is 13.236,00 EUR
    Scenario: the currency column is EUR
      Given a Postbank CSV "a.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | x       | -1,00  |
      And I start konto
      When I import "a.csv" categorising every row
      Then every imported posting is in "EUR"

  Rule: Malformed content

    @proposed
    Scenario Outline: a broken row is reported, never written, never a crash
      Given a Postbank CSV "m.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR and a broken row "<row>"
      And I start konto
      When I open "m.csv" in Import
      Then konto is still running
      And I see a message about the unreadable row or file
      And the journal is unchanged

      Examples:
        | row                                      |
        | 32.13.2026;;Kartenzahlung;X;y;;;;;;;-1,00;;;;;;EUR |
        | 29.9.2026;;Kartenzahlung;X;y;;;;;;;abc;;;;;;EUR    |
        | 29.9.2026                                |

    @proposed
    Scenario: a trailing empty line is harmless
      Given a Postbank CSV "t.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR and a trailing blank line:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | x       | -1,00  |
      And I start konto
      When I open "t.csv" in Import
      Then the row table has exactly 1 row

    @proposed
    Scenario: CRLF line endings are tolerated
      Given a Postbank CSV "crlf.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR and CRLF line endings:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | x       | -1,00  |
      And I start konto
      When I open "crlf.csv" in Import
      Then the row table has exactly 1 row
