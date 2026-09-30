@spec-4.9 @spec-6.1
Feature: Import — a staged import survives view switches and guards quitting

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
    And I start konto
    And I open "sept.csv" in Import
    And I start a new rule for "Kiosk" with category "expenses:books"
    And I confirm the rule

  Rule: Views stay alive

    Scenario Outline: the staged import survives switching views
      When I press "<view>"
      And I press "2"
      Then the row "Kiosk" shows "✓" and rule id "6"
      And the cursor is where I left it

      Examples:
        | view |
        | 1    |
        | 3    |
        | 4    |
        | 5    |

    Scenario: the selected row survives switching views
      Given I select the row "Kaffeehaus"
      When I press "1"
      And I press "2"
      Then the row "Kaffeehaus" is selected

    Scenario: a refresh keeps the staged import
      When I press "r"
      Then the row "Kiosk" shows "✓" and rule id "6"

    Scenario: a refresh re-derives the skipped list from the new journal
      When I append to the journal externally a checking transaction on 2026-09-29 "Bäckerei Schmidt" of 6,50 EUR
      And I press "r"
      Then the row "Bäckerei Schmidt" is skipped

  Rule: Quitting with staged work

    Scenario: q asks y/n when something is staged
      When I press "q"
      Then I see a y/n prompt about discarding the staged import
      And konto is still running

    Scenario: n stays
      Given I press "q"
      When I press "n"
      Then konto is still running
      And the row "Kiosk" shows "✓"

    Scenario: Esc stays
      Given I press "q"
      When I press "Esc"
      Then konto is still running

    Scenario: y quits and leaves the disk untouched
      Given I press "q"
      When I press "y"
      Then konto exits with code 0
      And the journal is unchanged
      And categories.rules is unchanged
      And no new file exists next to the journal

    Scenario: q on another view still asks while work is staged
      Given I press "1"
      When I press "q"
      Then I see a y/n prompt about discarding the staged import

    Scenario: opening a file with nothing categorised already counts as staged
      Given I press "q"
      And I press "y"
      And I start konto
      And I open "sept.csv" in Import
      When I press "q"
      Then I see a y/n prompt about discarding the staged import

    Scenario: q quits at once after a successful write
      Given I open "sept.csv" in Import and categorise every row
      And I write the import
      When I press "q"
      Then konto exits with code 0

    Scenario: q quits at once with no import in progress
      Given I press "q"
      And I press "y"
      And I start konto
      When I press "q"
      Then konto exits with code 0

    @needs-decision
    Scenario: Ctrl+C with staged work
      # spec only says `q` asks; decide whether Ctrl+C asks too
      When I press "Ctrl+C"
      Then I see a y/n prompt about discarding the staged import

  Rule: Discarding

    Scenario: a discarded import can be started again from scratch
      Given I press "q"
      And I press "y"
      When I start konto
      And I open "sept.csv" in Import
      Then the row "Kiosk" shows "?"
      And no rule is staged

    Scenario: the lock is released after discarding
      Given I press "q"
      And I press "y"
      When I start konto
      Then I see the tab bar

  Rule: Staged rules do not leak into the next file

    Scenario: a fresh file starts with only the rules on disk
      Given I import "sept.csv" categorising every row
      And a Postbank CSV "later.csv" for IBAN "DE89370400440532013000" with closing balance 13.000,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 30.9.2026 | Kartenzahlung | Kiosk | Zeitung | -2,00  |
      When I open "later.csv" in Import
      Then the row "Kiosk" shows rule id "6"
      And I see the rule 6 was read from categories.rules
