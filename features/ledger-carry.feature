@adr-0013
Feature: Carry-over between year files

  Fixture ledger "bank" (ADR 0013): checking ends 2026 at 1.000,00 EUR, 2027 opens with that as a carry and
  ends at 850,00 EUR. Today is 2028-01-15, so 2026.journal is no longer loaded and 2028 has no file yet.

  Background:
    Given the ledger "bank"
    And today is 2028-01-15
    And the terminal is 180×60
    And a Postbank CSV "jan.csv" for IBAN "DE89370400440532013000" with closing balance 840,00 EUR:
      | date      | type             | payee      | purpose | amount |
      | 5.1.2028  | SEPA Lastschrift | REWE Markt | Einkauf | -10,00 |
    And I start konto

  Scenario: the first import of a new year closes the old year and opens the new one
    When I import "jan.csv" categorising every row
    Then "2027.journal" ends with a closing balance of "-850,00 EUR" on "assets:checking"
    And "2028.journal" starts with an opening balance of "850,00 EUR" on "assets:checking"
    And hledger check passes

  Scenario: the carry keeps the balance right while the year before the window stays unloaded
    When I import "jan.csv" categorising every row
    And I press "1"
    Then I see a "NET WORTH" card showing "840,00 EUR"
