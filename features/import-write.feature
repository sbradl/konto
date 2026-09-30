@spec-4.9 @spec-10 @adr-0009
Feature: Import — writing, assertions, opening balance, staging and quitting

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

  Rule: w is gated on categorisation (ADR 0008)

    @proposed
    Scenario: w is blocked while ? rows remain
      Given I open "sept.csv" in Import
      When I press "w"
      Then I see "4 rows need a category"
      And no y/n prompt is shown
      And the journal is unchanged

    @proposed
    Scenario: rows cannot be skipped
      Given I open "sept.csv" in Import
      Then no key skips or dismisses a "?" row
      And I do not see "skip" in the key hints

    Scenario: there is no catch-all account
      Given I open "sept.csv" in Import and categorise every row
      When I write the import
      Then the journal contains no "unknown" or "uncategorized" account

    Scenario: w asks y/n and shows what will be written
      Given I open "sept.csv" in Import and categorise every row
      When I press "w"
      Then I see a y/n prompt
      And I see a preview containing "2026-09-30 REWE Markt"
      And the journal is unchanged

    Scenario Outline: n, Esc cancel the write; the staged import survives
      Given I open "sept.csv" in Import and categorise every row
      And I press "w"
      When I press "<key>"
      Then the journal is unchanged
      And the row table is still shown with every row categorised

      Examples:
        | key |
        | n   |
        | Esc |

  Rule: The written transactions

    Background:
      Given I open "sept.csv" in Import
      And I categorise the rows
        | row               | how                                   |
        | Bäckerei Schmidt  | new rule → expenses:groceries         |
        | Kaffeehaus        | new rule → create expenses:cafe       |
        | Kiosk             | once → expenses:books                 |
      And I press "w"
      And I press "y"

    Scenario: transactions land in the journal with purpose as comment and rule id on the category posting
      Then the journal contains
        """
        2026-09-28 Kaffeehaus  ; Kaffee
            expenses:cafe  3,20 EUR  ; konto-rule:7
            assets:checking
        """
      And the journal contains
        """
        2026-09-26 Kiosk  ; Zeitung
            expenses:books  2,00 EUR
            assets:checking
        """

    Scenario: a once-categorised row carries no rule tag
      Then the journal's "Kiosk" transaction on "2026-09-26" has no "konto-rule" tag

    Scenario: only the newest imported transaction carries the closing-balance assertion
      Then the "2026-09-30 REWE Markt" transaction has the assertion "= 13.182,03 EUR" on "assets:checking"
      And no other imported transaction has an assertion

    Scenario: booking date only, no secondary date
      Then no imported transaction has a secondary date

    Scenario: nothing but date, payee, purpose and rule id is written
      Then the imported transactions contain no IBAN, BIC, reference or transaction type

    Scenario: skipped rows are not written again
      Then the journal has exactly 1 "Acme GmbH" transaction on "2026-09-25"
      And the journal has exactly 1 "Sparen" checking transaction on "2026-09-26"

    Scenario: the journal stays date-ordered and every check passes
      Then every hledger check passes
      And the journal balance of "assets:checking" is 13.182,03 EUR

    Scenario: new payees are declared, known payees are not redeclared
      Then the journal declares "payee Kaffeehaus"
      And the journal declares "payee Bäckerei Schmidt"
      And the journal declares "payee REWE Markt"
      And "payee REWE" is declared exactly once

    Scenario: new categories are declared
      Then the journal declares "account expenses:cafe"

    Scenario: konto-rule tag is declared
      Then the journal declares "tag konto-rule"

    Scenario: rules are written to categories.rules in the same unit
      Then categories.rules contains
        """
        if %payee Bäckerei Schmidt
            account2 expenses:groceries
            comment2 konto-rule:6

        if %payee Kaffeehaus
            account2 expenses:cafe
            comment2 konto-rule:7
        """
      And categories.rules still contains rules 1 to 5 unchanged
      And categories.rules does not mention "Kiosk"

    Scenario: no other files are created
      Then no ".latest" file exists
      And no temporary rules file is left behind
      And no file other than the journal and categories.rules was written

  Rule: Write target and ordering

    Scenario: the account's latest transaction's file receives the rows
      Given the journal "household" where every "assets:checking" transaction lives in included file "sub/checking.journal"
      And I start konto
      When I import "sept.csv" categorising every row
      Then the new transactions are in "sub/checking.journal"
      And the main journal has no new transaction

    Scenario: new directives go to the main journal even when rows go to an include
      Given the journal "household" where every "assets:checking" transaction lives in included file "sub/checking.journal"
      And I start konto
      When I import "sept.csv" creating the category "expenses:cafe" for "Kaffeehaus"
      Then the main journal contains "account expenses:cafe"

    Scenario: rows are inserted at their date position, not appended
      Given the journal "household" with an unrelated transaction dated 2026-09-29 as the last entry of the file
      And I start konto
      When I import "sept.csv" categorising every row
      Then the "2026-09-28 Kaffeehaus" transaction is above the unrelated "2026-09-29" transaction
      And hledger check ordereddates passes

    Scenario: a transaction dated after every existing one is appended at the end
      When I import "sept.csv" categorising every row
      Then the last transaction of the file is dated "2026-09-30"

  Rule: Balance assertion blocks a wrong import

    Scenario: a CSV with missing rows is blocked, showing bank, journal and difference
      Given a Postbank CSV "gap.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
        | date      | type          | payee      | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -6,50  |
      When I import "gap.csv" categorising every row
      Then the write is blocked
      And I see "bank" "13.182,03 EUR"
      And I see "journal" "13.233,50 EUR"
      And I see "difference" "-51,47 EUR"
      And the journal is unchanged
      And categories.rules is unchanged

    Scenario: the staged import survives a blocked write
      Given a Postbank CSV "gap.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
        | date      | type          | payee      | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -6,50  |
      When I import "gap.csv" categorising every row
      Then the row table is still shown

    Scenario: the pre-check runs on stdin, never on a half-written file
      Given a Postbank CSV "gap.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
        | date      | type          | payee      | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kaffeehaus | Kaffee  | -6,50  |
      When I import "gap.csv" categorising every row
      Then the journal file was never modified during the attempt

    Scenario: a correct CSV passes the assertion
      When I import "sept.csv" categorising every row
      Then the write succeeds

    @proposed
    Scenario: a journal with a failing assertion never reaches Import
      Given the journal "household" with a wrong assertion "= 99,00 EUR" on the last "assets:checking" posting
      When I start konto
      Then I do not see the tab bar

  Rule: After the write

    Scenario: confirmation, empty start state, refreshed views
      When I import "sept.csv" categorising every row
      Then the status bar shows "imported 5 transactions into assets:checking"
      And I see the empty Import
      And the status bar shows "● clean"

    Scenario: the dashboard reflects the import
      When I import "sept.csv" categorising every row
      And I press "1"
      Then the row "assets:checking" shows "13.182,03 EUR"
      And the Recent panel's first row is dated "2026-09-30"
      And the tab bar shows "59" transactions

    Scenario: n picks the next file
      Given I import "sept.csv" categorising every row
      When I press "n"
      Then I see a file picker

    Scenario: importing the same file again finds nothing new
      Given I import "sept.csv" categorising every row
      When I open "sept.csv" in Import
      Then the row table has no rows
      And the skipped list has 9 rows

    Scenario: an overlapping second export imports only the new rows
      Given I import "sept.csv" categorising every row
      And a Postbank CSV "later.csv" for IBAN "DE89370400440532013000" with closing balance 13.171,03 EUR:
        | date      | type          | payee | purpose | amount |
        | 30.9.2026 | Kartenzahlung | Kiosk | Zeitung | -2,00  |
        | 30.9.2026 | SEPA Lastschrift | REWE Markt | Einkauf | -42,17 |
        | 29.9.2026 | Kartenzahlung | Kaffeehaus Nord | Kaffee | -4,10 |
      When I open "later.csv" in Import
      Then the row table has exactly 1 row
      And the row "Kiosk" is new

  Rule: First import into a new account

    Background:
      Given a Postbank CSV "new.csv" for IBAN "DE27100777770209299700" with closing balance 4.412,34 EUR:
        | date      | type            | payee          | purpose             | amount  |
        | 15.9.2026 | Kontoabrechnung |                | Zinsen              | 12,34   |
        | 1.9.2026  | Gutschrift      | Max Mustermann | Umbuchung vom Giro  | 500,00  |
        | 20.8.2026 | Auszahlung      |                | Geldautomat         | -100,00 |
      And I open "new.csv" in Import

    @proposed
    Scenario: a suggested editable account name is offered
      Then I see the account name input
      And the suggested name is not empty
      And the suggested name starts with "assets:" or "liabilities:"

    @proposed
    Scenario: the name is checked with uniqueleafnames
      When I set the account name to "assets:checking"
      Then I see a refusal mentioning "checking"
      When I set the account name to "assets:giro:checking"
      Then I see a refusal mentioning "checking"

    Scenario: a fresh leaf name is accepted
      When I set the account name to "assets:holiday"
      Then I see no refusal

    @proposed
    Scenario: an empty name is refused
      When I set the account name to ""
      Then I cannot continue

    Scenario: Esc abandons account creation
      When I press "Esc"
      Then I see the empty Import
      And the journal is unchanged

    Scenario: the new account is staged, not written
      When I create the account "assets:holiday"
      Then the journal is unchanged
      And the staged unit contains the directive "account assets:holiday"

    Scenario: opening balance is derived and shown for confirmation
      When I create the account "assets:holiday"
      And I categorise the rows
        | row             | how                                      |
        | Kontoabrechnung | new rule → create income:interest        |
        | Max Mustermann  | new rule → transfer to assets:checking   |
        | Auszahlung      | new rule → create expenses:cash          |
      And I press "w"
      Then I see a preview containing
        """
        2026-08-19 opening balance
            assets:holiday  4.000,00 EUR = 4.000,00 EUR
            equity:opening-balances
        """

    Scenario: the written unit
      Given I create the account "assets:holiday"
      And I categorise the rows
        | row             | how                                      |
        | Kontoabrechnung | new rule → create income:interest        |
        | Max Mustermann  | new rule → transfer to assets:checking   |
        | Auszahlung      | new rule → create expenses:cash          |
      When I press "w"
      And I press "y"
      Then the journal contains
        """
        account assets:holiday  ; konto:source:import, konto:iban:DE27100777770209299700, konto:format:postbank
        """
      And the journal's "opening balance" on "2026-08-19" posts 4.000,00 EUR to "assets:holiday" with assertion "= 4.000,00 EUR"
      And the journal's "opening balance" on "2026-08-19" posts to "equity:opening-balances"
      And the "2026-09-15 Kontoabrechnung" transaction has the assertion "= 4.412,34 EUR" on "assets:holiday"
      And the "1.9." transfer posts against "assets:transfers"
      And the journal declares "account income:interest"
      And the journal declares "account expenses:cash"
      And every hledger check passes
      And the journal balance of "assets:holiday" is 4.412,34 EUR

    Scenario: the dashboard shows the new account as import-fed
      Given I create the account "assets:holiday" and import every row
      When I press "1"
      Then the row "assets:holiday" shows "4.412,34 EUR"
      And the row "assets:holiday" shows the glyph "↓"
      And I see a "NET WORTH" card showing "30.512,34 EUR"

    Scenario: a zero opening balance is still written
      Given a Postbank CSV "zero.csv" for IBAN "DE91100000000123456789" with closing balance 10,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 15.9.2026 | Gutschrift | Oma   | Geld    | 10,00  |
      When I create the account "assets:wallet" from "zero.csv" and import every row
      Then the journal's "opening balance" on "2026-09-14" posts 0,00 EUR to "assets:wallet" with assertion "= 0,00 EUR"

    Scenario: the opening balance is the day before the oldest row
      Given a Postbank CSV "x.csv" for IBAN "DE91100000000123456789" with closing balance 5,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 1.1.2026  | Gutschrift | Oma   | Geld    | 5,00   |
      When I create the account "assets:wallet" from "x.csv" and import every row
      Then the journal's "opening balance" is dated "2025-12-31"

    @proposed
    Scenario: the opening balance is written to the main journal even when other accounts live in includes
      Given the journal "household" where every transaction lives in included file "sub/all.journal"
      And I start konto
      When I create the account "assets:holiday" from "new.csv" and import every row
      Then the main journal contains "account assets:holiday"
      And the main journal contains the opening balance for "assets:holiday"

    Scenario: a later CSV for the new account dedups against the first import
      Given I create the account "assets:holiday" and import every row
      When I open "new.csv" in Import
      Then the row table has no rows
      And the info block shows watermark "2026-09-15"
