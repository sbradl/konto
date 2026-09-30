@spec-6.4 @spec-6.4a @adr-0011
Feature: Chrome, navigation and global keys

  Background:
    Given the journal "household"
    And the terminal is 120×40
    And I start konto

  @wip
  Scenario: tab bar
    Then the tab bar reads " 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports "
    And the tab bar shows the journal path and "54" transactions on the right
    And the tab bar does not show a check status

  @wip
  Scenario Outline: number keys switch views
    When I press "<key>"
    Then the active tab is "<tab>"
    And the status bar starts with "<view>"

    Examples:
      | key | tab      | view     |
      | 1   | dash     | dash     |
      | 2   | import   | import   |
      | 3   | sched    | sched    |
      | 4   | forecast | forecast |
      | 5   | reports  | reports  |

  @wip
  Scenario: there is no sixth view
    When I press "6"
    Then the active tab is unchanged

  @wip
  Scenario Outline: vim-style and dropped chrome is unbound
    When I press "<key>"
    Then the screen is unchanged
    And I do not see "NORMAL"

    Examples:
      | key |
      | j   |
      | k   |
      | g   |
      | :   |
      | v   |
      | y   |

  @wip
  Scenario: view state survives switching
    Given I am on the dashboard
    And I press "↓" twice
    When I press "3"
    And I press "1"
    Then the Accounts cursor is on row 3

  @wip
  Scenario: status bar contents
    Then the status bar shows the view name, a hint, the check status and "? keys"
    And the status bar does not show a mode badge

  @wip
  Scenario: hint is per view
    When I press "3"
    Then the status bar hint lists "n new" "e edit" "d delete"
    When I press "5"
    Then the status bar hint lists "p period"

  @wip
  Scenario: n/total only on cursor views
    When I press "1"
    Then the status bar shows "n/total"
    When I press "5"
    Then the status bar does not show "n/total"

  Rule: Every key press is a real key, not a substring match

    @wip
    Scenario Outline: modified keys do not trigger plain-letter actions
      When I press "<chord>"
      Then the screen is unchanged
      And konto is still running

      Examples:
        | chord   |
        | Alt+1   |
        | Ctrl+r  |
        | Ctrl+q  |
        | Shift+q |

    @wip
    Scenario: unknown keys are ignored
      When I press "F5"
      And I press "Insert"
      And I press "Ctrl+X"
      Then the screen is unchanged
      And konto is still running

  Rule: Keys popup

    @wip
    Scenario: ? opens and Esc closes it
      When I press "?"
      Then I see the keys popup listing "1–5" "?" "!" "r" "q"
      When I press "Esc"
      Then I do not see the keys popup

    @wip
    Scenario: popup shows the active view's keys
      When I press "3"
      And I press "?"
      Then I see "n" "e" "d" in the keys popup

    @wip
    Scenario: popup is an overlay over the view
      When I press "?"
      Then the view content is still visible above the popup
      And the popup spans the full width
      And the popup is no taller than half the screen

    @wip
    Scenario: popup reflows to fewer columns on narrow terminals
      Given the terminal is 100×30
      When I press "?"
      Then the popup uses at most 4 columns

    @wip
    Scenario: view keys are ignored while the popup is open
      When I press "?"
      And I press "3"
      Then the active tab is "dash"

  Rule: Refresh

    @wip
    Scenario: r reloads the journal
      When I append a transaction to the journal externally
      And I press "r"
      Then the new transaction appears after refresh

    @wip
    Scenario: stale-while-revalidate
      Given hledger is slowed by 1s
      When I press "r"
      Then the status bar shows a spinner
      And the current panels stay rendered

    @wip
    Scenario: failed refresh keeps the stale snapshot
      Given hledger starts failing
      When I press "r"
      Then the status bar shows an error indicator
      And the previous balances are still shown
      And konto is still running

    @wip
    Scenario: hledger failure detail is in the ! panel
      Given hledger starts failing with "boom"
      When I press "r"
      And I press "!"
      Then I see "boom"

    @wip
    Scenario: unparseable hledger output does not crash
      Given hledger returns garbage for "bal"
      When I press "r"
      Then konto is still running
      And the status bar shows an error indicator

  Rule: Quit

    @wip
    Scenario Outline: quit keys
      When I press "<key>"
      Then konto exits with code 0

      Examples:
        | key    |
        | q      |
        | Ctrl+C |

    @wip
    Scenario: q does not quit while a text input is focused
      Given I am on "3"
      And I press "n"
      When I type "q"
      Then konto is still running
      And the focused input contains "q"

    @wip
    Scenario: global number keys are text while a text input is focused
      Given I am on "3"
      And I press "n"
      When I type "2"
      Then the active tab is "sched"

    @wip
    Scenario: Esc still works in a text input
      Given I am on "3"
      And I press "n"
      When I press "Esc"
      Then the form is closed
      And konto is still running

    @wip
    Scenario: Ctrl+C still quits from a text input
      Given I am on "3"
      And I press "n"
      When I press "Ctrl+C"
      Then konto exits with code 0
      And the journal is unchanged

    @wip
    Scenario Outline: global keys are ignored while a y/n prompt is open
      Given I am on "1"
      And I select "assets:spare" and press "c"
      When I press "<key>"
      Then the active tab is "dash"
      And the prompt is still open

      Examples:
        | key |
        | 2   |
        | r   |
        | ?   |

  Rule: Responsive layout

    @wip
    Scenario Outline: layout fills the terminal at any size
      Given the terminal is <w>×<h>
      Then every line of the screen is at most <w> cells wide
      And the screen has exactly <h> lines
      And the status bar is on the last line

      Examples:
        | w   | h  |
        | 100 | 30 |
        | 120 | 40 |
        | 180 | 60 |
        | 240 | 70 |

    @wip
    Scenario: hidden panels are announced
      Given the terminal is 100×30
      Then the status bar shows "panels hidden · widen terminal"

    @wip
    Scenario: no hint when nothing is hidden
      Given the terminal is 180×60
      Then the status bar does not show "hidden"

    @wip
    Scenario: resizing re-lays out live
      When I resize the terminal to 160×50
      Then the dashboard fills 160×50

  Rule: Theme

    @adr-0005 @wip
    Scenario: output uses ANSI-16 indices only
      Then the screen contains no 24-bit or 256-colour escape sequences

    @wip
    Scenario: background and foreground are unset
      Then no cell sets a background colour except the selected row and the popup

    @wip
    Scenario: selection is bg 8 fg 15
      When I press "1"
      Then the selected row has background 8 and foreground 15

    @wip
    Scenario: panel titles are cyan
      Then a panel title is coloured 6
