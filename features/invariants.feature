@spec-10 @spec-4.6
Feature: Product invariants that must hold across the whole app

  These cross-cutting rules are checked end-to-end after real user flows, not per view.

  Rule: Every hledger check passes after every konto write

    Background:
      Given the journal "household-scheduled"
      And today is 2026-09-30
      And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
        | date      | type             | payee             | purpose      | amount   |
        | 30.9.2026 | SEPA Lastschrift | REWE Markt        | Einkauf      | -42,17   |
        | 29.9.2026 | Kartenzahlung    | Bäckerei Schmidt  | Brötchen     | -6,50    |
        | 29.9.2026 | Kartenzahlung    | Kaffeehaus Nord   | Kaffee       | -4,10    |
        | 28.9.2026 | Kartenzahlung    | Kaffeehaus        | Kaffee       | -3,20    |
        | 26.9.2026 | Kartenzahlung    | Kiosk             | Zeitung      | -2,00    |
        | 26.9.2026 | Umbuchung        | Sparen            | Sparen       | -500,00  |
      And I start konto


    Scenario Outline: after each kind of write all eleven checks pass individually
      When I perform the write "<write>"
      Then every hledger check passes individually
      And the status bar shows "● clean"

      Examples:
        | write                                      |
        | import with new rules and a new category   |
        | import into a new account                  |
        | close an account                           |
        | reopen an account                          |
        | new periodic rule                          |
        | edited periodic rule                       |
        | deleted periodic rule                      |


    Scenario: a konto write never introduces a new issue into a clean journal
      When I perform every kind of write in sequence
      Then the checks panel shows "✓ all"


    Scenario: a konto write on a journal that already has a warning adds no new warning
      Given the journal "household-scheduled" with one warning
      And I start konto
      When I perform the write "new periodic rule"
      Then the status bar shows "● 1 warn"

  Rule: An account is manual or import-fed, never both


    Scenario: import refuses a manual account
      Given the journal "household" where "assets:cash" is manual with IBAN DE91100000000123456789
      And a Postbank CSV "x.csv" for IBAN "DE91100000000123456789" with closing balance 1,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | X     | x       | 1,00   |
      And I start konto
      When I open "x.csv" in Import
      Then the import is refused with a message naming "assets:cash"


    Scenario: konto never writes a source tag that flips an account's source
      Given the journal "household"
      And I start konto
      When I close and reopen "assets:spare"
      Then the journal's "account assets:spare" directive contains "konto:source:import"
      And the journal's "account assets:spare" directive does not contain "konto:source:manual"


    Scenario: a new import account is import-fed and only that
      Given the journal "household"
      And a Postbank CSV "new.csv" for IBAN "DE00000000000000000003" with closing balance 10,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | Oma   | Geld    | 10,00  |
      And I start konto
      When I create the account "assets:holiday" from "new.csv" and import every row
      Then the journal's "account assets:holiday" directive contains "konto:source:import"
      And the journal's "account assets:holiday" directive has exactly one "konto:source:" tag

  Rule: Every transaction is categorised, a transfer, or an opening balance (ADR 0008)

    # No catch-all account: import-write.feature, "there is no catch-all account".
    # No write with an uncategorised row: import-write.feature, "w is blocked while ? rows remain".

  Rule: Periodic rules never write transactions (ADR 0006)


    Scenario: a long session across every view writes nothing
      Given the journal "household-scheduled"
      And I start konto
      When I visit every view and press every read-only key
      Then the journal is unchanged
      And categories.rules is unchanged
      And the transaction count is still 54


    Scenario: periodic rules with past dates do not materialise
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-01-01  Old\n    expenses:rent  5,00 EUR\n    assets:checking\n"
      And I start konto
      Then the tab bar shows "54" transactions
      And the journal is unchanged

  Rule: Dedup surfaces skipped rows as a list


    Scenario: the skipped rows are named, not counted
      Given the journal "household"
      And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date      | type       | payee     | purpose | amount   |
        | 25.9.2026 | Gutschrift | Acme GmbH | Gehalt  | 3.000,00 |
      And I start konto
      When I open "sept.csv" in Import
      Then the skipped list contains the row "2026-09-25 Acme GmbH 3.000,00 EUR"

  Rule: Import writes the reconciliation assertion


    Scenario: the newest imported transaction states the bank's closing balance
      Given the journal "household"
      And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung | -1,00  |
      And I start konto
      When I import "sept.csv" categorising every row
      Then the "2026-09-29 Kiosk" transaction has the assertion "= 13.239,00 EUR" on "assets:checking"


    Scenario: a failing assertion blocks the write
      Given the journal "household"
      And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 1,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung | -1,00  |
      And I start konto
      When I import "sept.csv" categorising every row
      Then the write is blocked
      And the journal is unchanged

  Rule: Vocabulary — "currency" and "category", never "commodity" or "catch-all"


    Scenario Outline: UI copy
      Given the journal "household-scheduled"
      And I start konto
      When I press "<view>"
      Then I do not see the word "commodity"
      And I do not see the word "uncategorized"

      Examples:
        | view |
        | 1    |
        | 2    |
        | 3    |
        | 4    |
        | 5    |


    Scenario: the rule form says currency
      Given the journal "household-scheduled"
      And I start konto
      And I press "3"
      When I press "n"
      Then I see the field "currency"
      And I do not see the word "commodity"


    Scenario: expenses and income are called categories, accounts are assets and liabilities
      Given the journal "household-scheduled"
      And I start konto
      Then the Accounts panel lists no "expenses:*" or "income:*" entries

  Rule: No network, no hidden files

    @proposed
    Scenario: konto makes no network connection
      Given the journal "household-scheduled"
      And I start konto
      When I visit every view
      Then konto opened no network sockets

    @proposed
    Scenario: no config or state file is created
      Given the journal "household-scheduled"
      And I start konto
      When I visit every view and quit
      Then no file was created in the user's config or state directories
      And no file other than the lock file was created next to the journal

  Rule: Single instance lock


    Scenario: lock file lives next to the journal
      Given the journal "household"
      And I start konto
      Then a lock file exists next to the journal

    @proposed
    Scenario: a crashed konto does not leave a stuck lock
      Given the journal "household"
      And I start konto
      And konto was killed with SIGKILL
      When I start konto again
      Then I see the tab bar

    @proposed
    Scenario: SIGTERM and SIGINT quit cleanly and release the lock
      Given the journal "household"
      And I start konto
      When konto receives SIGTERM
      Then konto exits
      And I can start konto again

    @proposed
    Scenario: the terminal is restored on exit
      Given the journal "household"
      And I start konto
      When I press "q"
      Then the terminal left the alternate screen
      And the cursor is visible again

  Rule: Robustness against hostile or odd journals

    @proposed
    Scenario Outline: odd journals do not crash konto
      Given a journal containing "<content>"
      When I start konto
      Then konto is still running
      And I see the tab bar or the parse-error screen

      Examples:
        | content                                                   |
        |                                                           |
        | ; only a comment\n                                        |
        | decimal-mark ,\n                                          |
        | account assets:x\n                                        |


    Scenario: a journal with no transactions starts cleanly
      Given the journal "minimal"
      When I start konto
      Then I see the tab bar
      And the tab bar shows "0" transactions
      And I see a "NET WORTH" card showing "0,00 EUR"

    Scenario: unicode and spaces in paths
      Given the journal "household" at "<tmp>/mein Konto ä/main.journal"
      When I start konto with "--file" pointing there
      Then I see the tab bar

    @proposed
    Scenario: a read-only journal is browsable but writes fail visibly
      Given the journal "household" is read-only
      And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung | -1,00  |
      And I start konto
      And I press "2"
      When I try to write the import
      Then I see an error naming the file
      And konto is still running

    @proposed
    Scenario: the journal file disappears while running
      Given the journal "household"
      And I start konto
      When the journal file is deleted externally
      And I press "r"
      Then the status bar shows an error indicator
      And the previous balances are still shown
      And konto is still running
