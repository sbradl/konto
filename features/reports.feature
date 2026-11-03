@spec-6.4a @adr-0011
Feature: Reports — balance sheet and income statement

  Fixture household.journal, today 2026-09-30. Read-only; `p` cycles the period.

  Background:
    Given the journal "household"
    And today is 2026-09-30
    And the terminal is 180×60
    And I start konto
    And I press "5"

  Rule: Period cycling

    Scenario: starts on month to date
      Then the report period reads "month to date"

    Scenario: p cycles month → quarter → year → all time → month
      When I press "p"
      Then the report period reads "quarter to date"
      When I press "p"
      Then the report period reads "year to date"
      When I press "p"
      Then the report period reads "all time"
      When I press "p"
      Then the report period reads "month to date"

    Scenario: the period is Reports-local state and survives view switches
      Given I press "p"
      When I press "3"
      And I press "5"
      Then the report period reads "quarter to date"

    Scenario: every period ends today
      Then I see no figure dated after "2026-09-30"

    Scenario: hint shows the period key, no position counter
      Then the status bar hint lists "p period"
      And the status bar does not show "n/total"

    Scenario Outline: other period keys do not exist
      When I press "<key>"
      Then the screen is unchanged

      Examples:
        | key   |
        | P     |
        | v     |
        | y     |
        | Enter |
        | s     |

  Rule: Balance sheet

    Scenario: balance is at today and Δ is the change within the period (month)
      Then the balance sheet shows
        | account                | balance        | Δ period     |
        | assets:checking        | 13.240,00 EUR  | 1.350,00 EUR |
        | assets:savings         | 13.500,00 EUR  | 500,00 EUR   |
        | liabilities:creditcard | -140,00 EUR    | -35,00 EUR   |

    Scenario: quarter to date
      When I press "p"
      Then the balance sheet shows
        | account                | balance        | Δ period     |
        | assets:checking        | 13.240,00 EUR  | 4.050,00 EUR |
        | assets:savings         | 13.500,00 EUR  | 1.500,00 EUR |
        | liabilities:creditcard | -140,00 EUR    | -70,00 EUR   |

    Scenario: year to date
      When I press "p"
      And I press "p"
      Then the balance sheet shows
        | account                | balance        | Δ period      |
        | assets:checking        | 13.240,00 EUR  | 13.240,00 EUR |
        | assets:savings         | 13.500,00 EUR  | 13.500,00 EUR |
        | liabilities:creditcard | -140,00 EUR    | -140,00 EUR   |

    Scenario: the balance does not change with the period
      When I press "p"
      Then "assets:checking" shows the balance "13.240,00 EUR"

    Scenario: Δ column is hidden under all time
      When I press "p" 3 times
      Then the balance sheet has no "Δ period" column
      And the balance sheet shows "assets:checking" with "13.240,00 EUR"

    @proposed
    Scenario: Δ colours — positive green, negative red, zero dim
      Then "1.350,00 EUR" in the Δ column of "assets:checking" is coloured 2
      And "-35,00 EUR" in the Δ column of "liabilities:creditcard" is coloured 1

    @proposed
    Scenario: a zero Δ is dim
      Given today is 2026-10-01
      And I start konto
      And I press "5"
      Then the Δ of "assets:savings" is "0,00 EUR" coloured 8
      And the balance of "assets:savings" is "13.500,00 EUR"

    Scenario: row kinds are styled
      Then the section header "Assets" is coloured 6 and bold
      And the liability balance "-140,00 EUR" is coloured 1
      And the grand total is bright white

    Scenario: net worth grand total
      Then the balance sheet's grand total shows "26.600,00 EUR"

    Scenario: accounts with zero balance are not listed
      Then the balance sheet does not list "assets:old-account"

    Scenario: the clearing account appears when it has a balance
      Given the journal "household" with a one-sided transfer leg of 200,00 EUR on the clearing account
      And I start konto
      And I press "5"
      Then the balance sheet lists "assets:transfers" with "200,00 EUR"

  Rule: Income statement

    Scenario: month to date
      Then the income statement shows
        | account            | amount       |
        | income:salary      | 3.000,00 EUR |
        | expenses:groceries | 250,00 EUR   |
        | expenses:rent      | 900,00 EUR   |
        | expenses:books     | 35,00 EUR    |
      And the income statement total income is "3.000,00 EUR"
      And the income statement total expenses is "1.185,00 EUR"
      And the income statement net is "1.815,00 EUR"

    Scenario: quarter to date
      When I press "p"
      Then the income statement shows
        | account            | amount       |
        | income:salary      | 9.000,00 EUR |
        | expenses:groceries | 750,00 EUR   |
        | expenses:rent      | 2.700,00 EUR |
        | expenses:books     | 70,00 EUR    |
      And the income statement net is "5.480,00 EUR"

    Scenario: year to date and all time
      When I press "p"
      And I press "p"
      Then the income statement net is "16.600,00 EUR"
      When I press "p"
      Then the income statement net is "16.600,00 EUR"

    Scenario: only account and amount, no share column
      Then the income statement has exactly 2 columns
      And I do not see "%" in the income statement

    Scenario: transfers are not income or expense
      Then the income statement does not list "assets:transfers" or "assets:savings"

    Scenario: opening balances are not income
      When I press "p" 3 times
      Then the income statement does not list "equity:opening-balances"

  Rule: Period boundaries

    Scenario Outline: boundaries follow today
      Given today is <today>
      And I start konto
      And I press "5"
      And I press "p" <presses> times
      Then the income statement net is "<net>"

      Examples:
        | today      | presses | net          |
        | 2026-09-01 | 0       | 0,00 EUR     |
        | 2026-10-01 | 0       | 0,00 EUR     |
        | 2026-10-01 | 1       | 0,00 EUR     |
        | 2026-07-01 | 1       | 0,00 EUR     |
        | 2026-02-28 | 0       | 1.870,00 EUR |
        | 2026-01-01 | 2       | 0,00 EUR     |

    @proposed
    Scenario: an empty period shows empty tables, not an error
      Given today is 2026-10-01
      And I start konto
      And I press "5"
      Then the income statement is empty
      And the status bar shows no error indicator

  Rule: Scrolling and layout

    @proposed
    Scenario: both tables scroll together
      Given the journal "household" with 40 expense categories used this month
      And the terminal is 100×30
      And I start konto
      And I press "5"
      When I press "↓" 3 times
      Then both tables have scrolled by 3 rows

    @proposed
    Scenario: overflow is announced
      Given the journal "household" with 40 expense categories used this month
      And the terminal is 100×30
      And I start konto
      And I press "5"
      Then the tables end with "… N more" where N > 0

    Scenario: paging and ends
      Given the journal "household" with 40 expense categories used this month
      And the terminal is 100×30
      And I start konto
      And I press "5"
      When I press "End"
      Then the last row of each table is visible
      When I press "Home"
      Then the first row of each table is visible

    Scenario: no cursor, nothing is selectable
      Then no row is highlighted with background 8

    Scenario Outline: layout fits the terminal
      Given the terminal is <w>×<h>
      Then the layout has no line wider than <w> cells

      Examples:
        | w   | h  |
        | 100 | 30 |
        | 120 | 40 |
        | 180 | 60 |

    Scenario: two panels side by side on a wide terminal
      Then I see "BALANCE SHEET" and "INCOME STATEMENT" on the same screen row

  Rule: EUR only

    Scenario: no valuation copy
      Then I do not see "at cost" or "-V" or "market" or "unrealized"

    Scenario: a stray foreign currency renders as + N cur
      Given the journal "multicommodity"
      And I start konto
      And I press "5"
      Then the balance of "assets:wallet" shows "+ 2 cur" or all its currencies
      And I do not see the word "commodity"

  Rule: Refresh

    Scenario: r updates the reports
      When I append this to the journal externally:
        """
        2026-09-29 REWE
            expenses:groceries  10,00 EUR
            assets:checking
        """
      And I press "r"
      Then the income statement net is "1.805,00 EUR"
