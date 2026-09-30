@spec-4.6 @spec-6.4a @spec-5.2
Feature: Journal checks — status bar indicator and the ! panel

  Every hledger check runs, always, each as its own invocation (first failure per check).
  konto adds its own check (unclassified account) which enumerates fully. Severity is
  assigned by check name: ERR = parseable/autobalanced/assertions/balanced,
  WARN = everything else.

  Rule: Clean journal

    Background:
      Given the journal "household"
      And the terminal is 120×40
      And I start konto

    @wip
    Scenario: status bar shows clean in green on every view
      Then the status bar shows "● clean"
      And "● clean" is coloured 2
      When I press "3"
      Then the status bar shows "● clean"
      When I press "5"
      Then the status bar shows "● clean"

    @wip
    Scenario: ! opens a panel with a single all-pass line
      When I press "!"
      Then I see the checks panel
      And I see "✓ all" and "checks pass" in the checks panel
      And the all-pass line is coloured dim-green
      And the checks panel has no "OK" rows

    @wip
    Scenario: the all-pass count equals the number of checks run
      Then konto ran 11 separate "hledger check" invocations
      And each invocation named exactly one check
      And the checks run are "parseable" "autobalanced" "assertions" "balanced" "commodities" "accounts" "ordereddates" "tags" "payees" "recentassertions" "uniqueleafnames"

    @wip
    Scenario: ! toggles and Esc closes
      When I press "!"
      And I press "!"
      Then I do not see the checks panel
      When I press "!"
      And I press "Esc"
      Then I do not see the checks panel

    @wip
    Scenario: panel is an overlay sized to its content
      When I press "!"
      Then the view content is still visible above the panel
      And the panel spans the full width
      And the panel is no taller than half the screen

    @wip
    Scenario: opening ! closes ?, only one popup at a time
      When I press "?"
      And I press "!"
      Then I see the checks panel
      And I do not see the keys popup

  Rule: Each check produces one row with the right severity

    @wip
    Scenario Outline: a single problem is reported with check name, severity and location
      # minimal.journal has 6 lines; <appended> starts after it, so its first line is line 7.
      # Verified against hledger 1.52.1: exactly one check fails per example.
      Given the journal "minimal" with appended text "<appended>"
      And I start konto
      When I press "!"
      Then the checks panel has exactly 1 row
      And the row shows severity "<severity>", check "<check>" and a "file:<line>" reference
      And the status bar shows "<status>"

      Examples:
        | check           | severity | line | status   | appended |
        | accounts        | WARN     | 10   | ● 1 warn | \npayee Kiosk\n2026-09-01 Kiosk\n    expenses:coffee  2,00 EUR\n    assets:transfers\n |
        | uniqueleafnames | WARN     | 13   | ● 1 warn | \npayee Kiosk\naccount expenses:a\naccount income:a\n2026-09-01 Kiosk\n    expenses:a  1,00 EUR\n    income:a\n |
        | ordereddates    | WARN     | 13   | ● 1 warn | \npayee Zed\naccount expenses:x\n2026-09-02 Zed\n    expenses:x  1,00 EUR\n    assets:transfers\n2026-09-01 Zed\n    expenses:x  1,00 EUR\n    assets:transfers\n |
        | tags            | WARN     | 10   | ● 1 warn | \npayee Kiosk\naccount expenses:x\n2026-09-01 Kiosk ; foo:bar\n    expenses:x  1,00 EUR\n    assets:transfers\n |
        | commodities     | WARN     | 11   | ● 1 warn | \npayee Kiosk\naccount expenses:x\n2026-09-01 Kiosk\n    expenses:x  1,00 USD\n    assets:transfers\n |
        | payees          | WARN     | 9    | ● 1 warn | \naccount expenses:x\n2026-09-01 Kiosk\n    expenses:x  1,00 EUR\n    assets:transfers\n |

    @wip
    Scenario: a failing balance assertion is a load failure, not a panel row
      # hledger refuses to load a journal whose assertion fails; konto does not pass -I,
      # so this is the parse-error screen (decided). `assertions` therefore never reaches `!`.
      Given the journal "household" with a wrong assertion "= 99,00 EUR" on the last "assets:checking" posting
      When I start konto
      Then I see an "ERR" issue with a "file:line" reference
      And I do not see the tab bar
      And I see an offer to jump to the editor

    @wip
    Scenario: recentassertions WARN when an import account's latest transaction has no assertion
      Given the journal "household" without the assertion on the last "assets:checking" transaction
      And I start konto
      When I press "!"
      Then the checks panel has a row for check "recentassertions"
      And its severity is "WARN"

  Rule: Unclassified account (konto's own check)

    @wip
    Scenario: an asset without a source tag is a WARN
      Given the journal "household" with the tag removed from "assets:spare"
      And I start konto
      When I press "!"
      Then I see a "WARN" row naming "assets:spare"
      And the status bar shows "● 1 warn"

    @wip
    Scenario: every unclassified account is listed, not only the first
      Given the journal "household" with the tags removed from "assets:spare" and "assets:savings"
      And I start konto
      When I press "!"
      Then I see a "WARN" row naming "assets:spare"
      And I see a "WARN" row naming "assets:savings"
      And the status bar shows "● 2 warn"

    @wip
    Scenario Outline: categories and equity are never unclassified
      Given the journal "household"
      And I start konto
      When I press "!"
      Then I do not see "<account>" in the checks panel

      Examples:
        | account                 |
        | expenses:rent           |
        | income:salary           |
        | equity:opening-balances |

    @wip
    Scenario: the clearing account is not unclassified
      Given the journal "household"
      And I start konto
      When I press "!"
      Then I do not see "assets:transfers" in the checks panel

    @proposed @wip
    Scenario: an unclassified account is never offered as an import target
      Given the journal "household" with the tag removed from "assets:checking"
      And a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.240,00 EUR:
        | date      | type  | payee | purpose | amount |
        | 26.9.2026 | Gutschrift | Acme | x   | 1,00   |
      And I start konto
      When I open "x.csv" in Import
      Then the import is refused with a message naming "assets:checking"

  Rule: Status bar colours and counts

    @wip
    Scenario: warnings only are amber, errors are red, errors win
      Given the journal "household" with one warning
      And I start konto
      Then "● 1 warn" is coloured 3

    @wip
    Scenario: mixed counts
      Given the journal "household" with 1 error and 2 warnings
      And I start konto
      Then the status bar shows "● 1 err · 2 warn"
      And the indicator is coloured 1

    @wip
    Scenario: indicator is on every view
      Given the journal "household" with one warning
      And I start konto
      When I press "2"
      Then the status bar shows "● 1 warn"

    @wip
    Scenario: no Journal Checks panel or stat card on the dashboard
      Given the journal "household" with one warning
      And I start konto
      Then I do not see "JOURNAL CHECKS"

  Rule: Panel navigation and scrolling

    @wip
    Scenario: arrows select a row
      Given the journal "household" with 4 warnings
      And I start konto
      And I press "!"
      When I press "↓"
      Then the second row is selected with background 8 and foreground 15

    @wip
    Scenario: cursor stops at the ends
      Given the journal "household" with 2 warnings
      And I start konto
      And I press "!"
      When I press "↑"
      Then the first row is selected
      When I press "↓" 5 times
      Then the last row is selected

    @wip
    Scenario: over half the screen the list scrolls
      Given the journal "household" with 12 unclassified accounts
      And the terminal is 100×30
      And I start konto
      When I press "!"
      Then the panel is no taller than 15 lines
      And I can reach the last row with "↓"
      And the selected row is always visible

    @wip
    Scenario: view keys do not leak through the open panel
      Given the journal "household"
      And I start konto
      And I press "!"
      When I press "3"
      Then the active tab is "dash"

  Rule: Jump to $EDITOR

    Background:
      Given the journal "minimal" with appended text "\npayee Kiosk\n2026-09-01 Kiosk\n    expenses:coffee  2,00 EUR\n    assets:transfers\n"
      And EDITOR is a stub recording its arguments

    @wip
    Scenario: Enter opens the editor at the issue line
      Given I start konto
      And I press "!"
      When I press "Enter"
      Then the editor was called with "+10 <journal path>"

    @wip
    Scenario: VISUAL wins over EDITOR
      Given VISUAL is a stub recording its arguments
      And I start konto
      And I press "!"
      When I press "Enter"
      Then the VISUAL stub was called
      And the EDITOR stub was not called

    @proposed @wip
    Scenario: neither set falls back to a sane default editor
      Given neither VISUAL nor EDITOR is set
      And I start konto
      And I press "!"
      When I press "Enter"
      Then a default editor is started with "+10 <journal path>"

    @wip
    Scenario: the TUI suspends and resumes around the editor
      Given I start konto
      And I press "!"
      When I press "Enter"
      Then the terminal left the alternate screen while the editor ran
      And after the editor exits I see the tab bar again

    @wip
    Scenario: fix in the editor, return, see a clean journal
      Given EDITOR is a stub that declares "expenses:coffee" in the journal
      And I start konto
      And I press "!"
      When I press "Enter"
      Then a full refresh happened
      And the status bar shows "● clean"

    @wip
    Scenario: issue in an included file opens that file
      Given the journal "minimal" includes "sub/extra.journal" with the issue in it
      And I start konto
      And I press "!"
      When I press "Enter"
      Then the editor was called with "+<line> <absolute path of sub/extra.journal>"

    @wip
    Scenario: fix one, refresh, next instance appears
      Given the journal "minimal" with two undeclared accounts on different lines
      And I start konto
      And I press "!"
      Then the checks panel lists only the first undeclared account
      When I fix that account externally and press "r"
      Then the checks panel lists the second undeclared account

  Rule: Parse-time checks do not enter the panel

    @wip
    Scenario: autobalanced failure shows the parse-error screen, not the panel
      Given a journal containing an unbalanced transaction
      When I start konto
      Then I see an "ERR" issue with a "file:line" reference
      And I do not see the tab bar
