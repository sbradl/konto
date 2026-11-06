@spec-5.3 @adr-0002
Feature: Closing and reopening accounts (Dashboard Accounts panel)

  `c` closes the selected open account, `o` reopens a closed one. Each asks y/n first.
  Closing writes `konto:closed:<today>` on the account directive.

  Background:
    Given the journal "household-scheduled"
    And today is 2026-09-30
    And the terminal is 180×60
    And I start konto
    And I am on the dashboard

  Rule: Closing

    Scenario: a zero-balance account closes after y
      Given I select "assets:spare"
      When I press "c"
      Then I see a y/n prompt naming "assets:spare"
      When I press "y"
      Then the journal's "account assets:spare" directive contains "konto:closed:2026-09-30"
      And the directive still contains "konto:source:import" and "konto:iban:DE00000000000000000004" and "konto:format:postbank"
      And the row "assets:spare" is listed below every open account
      And it shows "closed 2026-09-30"
      And the row is coloured 8
      And every hledger check passes

    Scenario: only the one directive line changes
      Given I select "assets:spare"
      When I press "c"
      And I press "y"
      Then exactly 1 line of the journal changed

    Scenario Outline: n, Esc and any other key do not close
      Given I select "assets:spare"
      And I press "c"
      When I press "<key>"
      Then the journal is unchanged

      Examples:
        | key |
        | n   |
        | Esc |

    Scenario: the prompt stays open on unrelated keys
      Given I select "assets:spare"
      And I press "c"
      When I press "x"
      Then the y/n prompt is still open
      And the journal is unchanged

    Scenario: closing refreshes everything
      Given I select "assets:spare"
      When I press "c"
      And I press "y"
      Then the status bar shows "● clean"
      And no full-screen error is shown
    Scenario: a closed account leaves the forecast
      Given I select "assets:spare"
      And I press "c"
      And I press "y"
      When I press "4"
      Then I do not see "assets:spare" in the forecast legend
      And I do not see "assets:spare" in the milestone table
    Scenario: a closed account leaves the periodic rule form
      Given I select "assets:spare"
      And I press "c"
      And I press "y"
      When I press "3"
      And I press "n"
      Then the from-account picker does not offer "assets:spare"
      And the to-account picker does not offer "assets:spare"
    Scenario: a CSV for a closed account's IBAN is refused
      Given I select "assets:spare"
      And I press "c"
      And I press "y"
      And a Postbank CSV "spare.csv" for IBAN "DE00000000000000000004" with closing balance 0,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | X     | x       | 1,00   |
      When I press "2"
      And I open "spare.csv" in Import
      Then I see "account closed — reopen with o"
      And nothing is staged

  Rule: Refusals

    Scenario: non-zero balance refuses with the reason
      Given I select "assets:checking"
      When I press "c"
      Then I see a refusal mentioning "balance" and "13.240,00 EUR"
      And no y/n prompt is shown
      And the journal is unchanged

    Scenario: a negative balance also refuses
      Given I select "liabilities:creditcard"
      When I press "c"
      Then I see a refusal mentioning "balance"

    Scenario: a periodic rule that references the account refuses
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-11-01  Top-up\n    assets:spare  10,00 EUR\n    assets:checking\n"
      And I start konto
      And I select "assets:spare"
      When I press "c"
      Then I see a refusal "still used by: Top-up"
      And the journal is unchanged

    Scenario: an ended rule still blocks closing
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-01-01 to 2026-03-01  Old top-up\n    assets:spare  10,00 EUR\n    assets:checking\n"
      And I start konto
      And I select "assets:spare"
      When I press "c"
      Then I see a refusal "still used by: Old top-up"

    Scenario: a once rule from the past still blocks closing
      Given the journal "household-scheduled" with a rule "~ 2026-01-05  Once-off\n    assets:spare  10,00 EUR\n    assets:checking\n"
      And I start konto
      And I select "assets:spare"
      When I press "c"
      Then I see a refusal "still used by: Once-off"

    Scenario: all referencing rules are named
      Given the journal "household-scheduled" with two rules "Alpha" and "Beta" referencing "assets:spare"
      And I start konto
      And I select "assets:spare"
      When I press "c"
      Then the refusal names "Alpha" and "Beta"
    Scenario: deleting the rule makes closing possible
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-11-01  Top-up\n    assets:spare  10,00 EUR\n    assets:checking\n"
      And I start konto
      And I delete the rule "Top-up" in Scheduled
      And I select "assets:spare" on the dashboard
      When I press "c"
      Then I see a y/n prompt naming "assets:spare"

    @proposed
    Scenario: the clearing account can never be closed
      Given I select "assets:transfers"
      When I press "c"
      Then I see a refusal mentioning "clearing" or "transfer"
      And the journal is unchanged

    @proposed
    Scenario: closing an already closed account does nothing
      Given I select "assets:old-account"
      When I press "c"
      Then no y/n prompt is shown
      And the journal is unchanged

  Rule: Reopening

    Scenario: o reopens a closed account after y
      Given I select "assets:old-account"
      When I press "o"
      Then I see a y/n prompt naming "assets:old-account"
      When I press "y"
      Then the journal's "account assets:old-account" directive does not contain "konto:closed"
      And the directive still contains "konto:source:import" and "konto:iban:DE00000000000000000007"
      And the row "assets:old-account" is listed among the open accounts
      And it shows "0,00 EUR" and is not coloured 8
      And every hledger check passes

    Scenario: n cancels the reopen
      Given I select "assets:old-account"
      And I press "o"
      When I press "n"
      Then the journal is unchanged

    @proposed
    Scenario: o on an open account does nothing
      Given I select "assets:checking"
      When I press "o"
      Then no y/n prompt is shown
      And the journal is unchanged
    Scenario: a reopened account is forecast again
      Given I select "assets:old-account"
      And I press "o"
      And I press "y"
      When I press "4"
      Then I see "assets:old-account" in the forecast legend
    Scenario: a reopened account is an import target again
      Given I select "assets:old-account"
      And I press "o"
      And I press "y"
      And a Postbank CSV "old.csv" for IBAN "DE00000000000000000007" with closing balance 1,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | X     | x       | 1,00   |
      When I press "2"
      And I open "old.csv" in Import
      Then I see the row table with 1 new row

    Scenario: close then reopen restores the original directive
      Given I select "assets:spare"
      When I press "c"
      And I press "y"
      And I select "assets:spare"
      And I press "o"
      And I press "y"
      Then the journal is byte-identical to the start

  Rule: Directive handling

    @proposed
    Scenario: an account without a directive gets one when closed
      Given the journal "household" where "assets:cash" is used in a transaction but never declared
      And I start konto
      And "assets:cash" has a zero balance
      When I select "assets:cash" and press "c" and "y"
      Then the journal contains a new "account assets:cash" directive with "konto:closed:2026-09-30"

    @proposed
    Scenario: the tag is written in the file that holds the directive
      Given the journal "household" where the "assets:spare" directive lives in included file "accounts.journal"
      And I start konto
      When I select "assets:spare" and press "c" and "y"
      Then "accounts.journal" contains "konto:closed:2026-09-30"
      And the main journal is unchanged

    Scenario: the date is today's date
      Given today is 2027-02-03
      And I start konto
      When I select "assets:spare" and press "c" and "y"
      Then the directive contains "konto:closed:2027-02-03"

  Rule: Keys are scoped to the Accounts panel

    Scenario Outline: c and o do nothing outside the dashboard
      When I press "<view>"
      And I press "<key>"
      Then the journal is unchanged
      And no y/n prompt is shown

      Examples:
        | view | key |
        | 2    | c   |
        | 2    | o   |
        | 4    | c   |
        | 4    | o   |
        | 5    | c   |
        | 5    | o   |
