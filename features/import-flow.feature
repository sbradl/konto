@spec-4.9 @adr-0009 @adr-0010
Feature: Import — picking a file, the row table, skipped rows

  Harness note: `a Postbank CSV … :` renders a complete Postbank export (BOM, preamble,
  header, rows in the order given — newest first like the bank — and the `Kontostand`
  trailer). Column meaning: date d.m.yyyy, type = Umsatzart, payee, purpose, amount signed.

  Background:
    Given the journal "household"
    And today is 2026-09-30
    And the terminal is 180×60
    And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
      | date      | type             | payee             | purpose      | amount   |
      | 30.9.2026 | SEPA Lastschrift | REWE Markt        | Einkauf      | -42,17   |
      | 29.9.2026 | Kartenzahlung    | Bäckerei Schmidt  | Brötchen     | -6,50    |
      | 29.9.2026 | Kartenzahlung    | Kaffeehaus Nord   | Kaffee       | -4,10    |
      | 28.9.2026 | Kartenzahlung    | Kaffeehaus        | Kaffee       | -3,20    |
      | 26.9.2026 | Kartenzahlung    | Kiosk             | Zeitung      | -2,00    |
      | 26.9.2026 | Umbuchung        | Sparen            | Sparen       | -500,00  |
      | 25.9.2026 | Gutschrift       | Acme GmbH         | Gehalt       | 3.000,00 |
      | 10.9.2026 | SEPA Lastschrift | REWE              | Einkauf      | -250,00  |
      | 2.9.2026  | Dauerauftrag     | Vermieter Mueller | Miete        | -900,00  |
    And I start konto

  Rule: Start state

    Scenario: Import starts empty
      When I press "2"
      Then I see an empty Import with a hint to press "n" for a new file
      And I do not see a file list
      And I do not see status dots

    Scenario: hint for the Import view
      When I press "2"
      Then the status bar hint lists "n new file"

    Scenario: keys that were dropped do nothing
      Given I press "2"
      When I press "d"
      And I press "u"
      Then the screen is unchanged

    Scenario: w does nothing with no staged import
      Given I press "2"
      When I press "w"
      Then the screen is unchanged
      And the journal is unchanged

  Rule: File picker

    Background:
      Given the XDG download directory contains
        | sept.csv    |
        | notes.txt   |
        | report.pdf  |
        | subdir/     |
      And I press "2"

    Scenario: n opens the picker in the XDG download directory
      When I press "n"
      Then I see a file picker
      And the picker shows the download directory path

    Scenario: only csv files and directories are listed
      When I press "n"
      Then the picker lists "sept.csv" and "subdir"
      And the picker does not list "notes.txt" or "report.pdf"

    Scenario: the download directory falls back to ~/Downloads
      Given XDG_DOWNLOAD_DIR is not configured
      When I press "n"
      Then the picker shows "~/Downloads"

    Scenario: Esc leaves the picker without staging anything
      When I press "n"
      And I press "Esc"
      Then I see the empty Import
      And the journal is unchanged

    Scenario: no remembered directory
      Given I press "n"
      And I enter "subdir" in the picker
      And I press "Esc"
      When I press "n"
      Then the picker shows the download directory path

    Scenario: Enter on a file opens it
      When I press "n"
      And I select "sept.csv" with "Enter"
      Then I see the row table

    @proposed
    Scenario: a missing download directory is not a crash
      Given the XDG download directory does not exist
      When I press "n"
      Then konto is still running
      And I see a file picker or a message naming the missing directory

  Rule: Account detection by IBAN

    Scenario: known IBAN selects the account and shows the info block
      When I open "sept.csv" in Import
      Then the info block shows file "sept.csv"
      And the info block shows account "assets:checking"
      And the info block shows format "postbank"
      And the info block shows watermark "2026-09-26"

    Scenario: no attach-to-existing step is offered
      When I open "sept.csv" in Import
      Then I do not see "attach" or "choose account"

    Scenario Outline: IBAN formatting in the CSV does not matter
      Given a Postbank CSV "spaced.csv" for IBAN "<iban>" with closing balance 13.240,00 EUR:
        | date      | type       | payee     | purpose | amount |
        | 25.9.2026 | Gutschrift | Acme GmbH | Gehalt  | 3.000,00 |
      When I open "spaced.csv" in Import
      Then the info block shows account "assets:checking"

      Examples:
        | iban                          |
        | DE89370400440532013000        |
        | DE89 3704 0044 0532 0130 00   |

    Scenario: unknown IBAN offers to create an account
      Given a Postbank CSV "new.csv" for IBAN "DE27100777770209299700" with closing balance 4.412,34 EUR:
        | date       | type           | payee | purpose | amount |
        | 15.9.2026  | Kontoabrechnung |      | Zinsen  | 12,34  |
      When I open "new.csv" in Import
      Then I see "create account"
      And I do not see a list of existing accounts to attach to

    @proposed
    Scenario Outline: accounts that cannot be imported into are refused with a reason
      Given the journal "household" where <setup>
      And a Postbank CSV "x.csv" for IBAN "<iban>" with closing balance 1,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | X     | x       | 1,00   |
      And I start konto
      When I open "x.csv" in Import
      Then I see "<message>"
      And nothing is staged
      And the journal is unchanged

      Examples:
        | setup                                           | iban                   | message                           |
        | IBAN DE75512108001245126199 is a closed account | DE75512108001245126199 | account closed — reopen with o    |
        | "assets:cash" is manual with IBAN DE91100000000123456789 | DE91100000000123456789 | manual              |
        | "assets:checking" has no source tag             | DE89370400440532013000 | unclassified                      |

    @proposed
    Scenario: the clearing account is never an import target
      Given the journal "household" where the clearing account carries IBAN DE55500105175407324999
      And a Postbank CSV "x.csv" for IBAN "DE55500105175407324999" with closing balance 1,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | X     | x       | 1,00   |
      And I start konto
      When I open "x.csv" in Import
      Then the import is refused with a message naming "assets:transfers"

  Rule: Row table

    Background:
      Given I open "sept.csv" in Import

    Scenario: new rows are listed with match state
      Then the row table has 5 rows
      And the row "REWE Markt" on "2026-09-30" shows "✓" coloured 2 and rule id "2"
      And the row "Bäckerei Schmidt" on "2026-09-29" shows "?" coloured 3
      And the row "Kaffeehaus Nord" on "2026-09-29" shows "?" coloured 3
      And the row "Kaffeehaus" on "2026-09-28" shows "?" coloured 3
      And the row "Kiosk" on "2026-09-26" shows "?" coloured 3

    Scenario: a matched row shows its category dim, an unmatched row says it needs a rule
      When I press "↓"
      Then the row "REWE Markt" shows target "expenses:groceries" coloured 8
      And the row "Kiosk" shows "needs a rule" coloured 3

    Scenario: amounts are shown in the journal's style
      Then the table row "REWE Markt" shows "-42,17 EUR"

    Scenario: the first row is selected, arrows move it
      Then the first table row is selected with background 8 and foreground 15
      When I press "↓"
      Then the second table row is selected
      And the status bar shows "2/5"

    Scenario: cursor limits
      When I press "↑"
      Then the first table row is selected
      When I press "End"
      Then the last table row is selected
      When I press "↓"
      Then the last table row is selected

    @proposed
    Scenario: a summary tells how many rows still need a category
      Then I see "4 need a category"

    Scenario: CSV columns konto does not write are not shown
      Then I do not see "Kundenreferenz" or "Mandatsreferenz" or "BIC"

    Scenario: only matched rows carry a rule id
      Then the row "Kiosk" shows no rule id

  Rule: Skipped rows

    Background:
      Given I open "sept.csv" in Import

    Scenario: skipped rows are a list, not a count
      Then I see a "SKIPPED" list with exactly these rows
        | 2026-09-26 | Sparen            | -500,00 EUR   |
        | 2026-09-25 | Acme GmbH         | 3.000,00 EUR  |
        | 2026-09-10 | REWE              | -250,00 EUR   |
        | 2026-09-02 | Vermieter Mueller | -900,00 EUR   |
      And the skipped rows are coloured 8

    Scenario: a count alone is never the only thing shown
      Then the skipped panel shows 4 rows, not just the number 4

    Scenario: skipped rows are not in the row table
      Then the row table has no row "Acme GmbH"
      And the row table has no row "Vermieter Mueller"

    Scenario: skipped rows need no category and never block the write
      Then "w" is blocked only by the 4 new rows that need a category

    Scenario: a file with nothing skipped shows an empty skipped panel
      Given a Postbank CSV "fresh.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung | -1,00 |
      When I open "fresh.csv" in Import
      Then the skipped panel is empty

  Rule: Journal is the watermark (ADR 0009)

    Scenario: rows before D are skipped, after D are new
      When I open "sept.csv" in Import
      Then the rows dated before "2026-09-26" are skipped
      And the rows dated after "2026-09-26" are new

    Scenario: rows on D are compared by date, amount and description
      When I open "sept.csv" in Import
      Then "Sparen" of -500,00 on "2026-09-26" is skipped
      And "Kiosk" of -2,00 on "2026-09-26" is new

    Scenario: same date and amount but different description is new
      Given a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 12.740,00 EUR:
        | date      | type       | payee       | purpose | amount  |
        | 26.9.2026 | Umbuchung  | Sparkonto   | Sparen  | -500,00 |
        | 26.9.2026 | Umbuchung  | Sparen      | Sparen  | -500,00 |
      When I open "x.csv" in Import
      Then the row "Sparkonto" is new
      And "Sparen" is skipped

    Scenario: same date and description but different amount is new
      Given a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 12.740,00 EUR:
        | date      | type       | payee  | purpose | amount  |
        | 26.9.2026 | Umbuchung  | Sparen | Sparen  | -500,00 |
        | 26.9.2026 | Umbuchung  | Sparen | Sparen  | -250,00 |
      When I open "x.csv" in Import
      Then the row "Sparen" of -250,00 is new
      And the row "Sparen" of -500,00 is skipped

    Scenario: multiset — two identical rows against one in the journal give one new
      Given the journal "household" with an extra checking transaction on 2026-09-26: description "Kaffeehaus", category "expenses:groceries", amount 3,20 EUR
      And a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.233,60 EUR:
        | date      | type          | payee      | purpose | amount |
        | 26.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
        | 26.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
        | 26.9.2026 | Umbuchung     | Sparen     | Sparen  | -500,00 |
      And I start konto
      When I open "x.csv" in Import
      Then the row table has exactly 1 row
      And the skipped list has 2 rows

    Scenario: multiset — two identical rows and none in the journal give two new
      Given a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.233,60 EUR:
        | date      | type          | payee      | purpose | amount |
        | 28.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
        | 28.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -3,20  |
      When I open "x.csv" in Import
      Then the row table has exactly 2 rows

    Scenario: the watermark is the account's latest transaction whatever its kind
      Given the journal "household" with a later checking transaction on 2026-09-29
      And a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date      | type          | payee      | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk      | Zeitung | -1,00  |
        | 28.9.2026 | Kartenzahlung | Kiosk      | Zeitung | -1,00  |
      And I start konto
      When I open "x.csv" in Import
      Then the info block shows watermark "2026-09-29"
      And the row "Kiosk" on "2026-09-28" is skipped

    Scenario: an account's watermark ignores other accounts' later transactions
      Given the journal "household" with a later savings transaction on 2026-09-29
      When I open "sept.csv" in Import
      Then the info block shows watermark "2026-09-26"

    Scenario: a fresh account has no watermark, every row is new
      Given a Postbank CSV "new.csv" for IBAN "DE27100777770209299700" with closing balance 4.412,34 EUR:
        | date      | type            | payee | purpose | amount |
        | 15.9.2026 | Kontoabrechnung |       | Zinsen  | 12,34  |
        | 20.8.2026 | Auszahlung      |       | Geld    | -100,00 |
      When I open "new.csv" in Import
      And I create the account "assets:holiday"
      Then the info block shows no watermark
      And the row table has exactly 2 rows
      And the skipped panel is empty

    Scenario: no state file is ever created
      When I open "sept.csv" in Import
      Then no ".latest" file exists next to the journal or the CSV
      And no file other than the journal and categories.rules exists next to the journal

    @proposed
    Scenario: a fully known CSV has nothing to import
      Given a Postbank CSV "known.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date      | type       | payee     | purpose | amount   |
        | 25.9.2026 | Gutschrift | Acme GmbH | Gehalt  | 3.000,00 |
      When I open "known.csv" in Import
      Then the row table has no rows
      And the skipped list has 1 row
      And I see "nothing new"
      When I press "w"
      Then the journal is unchanged

  Rule: Bad input

    @proposed
    Scenario Outline: files that are not a Postbank export are refused
      Given a file "bad.csv" in the download directory containing "<content>"
      And I press "2"
      When I open "bad.csv" in Import
      Then I see a message saying the file is not a recognised export
      And konto is still running
      And nothing is staged
      And the journal is unchanged

      Examples:
        | content                      |
        | just,some,other,csv\n1,2,3   |
        |                              |
        | not a csv at all             |

    @proposed
    Scenario: a CSV with a preamble but no IBAN is refused
      Given a Postbank CSV without an IBAN line named "noiban.csv"
      When I open "noiban.csv" in Import
      Then I see a message saying the account could not be detected

    Scenario: hledger failing on the conversion is reported, not fatal
      Given hledger fails on CSV conversion
      When I open "sept.csv" in Import
      Then the status bar shows an error indicator
      And I press "!"
      And I see hledger's error text
      And konto is still running

    @proposed
    Scenario: a binary file is refused
      Given a file "blob.csv" in the download directory with random binary content
      When I open "blob.csv" in Import
      Then I see a message saying the file is not a recognised export
      And konto is still running
