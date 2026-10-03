@spec-4.8 @adr-0006
Feature: Scheduled — periodic rules (forecast-only)

  Periodic rules are `~` blocks in the journal. konto lists them, projects occurrences via
  hledger, and offers n (new) / e (edit) / d (delete). It never posts an occurrence.
  Fixture: household-scheduled.journal (today 2026-09-30) — five rules:
    Vermieter Mueller monthly from 2026-10-01 · Streaming monthly from 2026-10-05 (has a comment) ·
    Acme GmbH monthly from 2026-10-25 · Sparen monthly from 2026-10-26 · Car insurance yearly from 2026-11-15.

  Background:
    Given the journal "household-scheduled"
    And today is 2026-09-30
    And the terminal is 180×60
    And I start konto
    And I press "3"

  Rule: The list

    Scenario: rules are sorted by next occurrence
      Then the rule list shows in order
        | next       | description       | cadence                          |
        | 2026-10-01 | Vermieter Mueller | monthly from 2026-10-01          |
        | 2026-10-05 | Streaming         | monthly from 2026-10-05          |
        | 2026-10-25 | Acme GmbH         | monthly from 2026-10-25          |
        | 2026-10-26 | Sparen            | monthly from 2026-10-26          |
        | 2026-11-15 | Car insurance     | yearly from 2026-11-15           |

    Scenario: the cadence is the raw period expression, not plain English
      Then I do not see "every month" or "monthly on the"
      And I see "monthly from 2026-10-01"

    Scenario: the first rule is selected, arrows move the cursor
      Then the first rule is selected with background 8 and foreground 15
      When I press "↓"
      Then the second rule is selected
      And the status bar shows "2/5"

    Scenario: cursor limits and paging
      When I press "End"
      Then the last rule is selected
      And the status bar shows "5/5"
      When I press "Home"
      Then the first rule is selected

    Scenario: status is upcoming for rules with future occurrences
      Then the status of "Vermieter Mueller" is "upcoming"

    Scenario: a rule with nothing in the next 12 months says so
      Given the journal "household-scheduled" with a rule "~ 2026-01-05  Old once-off\n    expenses:rent  5,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      Then the status of "Old once-off" is "none in the next 12 months"

    Scenario: an ended rule says none in the next 12 months
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-01-01 to 2026-03-01  Ended\n    expenses:rent  5,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      Then the status of "Ended" is "none in the next 12 months"

    Scenario: rules whose dates come only from hledger — konto never computes them
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-01-31  Month end\n    expenses:rent  5,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      Then the next occurrence of "Month end" is "2026-10-31"

    Scenario: hint and keys
      Then the status bar hint lists "n new" "e edit" "d delete"
      And the status bar does not list "p pause" or "x delete" or "Enter post"

    @proposed
    Scenario: an empty journal of rules shows an empty state
      Given the journal "household"
      And I start konto
      And I press "3"
      Then the rule list is empty
      And I see a hint to press "n" to add a rule
      And the status bar does not show "n/total"

    @proposed
    Scenario: rules in included files are listed too
      Given the journal "household" with a rule in included file "rules.journal"
      And I start konto
      And I press "3"
      Then the rule is listed

  Rule: Detail panels

    Scenario: raw journal entry of the selected rule
      Then the entry panel shows
        """
        ~ monthly from 2026-10-01  Vermieter Mueller
            expenses:rent  900,00 EUR
            assets:checking
        """
      And the "~" line is coloured 5

    Scenario: comments in the block are shown dim
      When I select the rule "Streaming"
      Then the entry panel shows "; shared account" coloured 8

    Scenario: next six occurrences
      Then the occurrences panel lists
        | 2026-10-01 |
        | 2026-11-01 |
        | 2026-12-01 |
        | 2027-01-01 |
        | 2027-02-01 |
        | 2027-03-01 |

    @proposed
    Scenario: fewer than six when fewer exist in the horizon
      When I select the rule "Car insurance"
      Then the occurrences panel lists exactly
        | 2026-11-15 |

    Scenario: occurrences follow the cursor
      When I select the rule "Acme GmbH"
      Then the occurrences panel's first date is "2026-10-25"

    Scenario: annual impact is the signed 12-month net-worth effect per rule
      Then the annual impact table shows "Vermieter Mueller" with "-10.800,00 EUR" coloured 1
      And it shows "Streaming" with "-156,00 EUR" coloured 1
      And it shows "Acme GmbH" with "36.000,00 EUR" coloured 2
      And it shows "Car insurance" with "-240,00 EUR" coloured 1

    Scenario: a transfer between own accounts has zero impact
      Then the annual impact table shows "Sparen" with "0,00 EUR" coloured 8

    Scenario: the total is the net change of net worth, in plain EUR
      Then the annual impact total is "24.804,00 EUR" coloured 2
      And there is no valuation toggle

    Scenario: a net-negative load is red
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-10-02  Big bill\n    expenses:rent  5.000,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      Then the annual impact total is "-35.196,00 EUR" coloured 1

    Scenario: occurrences are projections, not journal entries
      Then the journal is unchanged
      And the tab bar shows "54" transactions

  Rule: Periodic rules never post (ADR 0006)

    Scenario Outline: no Enter-to-post, no pause
      When I press "<key>"
      Then the journal is unchanged
      And no y/n prompt is shown

      Examples:
        | key   |
        | Enter |
        | p     |
        | x     |

    Scenario: a day passing does not write anything
      Given today is 2026-10-02
      And I start konto
      Then the journal is unchanged
      And the tab bar shows "54" transactions

  Rule: New rule form

    @wip
    Scenario: n opens the form with the documented fields
      When I press "n"
      Then I see a form with the fields
        | next due date        |
        | frequency            |
        | remaining payments   |
        | description          |
        | amount               |
        | currency             |
        | from account         |
        | to account           |

    @wip
    Scenario: the frequency dropdown offers exactly these choices
      When I press "n"
      Then the frequency choices are "once" "weekly" "biweekly" "monthly" "quarterly" "half-yearly" "yearly"

    @wip
    Scenario: once hides the remaining-payments field
      When I press "n"
      And I choose the frequency "once"
      Then I do not see the field "remaining payments"
      When I choose the frequency "monthly"
      Then I see the field "remaining payments"

    @wip
    Scenario: Tab and Shift+Tab move between fields
      When I press "n"
      And I press "Tab"
      Then the focus is on "frequency"
      When I press "Shift+Tab"
      Then the focus is on "next due date"

    @wip
    Scenario: digits and letters typed into a field are text, not global keys
      When I press "n"
      And I focus the field "description"
      And I type "q2r!"
      Then konto is still running
      And the active tab is "sched"
      And the field "description" contains "q2r!"

    @wip
    Scenario: Esc cancels and writes nothing
      Given I press "n"
      And I fill the form with valid values
      When I press "Esc"
      Then the form is closed
      And the journal is unchanged

    @wip
    Scenario: currency is a dropdown of declared currencies
      When I press "n"
      Then the currency choices are the currencies declared in the journal
      And I do not see the word "commodity"

    @wip
    Scenario: currency is pre-selected from the from-account's held currency
      Given the journal "multicommodity"
      And I start konto
      And I press "3"
      And I press "n"
      When I choose the from account "assets:checking"
      Then the currency is "EUR"

    @proposed @wip
    Scenario: currency falls back to the first declared one
      When I press "n"
      And I choose the from account "income:salary"
      Then the currency is the first declared currency

    @wip
    Scenario: account pickers filter by typing and offer existing accounts only
      When I press "n"
      And I focus the field "from account"
      And I type "check"
      Then the picker lists "assets:checking"
      And the picker does not list "expenses:rent"

    @wip
    Scenario: closed accounts are not offered
      When I press "n"
      Then the from-account picker does not offer "assets:old-account"
      And the to-account picker does not offer "assets:old-account"

    @proposed @wip
    Scenario: an unknown account cannot be typed in
      When I press "n"
      And I focus the field "to account"
      And I type "expenses:nonexistent" and press "Enter"
      Then the field "to account" is empty or shows an error

    @proposed @wip
    Scenario Outline: required fields block saving
      When I press "n"
      And I fill the form with valid values except <field> is empty
      And I confirm the form
      Then the form stays open
      And I see a message about the missing "<field>"
      And the journal is unchanged

      Examples:
        | field        |
        | description  |
        | amount       |
        | from account |
        | to account   |
        | next due date |

    @proposed @wip
    Scenario Outline: invalid input is refused
      When I press "n"
      And I fill the form with valid values except <field> is "<value>"
      And I confirm the form
      Then the form stays open
      And the journal is unchanged

      Examples:
        | field         | value      |
        | amount        | abc        |
        | amount        | 0          |
        | amount        | -5,00      |
        | next due date | 2026-13-45 |
        | next due date | tomorrow   |

    @proposed @wip
    Scenario: from and to must differ
      When I press "n"
      And I fill the form with the same account for from and to
      And I confirm the form
      Then the form stays open
      And the journal is unchanged

    @proposed @wip
    Scenario: remaining payments must be a positive whole number
      When I press "n"
      And I choose the frequency "monthly"
      And I fill the form with valid values except remaining payments is "0"
      And I confirm the form
      Then the form stays open

  Rule: New rule — what is written

    @wip
    Scenario Outline: each frequency writes its period expression
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency <freq>, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the journal contains
        """
        <line>
            expenses:books  12,00 EUR
            assets:checking
        """
      And every hledger check passes

      Examples:
        | freq        | line                                        |
        | weekly      | ~ weekly from 2026-10-15  Book club         |
        | biweekly    | ~ biweekly from 2026-10-15  Book club       |
        | monthly     | ~ monthly from 2026-10-15  Book club        |
        | quarterly   | ~ quarterly from 2026-10-15  Book club      |
        | half-yearly | ~ every 6 months from 2026-10-15  Book club |
        | yearly      | ~ yearly from 2026-10-15  Book club         |

    @wip
    Scenario: once writes a bare date
      When I press "n"
      And I create a rule with next due date 2026-11-01, frequency once, description "Tax", amount "5,00", currency "EUR", from "assets:checking", to "expenses:rent"
      Then the journal contains
        """
        ~ 2026-11-01  Tax
            expenses:rent  5,00 EUR
            assets:checking
        """

    @wip
    Scenario: remaining payments become an exclusive end date one day after the last occurrence
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, 3 remaining payments, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the journal contains
        """
        ~ monthly from 2026-10-15 to 2026-12-16  Book club
        """
      And the occurrences panel for "Book club" lists exactly
        | 2026-10-15 |
        | 2026-11-15 |
        | 2026-12-15 |

    @wip
    Scenario: one remaining payment
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, 1 remaining payments, description "Once more", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the journal contains "~ monthly from 2026-10-15 to 2026-10-16  Once more"

    @wip
    Scenario: amount sits on the to-account posting in the journal's number style
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, description "Bonus", amount "1.234,50", currency "EUR", from "income:salary", to "assets:checking"
      Then the journal contains
        """
        ~ monthly from 2026-10-15  Bonus
            assets:checking  1.234,50 EUR
            income:salary
        """

    @wip
    Scenario: the new rule is appended to the file of the last existing rule
      Given the journal "household" with a rule in included file "rules.journal"
      And I start konto
      And I press "3"
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then "rules.journal" contains "~ monthly from 2026-10-15  Book club"
      And the main journal is unchanged

    @wip
    Scenario: with no rules yet it goes to the main journal
      Given the journal "household"
      And I start konto
      And I press "3"
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the main journal ends with the new rule

    @wip
    Scenario: the new rule appears in the list, dashboard Upcoming and forecast after the write
      When I press "n"
      And I create a rule with next due date 2026-10-03, frequency monthly, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the rule list shows "Book club" with next date "2026-10-03"
      When I press "1"
      Then the Upcoming panel lists "Book club" on "2026-10-03"
      When I press "4"
      Then the checking line has changed

    @wip
    Scenario: the write is followed by a full reload
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the journal was reloaded
      And the status bar shows "● clean"

    @wip
    Scenario: the rule form does not write a transaction
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency monthly, description "Book club", amount "12,00", currency "EUR", from "assets:checking", to "expenses:books"
      Then the tab bar still shows "54" transactions

    @wip
    Scenario: the written rule is valid for hledger by construction
      When I press "n"
      And I create a rule with next due date 2026-10-15, frequency half-yearly, description "Tax", amount "5,00", currency "EUR", from "assets:checking", to "expenses:rent"
      Then hledger prints occurrences for "Tax" on "2026-10-15" and "2027-04-15"

  Rule: Edit

    @wip
    Scenario: e pre-fills the form from the rule with hledger's next occurrence
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-01-15  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "e"
      Then the field "next due date" contains "2026-10-15"
      And the field "frequency" is "monthly"
      And the field "description" contains "Gym"
      And the field "amount" contains "25,00"
      And the field "currency" is "EUR"
      And the field "to account" is "expenses:rent"
      And the field "from account" is "assets:checking"

    @wip
    Scenario: saving replaces the block in place
      Given the journal "household-scheduled"
      And I select the rule "Acme GmbH"
      When I press "e"
      And I change the amount to "3.100,00"
      And I confirm the form
      Then the journal has the same number of lines
      And the block of "Acme GmbH" now shows "3.100,00 EUR"
      And the other four rules are byte-identical
      And every hledger check passes

    @wip
    Scenario: editing keeps comment lines above the rule
      Given the journal "household-scheduled" with a rule "; my note above\n~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      And I press "e"
      When I change the amount to "30,00"
      And I confirm the form
      Then the journal still contains "; my note above" directly above the rule

    @wip
    Scenario: remaining payments are derived from the to date
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-10-15 to 2026-12-16  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "e"
      Then the field "remaining payments" contains "3"

    @wip
    Scenario: changing frequency from monthly to once
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-10-15  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "e"
      And I choose the frequency "once"
      And I confirm the form
      Then the journal contains "~ 2026-10-15  Gym"
      And the journal does not contain "monthly from 2026-10-15  Gym"

    @wip
    Scenario: a once rule can be edited
      Given the journal "household-scheduled" with a rule "~ 2026-11-01  Tax\n    expenses:rent  5,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Tax"
      When I press "e"
      Then the field "frequency" is "once"
      And I do not see the field "remaining payments"

    @proposed @wip
    Scenario: an amount on the second posting is normalised to the first
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-10-20  Gym\n    expenses:rent\n    assets:checking  -25,00 EUR\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "e"
      And I confirm the form
      Then the journal contains
        """
        ~ monthly from 2026-10-20  Gym
            expenses:rent  25,00 EUR
            assets:checking
        """

    @wip
    Scenario: Esc leaves the rule unchanged
      Given I select the rule "Acme GmbH"
      And I press "e"
      And I change the amount to "1,00"
      When I press "Esc"
      Then the journal is unchanged

    @wip
    Scenario Outline: e is refused on rules that do not fit the form
      Given the journal "household-scheduled" with a rule "<rule>"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "e"
      Then I see a hint to edit the journal by hand
      And no form is shown
      And the journal is unchanged

      Examples:
        | rule                                                                                     |
        | ~ every 2 weeks from 2026-10-20  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n          |
        | ~ monthly in 2026  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n                      |
        | ~ monthly from 2026-10-20  Gym  ; note\n    expenses:rent  25,00 EUR\n    assets:checking\n        |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR  ; tag:x\n    assets:checking\n      |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR\n    expenses:books  5,00 EUR\n    assets:checking\n |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR\n    assets:checking  -25,00 EUR\n |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR @ 1,10 USD\n    assets:checking\n   |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR = 100,00 EUR\n    assets:checking\n |
        | ~ monthly from 2026-10-20  Gym\n    expenses:rent\n    assets:checking\n                        |

    @wip
    Scenario: the refused rule in the fixture (comment) cannot be edited
      When I select the rule "Streaming"
      And I press "e"
      Then I see a hint to edit the journal by hand

  Rule: Delete

    Scenario: d asks y/n and removes the block
      Given I select the rule "Streaming"
      When I press "d"
      Then I see a y/n prompt naming "Streaming"
      When I press "y"
      Then the journal no longer contains "Streaming" as a rule
      And the rule list has 4 rules
      And the other four rules are byte-identical
      And every hledger check passes

    Scenario: a rule that does not fit the form can still be deleted
      Given I select the rule "Streaming"
      When I press "d"
      And I press "y"
      Then the rule list has 4 rules

    Scenario Outline: n, Esc cancel
      Given I select the rule "Acme GmbH"
      And I press "d"
      When I press "<key>"
      Then the journal is unchanged
      And the rule list has 5 rules

      Examples:
        | key |
        | n   |
        | Esc |

    Scenario: comment lines above the rule stay
      Given the journal "household-scheduled" with a rule "; keep me\n~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "d"
      And I press "y"
      Then the journal still contains "; keep me"
      And the journal no longer contains "Gym" as a rule

    Scenario: postings and posting comments go with the rule
      Given the journal "household-scheduled" with a rule "~ monthly from 2026-10-20  Gym\n    expenses:rent  25,00 EUR  ; posting note\n    ; own line\n    assets:checking\n"
      And I start konto
      And I press "3"
      And I select the rule "Gym"
      When I press "d"
      And I press "y"
      Then the journal does not contain "posting note" or "own line"

    Scenario: the line after the block is untouched
      Given I select the rule "Sparen"
      When I press "d"
      And I press "y"
      Then the rule "Car insurance" is intact

    Scenario: deleting the last rule leaves an empty state
      Given the journal "household" with a rule in included file "rules.journal"
      And I start konto
      And I press "3"
      When I press "d"
      And I press "y"
      Then the rule list is empty

    Scenario: delete in an included file edits that file
      Given the journal "household" with a rule in included file "rules.journal"
      And I start konto
      And I press "3"
      When I press "d"
      And I press "y"
      Then "rules.journal" no longer contains the rule
      And the main journal is unchanged

    Scenario: the dashboard and forecast drop the deleted rule
      Given I select the rule "Vermieter Mueller"
      And I press "d"
      And I press "y"
      When I press "1"
      Then the Upcoming panel does not list "Vermieter Mueller"

    Scenario: the cursor stays in range after deleting the last row
      Given I press "End"
      And I press "d"
      And I press "y"
      Then a rule is selected
      And the status bar shows "4/4"

  Rule: Every write is followed by a reload and keeps line numbers honest

    @wip
    Scenario: editing one rule then deleting another uses fresh positions
      Given I select the rule "Streaming"
      And I press "d"
      And I press "y"
      And I select the rule "Sparen"
      When I press "e"
      And I change the amount to "600,00"
      And I confirm the form
      Then the block of "Sparen" shows "600,00 EUR"
      And "Streaming" is gone
      And the other rules are byte-identical

    @wip
    Scenario: a second konto cannot write concurrently
      When I start a second konto on the same journal
      Then konto exits with code 1
      And stderr mentions "already running"
