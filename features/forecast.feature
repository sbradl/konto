@spec-4.10 @adr-0007
Feature: Forecast — balances plus scheduled transactions, nothing else

  Fixture household-scheduled.journal, today 2026-09-30. Milestones are the balance on
  today, today+3, +6 and +12 months (2026-12-30, 2027-03-30, 2027-09-30).

    checking  13.240,00 · 17.761,00 · 22.522,00 · 32.044,00
    savings   13.500,00 · 15.000,00 · 16.500,00 · 19.500,00
    spare      0,00 · 0,00 · 0,00 · 0,00
    creditcard -140,00 ×4

  Background:
    Given the journal "household-scheduled"
    And today is 2026-09-30
    And the terminal is 180×60

  Rule: Chart per currency

    @wip
    Scenario: one chart for a one-currency journal
      Given I start konto
      When I press "4"
      Then I see exactly 1 chart titled "EUR"
      And the chart is drawn with braille characters

    @wip
    Scenario: one line per open asset and liability account
      Given I start konto
      When I press "4"
      Then the EUR chart has lines for
        | assets:checking        |
        | assets:savings         |
        | assets:spare           |
        | liabilities:creditcard |
      And the EUR chart has exactly 4 lines

    @wip
    Scenario: closed accounts and the clearing account are not forecast
      Given I start konto
      When I press "4"
      Then I do not see "assets:old-account"
      And I do not see "assets:transfers"

    @wip
    Scenario: lines get fixed colours in order green, blue, magenta, cyan
      Given I start konto
      When I press "4"
      Then the legend entry "assets:checking" is coloured 2
      And the legend entry "assets:savings" is coloured 4
      And the legend entry "assets:spare" is coloured 5
      And the legend entry "liabilities:creditcard" is coloured 6

    @proposed @wip
    Scenario: the colour order wraps after six accounts with yellow and red next
      Given the journal "household-scheduled" with 6 open import accounts
      And I start konto
      When I press "4"
      Then the legend colours follow the order 2, 4, 5, 6, 3, 1

    @wip
    Scenario: every chart has a legend under it
      Given I start konto
      When I press "4"
      Then I see a legend below the "EUR" chart listing every line's account name

    @wip
    Scenario: the chart covers 6 months of history and 12 months of forecast
      Given I start konto
      When I press "4"
      Then the EUR chart's x-range starts six months before today
      And it ends twelve months after today

    @proposed @wip
    Scenario: a dim vertical line marks today
      Given I start konto
      When I press "4"
      Then the EUR chart has a vertical line coloured 8 at the column for 2026-09-30

    @wip
    Scenario: y-axis labels are in thousands
      Given I start konto
      When I press "4"
      Then the y-axis labels are formatted in thousands

    @wip
    Scenario: history is real, the future is projected
      Given I start konto
      When I press "4"
      Then the checking line at 2026-03 is at 6.500,00 EUR
      And the checking line at 2026-09 is at 13.240,00 EUR

  Rule: Several currencies

    Background:
      Given the journal "multicommodity"

    @wip
    Scenario: one chart per currency, stacked vertically, all visible
      Given I start konto
      When I press "4"
      Then I see exactly 3 charts titled "EUR" "PLN" "CZK"
      And the charts are stacked vertically
      And every chart is fully visible

    @wip
    Scenario: an account holding several currencies appears in each chart
      Given I start konto
      When I press "4"
      Then the "EUR" chart has a line for "assets:wallet"
      And the "PLN" chart has a line for "assets:wallet"
      And the "CZK" chart has a line for "assets:wallet"
      And the "EUR" chart has a line for "assets:checking"
      And the "PLN" chart has no line for "assets:checking"

    @wip
    Scenario: each commodity is in its own unit, never converted
      Given I start konto
      When I press "4"
      Then no value on the screen is a converted amount

    @wip
    Scenario: the milestone table is grouped by currency
      Given I start konto
      When I press "4"
      Then the milestone table has the groups "EUR" "PLN" "CZK"

  Rule: Milestone table

    Background:
      Given I start konto
      And I press "4"

    @wip
    Scenario: columns today, +3, +6, +12 months
      Then the milestone table header is "today" "+3 months" "+6 months" "+12 months"

    @wip
    Scenario Outline: values
      Then the milestone row "<account>" shows "<today>" "<m3>" "<m6>" "<m12>"

      Examples:
        | account                | today         | m3            | m6            | m12           |
        | assets:checking        | 13.240,00 EUR | 17.761,00 EUR | 22.522,00 EUR | 32.044,00 EUR |
        | assets:savings         | 13.500,00 EUR | 15.000,00 EUR | 16.500,00 EUR | 19.500,00 EUR |
        | assets:spare           | 0,00 EUR      | 0,00 EUR      | 0,00 EUR      | 0,00 EUR      |
        | liabilities:creditcard | -140,00 EUR   | -140,00 EUR   | -140,00 EUR   | -140,00 EUR   |

    @wip
    Scenario: negative balances are red
      Then "-140,00 EUR" in the milestone row "liabilities:creditcard" is coloured 1

    @wip
    Scenario: no totals anywhere
      Then the milestone table has no "Total" row
      And the charts show no total line
      And I do not see "net worth" on the forecast view

    @wip
    Scenario: no scenarios, drivers, projection table or averages
      Then I do not see "scenario" or "drivers" or "avg 6m" or "lean" or "income gap"
      When I press "s"
      Then the screen is unchanged

    @wip
    Scenario: closed accounts are not in the milestone table
      Then the milestone table has no row "assets:old-account"

    @wip
    Scenario: forecast is read-only — no cursor, no position counter
      Then the status bar does not show "n/total"

    @wip
    Scenario: forecast reflects journal edits after refresh
      When I append a rule externally:
        """
        ~ monthly from 2026-10-07  Extra
            expenses:rent  100,00 EUR
            assets:checking
        """
      And I press "r"
      Then the milestone row "assets:checking" shows "13.240,00 EUR" "17.461,00 EUR" "21.922,00 EUR" "30.844,00 EUR"

  Rule: Projection starts at each account's watermark (ADR 0007 amendment)

    @wip
    Scenario: an occurrence between the last import and today counts for an import-fed account
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-09-28  Late fee\n    expenses:streaming  13,00 EUR\n    assets:checking\n"
      And I start konto
      When I press "4"
      Then the milestone row "assets:checking" shows "13.227,00 EUR" "17.709,00 EUR" "22.431,00 EUR" "31.875,00 EUR"

    @wip
    Scenario: today's figure is itself projected
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-09-28  Late fee\n    expenses:streaming  13,00 EUR\n    assets:checking\n"
      And I start konto
      When I press "4"
      Then the "today" figure of "assets:checking" differs from its journal balance of 13.240,00 EUR

    @wip
    Scenario: a manual account projects from today, not from the past
      Given the journal "household-scheduled" with a manual account "assets:cash" holding 100,00 EUR from 2026-09-01 and the rule "~ monthly from 2026-09-15  Coffee beans\n    expenses:groceries  10,00 EUR\n    assets:cash\n"
      And I start konto
      When I press "4"
      Then the milestone row "assets:cash" shows "100,00 EUR" "70,00 EUR" "40,00 EUR" "-20,00 EUR"
      And "-20,00 EUR" in the milestone row "assets:cash" is coloured 1

    @wip
    Scenario: a fresher import moves the projection start forward
      Given a Postbank CSV "later.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Kiosk | Zeitung | -1,00  |
      And the journal "household-scheduled" with a rule "~ monthly from 2026-09-27  Early fee\n    expenses:streaming  13,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "4"
      Then the "today" figure of "assets:checking" is "13.227,00 EUR"
      When I import "later.csv" categorising every row
      And I press "4"
      Then the "today" figure of "assets:checking" is "13.239,00 EUR"

  Rule: The forecast is never written (ADR 0007, spec §10)

    @wip
    Scenario: viewing the forecast leaves the journal byte-identical
      Given I start konto
      When I press "4"
      And I press "r"
      Then the journal is unchanged
      And categories.rules is unchanged
      And the tab bar shows "54" transactions

    @wip
    Scenario: forecast transactions never appear in Recent or the ledger
      Given I start konto
      And I press "4"
      When I press "1"
      Then no Recent row is dated after "2026-09-30"

    @wip
    Scenario: quitting after forecasting leaves the disk untouched
      Given I start konto
      And I press "4"
      When I press "q"
      Then konto exits with code 0
      And the journal is unchanged

  Rule: States and layout

    @proposed @wip
    Scenario: no open accounts shows an empty state
      Given the journal "minimal"
      And I start konto
      When I press "4"
      Then I see a message that there is nothing to forecast
      And konto is still running

    @wip
    Scenario: an account without periodic rules is a flat line
      Given I start konto
      When I press "4"
      Then the "liabilities:creditcard" line is flat

    @wip
    Scenario Outline: layout fits the terminal
      Given I start konto
      And the terminal is <w>×<h>
      When I press "4"
      Then the layout has no line wider than <w> cells
      And the status bar is on the last line

      Examples:
        | w   | h  |
        | 100 | 30 |
        | 120 | 40 |
        | 180 | 60 |
        | 240 | 70 |

    @proposed @wip
    Scenario: lowest-priority panels drop first when small
      Given the journal "multicommodity"
      And the terminal is 100×30
      And I start konto
      When I press "4"
      Then the status bar shows "panels hidden · widen terminal"

    @proposed @wip
    Scenario: hledger failing keeps the last projection
      Given I start konto
      And I press "4"
      And hledger starts failing
      When I press "r"
      Then the status bar shows an error indicator
      And the milestone row "assets:checking" still shows "13.240,00 EUR"

    @wip
    Scenario: the forecast view has its own keys hint
      Given I start konto
      When I press "4"
      Then the status bar starts with "forecast"
