@spec-6.4a @adr-0002 @adr-0008 @adr-0011
Feature: Dashboard

  Figures below come from testdata/journals/household.journal (today = 2026-09-30):
  checking 13.240,00 · savings 13.500,00 · creditcard -140,00 · spare 0,00 · transfers 0,00 ·
  old-account closed · net worth 26.600,00 EUR · September income 3.000,00, expenses 1.185,00.

  Background:
    Given the journal "household-scheduled"
    And today is 2026-09-30
    And the terminal is 180×60
    And I start konto

  Rule: Stat cards

    Scenario: net worth is the sum of open assets and liabilities
      Then I see a "NET WORTH" card showing "26.600,00 EUR"

    Scenario: net income card is the calendar month to date
      Then I see a "SEPTEMBER NET INCOME" card showing "1.815,00 EUR"
      And the card note reads "3.000,00 in / 1.185,00 out"
      And "1.815,00 EUR" is coloured 2

    Scenario: negative net income is red
      Given the journal "minimal" with appended text "\npayee Kiosk\naccount expenses:x\n2026-09-05 Kiosk\n    expenses:x  40,00 EUR\n    assets:transfers\n"
      And I start konto
      Then I see a "SEPTEMBER NET INCOME" card showing "-40,00 EUR"
      And "-40,00 EUR" is coloured 1
      And the card note reads "0,00 in / 40,00 out"

    Scenario: transfers are neither income nor expense
      Then the "SEPTEMBER NET INCOME" figure does not include the 500,00 EUR transfer to savings

    Scenario: month label follows today
      Given today is 2026-10-01
      And I start konto
      Then I see a "OCTOBER NET INCOME" card showing "0,00 EUR"
      And the card note reads "0,00 in / 0,00 out"

    Scenario: first day of the month excludes last month's transactions
      Given today is 2026-10-01
      And I start konto
      Then the "OCTOBER NET INCOME" figure does not include any September transaction

    Scenario: last day of a short month
      Given today is 2026-02-28
      And I start konto
      Then I see a "FEBRUARY NET INCOME" card showing "1.870,00 EUR"

    Scenario: there are exactly two stat cards
      Then I see exactly 2 stat cards
      And I do not see "JOURNAL CHECKS"
      And I do not see "CASH FLOW"

    Scenario: the card is not hledger's cashflow
      Then "SEPTEMBER NET INCOME" equals income minus expenses, not the change in asset accounts

    Scenario: net worth counts the clearing account
      # checking -200 is imported, the savings leg is not yet: clearing +200 keeps net worth whole
      Given the journal "household" with a one-sided transfer leg of 200,00 EUR on the clearing account
      And I start konto
      Then the row "assets:transfers" shows "200,00 EUR"
      And I see a "NET WORTH" card showing "26.600,00 EUR"

    Scenario: net worth excludes closed accounts even with a stray balance
      Given the journal "household" where closed account "assets:old-account" holds 50,00 EUR
      And I start konto
      Then I see a "NET WORTH" card showing "26.600,00 EUR"

    @proposed
    Scenario: a foreign-currency balance renders as "+ N cur"
      Given the journal "household" where "assets:spare" holds 100,00 PLN
      And I start konto
      Then the "assets:spare" row shows "+ 1 cur" or "100,00 PLN"
      And I do not see the words "commodity" or "at cost"
  Rule: Accounts panel

    @proposed
    Scenario: flat list of full names, not a tree
      Then the Accounts panel lists in order
        | assets:checking        |
        | assets:savings         |
        | assets:spare           |
        | assets:transfers       |
        | liabilities:creditcard |
      And no Accounts row is indented
      And no Accounts row is a parent or header row
      And there are no subtotal rows

    Scenario: balances are right-aligned
      Then every balance in the Accounts panel ends at the same column
      And "assets:checking" shows "13.240,00 EUR"
      And "assets:savings" shows "13.500,00 EUR"

    Scenario: negative balances are red
      Then "-140,00 EUR" on "liabilities:creditcard" is coloured 1
      And "13.240,00 EUR" on "assets:checking" is not coloured 1

    Scenario: import-fed accounts show the import glyph
      Then the row "assets:checking" shows the glyph "↓" coloured 8
      And the row "assets:savings" shows the glyph "↓" coloured 8

    @proposed
    Scenario: manual accounts show the manual glyph
      Given the journal "household" with "assets:cash" declared as "konto:source:manual"
      And I start konto
      Then the row "assets:cash" shows the glyph "✎" coloured 3

    Scenario: closed accounts are listed dimmed below open ones
      Then the last Accounts row is "assets:old-account"
      And it shows "closed 2026-03-01" in the balance column
      And the row is coloured 8

    Scenario: closed accounts do not count toward any total or stat card
      Then I see a "NET WORTH" card showing "26.600,00 EUR"

    Scenario: the first row is selected at start
      Then the row "assets:checking" is selected with background 8 and foreground 15
      And the status bar shows "1/6"

    Scenario Outline: arrows move the cursor
      When I press "<key>"
      Then the row "<row>" is selected
      And the status bar shows "<pos>"

      Examples:
        | key  | row                    | pos |
        | ↓    | assets:savings         | 2/6 |
        | End  | assets:old-account     | 6/6 |
        | Home | assets:checking        | 1/6 |
        | PgDn | assets:old-account     | 6/6 |
        | PgUp | assets:checking        | 1/6 |

    Scenario: cursor stops at both ends
      When I press "↑"
      Then the row "assets:checking" is selected
      When I press "End"
      And I press "↓"
      Then the row "assets:old-account" is selected

    Scenario: vim keys do not move the cursor
      When I press "j"
      Then the row "assets:checking" is selected

    Scenario: paging on a long list
      Given the journal "household" with 40 open import accounts
      And the terminal is 120×40
      And I start konto
      When I press "PgDn"
      Then the selected row is more than 1 row below the first
      And the selected row is visible
      When I press "End"
      Then the last account is selected and visible

    Scenario: an overflowing read-only panel says how many rows are hidden
      Given the journal "household" with 200 transactions
      And the terminal is 100×30
      And I start konto
      Then the Activity panel ends with "… N more" where N > 0

  Rule: Activity panel, below today

    @proposed
    Scenario: newest first across all accounts
      Then the first two rows below today are dated "2026-09-26" and describe "Sparen"
      And the third row is dated "2026-09-25" and describes "Acme GmbH"
      And the rows are in non-increasing date order

    @proposed
    Scenario: row shape is date, description, category, amount, balance after
      Then the Activity row for "Acme GmbH" on "2026-09-25" shows category "income:salary"
      And it shows "3.000,00 EUR" coloured 2
      And it shows "13.740,00 EUR" coloured 2

    @proposed
    Scenario: expenses are not green
      Then the Activity row for "REWE" on "2026-09-10" shows category "expenses:groceries"
      And its amount is not coloured 2

    @proposed
    Scenario: a transfer shows the counterpart account, not a category
      Then the Activity rows for "Sparen" show "assets:transfers"

    @proposed
    Scenario: rows are not selectable
      When I press "Tab"
      And I press "↓"
      Then no Activity row is highlighted with background 8

    Scenario: no uncategorised marker
      Then I do not see "?" in any Activity row
      And I do not see the word "uncategorized" anywhere

    Scenario: no edit key on Activity
      When I press "e"
      Then the screen is unchanged

    Scenario: every month opens with its income and expenses, past and future
      Then I see "── September 2026 ── +1.815,00 EUR (+3.000,00 EUR / -1.185,00 EUR) ──"
      And I see "── October 2026 ── "

  Rule: Activity panel, above today

    Scenario: next occurrences from periodic rules, the soonest next to today
      Then I see "━━ today 2026-09-30 ━━"
      And the rows above today, nearest first, are
        | 2026-10-01 | Vermieter Mueller |
        | 2026-10-05 | Streaming         |
        | 2026-10-25 | Acme GmbH         |
        | 2026-10-26 | Sparen            |

    Scenario: upcoming items are not journal transactions
      Then the journal is unchanged
      And the transaction count in the tab bar is still 54

    @proposed
    Scenario: no rules means nothing above today, not an error
      Given the journal "household"
      And I start konto
      Then there are no rows above today
      And the status bar shows no error indicator

  Rule: Panel focus

    Scenario: Tab moves the focus between Accounts and Activity
      Then I see "╭─ ACCOUNTS ◂"
      When I press "Tab"
      Then I see "╭─ ACTIVITY ◂"
      When I press "Tab"
      Then I see "╭─ ACCOUNTS ◂"
      When I press "Shift+Tab"
      Then I see "╭─ ACTIVITY ◂"

    Scenario: the arrows scroll the focused read-only panel
      When I press "Tab"
      And I press "Home"
      And I press "↓"
      Then the status bar shows "2/"
      And the status bar shows "Tab focus"

    Scenario: t brings the today line back to the middle
      When I press "Tab"
      And I press "End"
      Then I do not see "━━ today 2026-09-30"
      When I press "t"
      Then I see "━━ today 2026-09-30"
      And the status bar shows "t today"

  Rule: Dropped panels

    Scenario Outline: v1 does not show handoff panels
      Then I do not see "<text>"

      Examples:
        | text          |
        | BUDGET        |
        | MARKET PRICES |
        | JOURNAL CHECKS|
        | at cost       |
        | -V market     |
        | SPEND         |

    Scenario Outline: dropped keys do nothing
      When I press "<key>"
      Then the screen is unchanged

      Examples:
        | key |
        | v   |
        | y   |
        | s   |

  Rule: Refresh and errors

    Scenario: external change shows after r
      When I append this to the journal externally:
        """
        2026-09-29 REWE
            expenses:groceries  10,00 EUR
            assets:checking
        """
      And I press "r"
      Then I see a "SEPTEMBER NET INCOME" card showing "1.805,00 EUR"
      And the first row below today is dated "2026-09-29"

    Scenario: external change is not visible before r
      When I append a transaction to the journal externally
      Then the screen is unchanged

  Rule: Responsive panels

    @proposed
    Scenario Outline: Accounts always survives, lower priorities drop
      Given the terminal is <w>×<h>
      Then I see the "ACCOUNTS" panel
      And the layout has no line wider than <w> cells

      Examples:
        | w   | h  |
        | 100 | 30 |
        | 120 | 40 |
        | 180 | 60 |

    Scenario: dropped panels come back when the terminal grows
      Given the terminal is 100×30
      When I resize the terminal to 180×60
      Then I see the "ACTIVITY" panel
      And the status bar does not show "hidden"

    Scenario: panel titles are inset two cells
      Then the top border of the "ACCOUNTS" panel reads "╭─ ACCOUNTS"
      And panel borders use rounded box-drawing characters
