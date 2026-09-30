@spec-4.5 @spec-7.1
Feature: Startup, preflight and journal resolution

  @wip
  Scenario: hledger missing refuses to start
    Given hledger is not on PATH
    When I start konto
    Then konto exits with code 1
    And stderr mentions "hledger" and "1.32"

  @wip
  Scenario: hledger too old refuses to start
    Given hledger reports version "1.31"
    When I start konto
    Then konto exits with code 1
    And stderr mentions the minimum version "1.32"

  @wip
  Scenario: hledger at minimum version starts
    Given hledger reports version "1.32"
    And the journal "minimal"
    When I start konto
    Then I see the tab bar

  @wip
  Scenario Outline: journal path precedence
    Given journals exist at <sources>
    When I start konto with <invocation>
    Then the tab bar shows the path of <winner>

    Examples:
      | sources                        | invocation                         | winner   |
      | flag, env, home default        | --file flag and $LEDGER_FILE set   | flag     |
      | env, home default              | only $LEDGER_FILE set              | env      |
      | home default                   | nothing set                        | default  |

  @wip
  Scenario: relative --file is resolved to an absolute path
    Given a journal "minimal" at "./j.journal"
    When I start konto with "--file ./j.journal"
    Then the tab bar shows an absolute path ending in "/j.journal"

  @wip
  Scenario: every hledger call gets the explicit -f path
    Given the journal "minimal"
    When I start konto
    Then every recorded hledger invocation contains "-f <absolute journal path>"

  @wip
  Scenario: a second instance refuses to start
    Given the journal "minimal"
    And konto is already running on it
    When I start a second konto on the same journal
    Then konto exits with code 1
    And stderr mentions "already running"

  @wip
  Scenario: lock is released on quit
    Given the journal "minimal"
    And konto is running on it
    When I press "q"
    And I start konto on the same journal
    Then I see the tab bar

  @adr-0010 @spec-4.5
  Rule: First run

    @wip
    Scenario: no journal shows first-run screen
      Given no journal exists at the resolved path
      When I start konto
      Then I see the resolved journal path
      And I see "y" to create and "n" to quit
      And I do not see the tab bar

    @wip
    Scenario: no path input on first-run
      Given no journal exists at the resolved path
      When I start konto
      And I type "/tmp/elsewhere"
      Then no journal is created at "/tmp/elsewhere"

    @wip
    Scenario: y creates the skeleton including parent dirs
      Given no journal exists at "<tmp>/a/b/main.journal"
      When I start konto with "--file <tmp>/a/b/main.journal"
      And I press "y"
      Then the journal contains
        """
        decimal-mark ,
        commodity 1.000,00 EUR
        account equity:opening-balances
        account assets:transfers  ; konto:source:clearing
        tag konto
        tag konto-rule
        """
      And a file "<tmp>/a/b/categories.rules" exists and is empty
      And hledger check passes
      And every hledger check passes individually
      And I see the tab bar

    @wip
    Scenario: n quits without writing
      Given no journal exists at the resolved path
      When I start konto
      And I press "n"
      Then konto exits with code 0
      And no journal exists at the resolved path

  Rule: Parse error

    @wip
    Scenario: unparseable journal shows top-severity issue and no tabs
      Given a journal containing "this is not valid hledger"
      When I start konto
      Then I see an "ERR" issue with a "file:line" reference
      And I do not see the tab bar
      And I see an offer to jump to the editor

    @wip
    Scenario: jumping to the editor from the parse error
      Given a journal containing "this is not valid hledger" on line 1
      And EDITOR is a stub recording its arguments
      When I start konto
      And I press "Enter"
      Then the editor was called with "+1 <journal path>"

    @wip
    Scenario: fixing the file externally enters the normal views
      Given a journal containing "this is not valid hledger"
      And EDITOR is a stub that replaces the journal with "minimal"
      When I start konto
      And I press "Enter"
      Then I see the tab bar

  @wip
  Scenario: too-small terminal
    Given the journal "minimal"
    And the terminal is 80×20
    When I start konto
    Then I see "terminal too small"
    And I see "have 80×20"
    And I do not see the tab bar

  @wip
  Scenario: only q is live when too small
    Given the journal "minimal"
    And the terminal is 80×20
    When I start konto
    And I press "2"
    Then I see "terminal too small"
    When I press "q"
    Then konto exits with code 0

  @wip
  Scenario: growing the terminal recovers
    Given the journal "minimal"
    And the terminal is 80×20
    When I start konto
    And I resize the terminal to 120×40
    Then I see the tab bar

  @proposed @wip
  Scenario: first load shows loading indicator
    Given the journal "minimal"
    And hledger is slowed by 500ms
    When I start konto
    Then I see "loading journal…"
