@adr-0013
Feature: The ledger window — this year and last year

  konto owns the ledger: main.journal (directives) plus one YYYY.journal per year (ADR 0013). hledger is
  given main.journal and the files of last year and this year only; older year files stay on disk, unloaded.
  Fixture ledger "years": 2024 (outside the window), 2025 and 2026, today 2026-09-30. Each year file opens
  with a carry transaction, so balances before the window stay right.

  Background:
    Given the ledger "years"
    And today is 2026-09-30
    And the terminal is 180×60

  Scenario: only this year and last year are loaded
    When I start konto
    Then the tab bar shows "6" transactions

  Scenario: balances carried in from older years are right
    When I start konto
    Then I see a "NET WORTH" card showing "3.600,00 EUR"

  Scenario: the window follows today
    Given today is 2027-01-15
    When I start konto
    Then the tab bar shows "3" transactions
    And I see a "NET WORTH" card showing "3.600,00 EUR"
