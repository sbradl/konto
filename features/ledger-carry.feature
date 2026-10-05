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

  # A new account whose export reaches back before last year: its oldest rows go to year files that stay
  # unloaded (2025 here), the carry chain 2025 → 2026 → 2027 keeps the loaded balances right.
  Scenario: rows older than the window are written to their own year files with the carry chain
    Given a Postbank CSV "old.csv" for IBAN "DE91100000000123456789" with closing balance 140,00 EUR:
      | date       | type       | payee | purpose | amount |
      | 3.3.2027   | Auszahlung | Bar   | Geld    | -10,00 |
      | 15.12.2025 | Gutschrift | Oma   | Geld    | 100,00 |
    When I create the account "assets:savings" from "old.csv" and import every row
    Then "2025.journal" ends with a closing balance of "-150,00 EUR" on "assets:savings"
    And "2026.journal" starts with an opening balance of "150,00 EUR" on "assets:savings"
    And "2026.journal" ends with a closing balance of "-150,00 EUR" on "assets:savings"
    And "2027.journal" starts with an opening balance of "150,00 EUR" on "assets:savings"
    And hledger check passes
    When I press "1"
    Then I see a "NET WORTH" card showing "990,00 EUR"
