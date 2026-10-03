@spec-4.9 @adr-0008 @adr-0009
Feature: Import — categorising rows, building and widening rules, the category picker

  Same `sept.csv` as import-flow.feature (5 new rows: REWE Markt ✓ rule 2; Bäckerei Schmidt,
  Kaffeehaus Nord, Kaffeehaus, Kiosk all `?`). Nothing in this file touches disk: every
  change is staged until `w` (see import-write.feature).

  Background:
    Given the journal "household"
    And today is 2026-09-30
    And the terminal is 180×60
    And a Postbank CSV "sept.csv" for IBAN "DE89370400440532013000" with closing balance 13.182,03 EUR:
      | date      | type             | payee             | purpose      | amount   |
      | 30.9.2026 | SEPA Lastschrift | REWE Markt        | Einkauf      | -42,17   |
      | 29.9.2026 | Kartenzahlung    | Bäckerei Schmidt  | Brötchen     | -6,50    |
      | 29.9.2026 | Kartenzahlung    | Kaffeehaus Nord   | Kaffee       | -4,10    |
      | 28.9.2026 | Kartenzahlung    | Kaffeehaus        | Kaffee       | -3,20    |
      | 26.9.2026 | Kartenzahlung    | Kiosk             | Zeitung      | -2,00    |
      | 26.9.2026 | Umbuchung        | Sparen            | Sparen       | -500,00  |
      | 25.9.2026 | Gutschrift       | Acme GmbH         | Gehalt       | 3.000,00 |
      | 10.9.2026 | SEPA Lastschrift | REWE              | Einkauf      | -250,00  |
      | 2.9.2026  | Dauerauftrag     | Vermieter Mueller | Miete        | -900,00  |
    And I start konto
    And I open "sept.csv" in Import

  Rule: The categorise popup

    Scenario: Enter on a ? row offers new rule, add to rule, once
      Given I select the row "Kiosk"
      When I press "Enter"
      Then I see a popup offering exactly "new rule", "add to rule", "once"

    Scenario: choices are picked with arrows and Enter
      Given I select the row "Kiosk"
      And I press "Enter"
      When I press "↓"
      And I press "Enter"
      Then I see the rule list to add to

    Scenario: Esc closes the popup and changes nothing
      Given I select the row "Kiosk"
      And I press "Enter"
      When I press "Esc"
      Then I do not see the popup
      And the row "Kiosk" still shows "?"

    Scenario: a ✓ row offers only once
      Given I select the row "REWE Markt"
      When I press "Enter"
      Then I see a popup offering exactly "once"

    Scenario: the popup is an overlay and the table stays visible
      Given I select the row "Kiosk"
      When I press "Enter"
      Then the row table is still visible

  Rule: Once

    Scenario: once sets the category on this row only, no rule
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "once"
      When I pick the category "expenses:books"
      Then the row "Kiosk" shows "✓" coloured 2
      And the row "Kiosk" shows target "expenses:books"
      And the row "Kiosk" shows no rule id
      And no rule is staged
      And the summary reads "3 need a category"

    Scenario: once on a ✓ row overrides it without changing the rule
      Given I select the row "REWE Markt"
      And I press "Enter"
      And I choose "once"
      When I pick the category "expenses:books"
      Then the row "REWE Markt" shows target "expenses:books"
      And the row "REWE Markt" shows no rule id
      And no rule is staged
      And the rule 2 is unchanged in the staged rules

    Scenario: once does not affect other rows with the same payee
      Given I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "once"
      And I pick the category "expenses:groceries"
      Then the row "Kaffeehaus Nord" still shows "?"

  Rule: New rule

    Scenario: the pattern is pre-filled with the full description
      Given I select the row "Kaffeehaus"
      When I press "Enter"
      And I choose "new rule"
      Then the pattern input contains "Kaffeehaus"

    @proposed
    Scenario: the pattern is an escaped literal
      Given a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type          | payee               | purpose | amount |
        | 29.9.2026 | Kartenzahlung | Café (Nord) & Söhne.| x       | -1,00  |
      When I open "x.csv" in Import
      And I select the row "Café (Nord) & Söhne."
      And I press "Enter"
      And I choose "new rule"
      Then the pattern input contains "Café \(Nord\) & Söhne\."

    Scenario: live count shows matches in this file
      Given I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "new rule"
      Then I see "matches 2 rows in this file"

    Scenario: shortening the pattern updates the live count
      Given I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "new rule"
      When I type "Kaffeehaus Nord" after clearing the pattern
      Then I see "matches 1 row in this file"

    Scenario: the live count never claims rows another rule owns without saying so
      Given I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "new rule"
      When I type "e" after clearing the pattern
      Then I see "would take" and "from rule 2"
      And saving is blocked

    Scenario: saving a rule categorises every row it matches
      Given I start a new rule for "Kaffeehaus" with category "expenses:cafe" created on the fly
      When I confirm the rule
      Then the row "Kaffeehaus" shows "✓" and rule id "6"
      And the row "Kaffeehaus Nord" shows "✓" and rule id "6"
      And the summary reads "2 need a category"

    @proposed
    Scenario: rule ids continue after the highest existing id
      Given I start a new rule for "Kiosk" with category "expenses:books"
      When I confirm the rule
      Then the row "Kiosk" shows rule id "6"

    @proposed
    Scenario: two new rules get consecutive ids
      Given I start a new rule for "Kiosk" with category "expenses:books"
      And I confirm the rule
      When I start a new rule for "Bäckerei Schmidt" with category "expenses:groceries"
      And I confirm the rule
      Then the row "Bäckerei Schmidt" shows rule id "7"

    @proposed
    Scenario: the rule text that will be written is shown
      Given I start a new rule for "Kiosk" with category "expenses:books"
      Then I see the rule preview
        """
        if %payee Kiosk
            account2 expenses:books
            comment2 konto-rule:6
        """

    Scenario: Esc abandons the rule
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I press "Esc"
      Then the row "Kiosk" still shows "?"
      And no rule is staged

    @proposed
    Scenario: a rule with an empty pattern is refused
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I clear the pattern and confirm
      Then I see a message that the pattern is required
      And no rule is staged

    @proposed
    Scenario: a rule must have a category
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I confirm without choosing a category
      Then no rule is staged

    @proposed
    Scenario: a rule with an optional purpose condition
      Given a Postbank CSV "pp.csv" for IBAN "DE89370400440532013000" with closing balance 13.190,00 EUR:
        | date      | type             | payee                                 | purpose                               | amount |
        | 29.9.2026 | SEPA Lastschrift | PayPal (Europe) S.a r.l. et Cie, S.C.A. | 1234 . Thalia Bucher GmbH, Ihr Einkauf | -30,00 |
        | 28.9.2026 | SEPA Lastschrift | PayPal (Europe) S.a r.l. et Cie, S.C.A. | 5678 . Netflix, Abo                   | -20,00 |
      When I open "pp.csv" in Import
      And I select the row "PayPal" dated "2026-09-29"
      And I press "Enter"
      And I choose "new rule"
      And I add the purpose condition "Thalia"
      Then I see "matches 1 row in this file"
      When I pick the category "expenses:books"
      Then the rule preview contains "& %purpose Thalia"
      When I confirm the rule
      Then the row dated "2026-09-29" shows "✓"
      And the row dated "2026-09-28" still shows "?"

    @proposed
    Scenario: the default rule for a row without payee matches the transaction type
      Given a Postbank CSV "cash.csv" for IBAN "DE89370400440532013000" with closing balance 13.140,00 EUR:
        | date      | type       | payee | purpose       | amount  |
        | 29.9.2026 | Auszahlung |       | Geldautomat   | -100,00 |
      When I open "cash.csv" in Import
      And I select the row "Auszahlung"
      And I press "Enter"
      And I choose "new rule"
      Then the pattern input contains "Auszahlung"
      And the rule preview starts with "if %type Auszahlung"

    Scenario Outline: the description of a row is the payee, else the type
      Given a Postbank CSV "t.csv" for IBAN "DE89370400440532013000" with closing balance 13.239,00 EUR:
        | date      | type   | payee   | purpose | amount |
        | 29.9.2026 | <type> | <payee> | x       | -1,00  |
      When I open "t.csv" in Import
      Then the row table shows the description "<shown>"

      Examples:
        | type              | payee  | shown             |
        | Kartenzahlung     | Kiosk  | Kiosk             |
        | Auszahlung        |        | Auszahlung        |
        | Bargeldeinzahlung |        | Bargeldeinzahlung |
        | Kontoabrechnung   |        | Kontoabrechnung   |

  Rule: Overlapping rules are refused

    Scenario: a pattern that matches a row another rule categorises is refused, naming the rule
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I type "REWE" after clearing the pattern
      And I pick the category "expenses:books"
      And I confirm the rule
      Then I see a refusal naming "rule 2"
      And no rule is staged
      And the row "REWE Markt" still shows rule id "2"

    Scenario: the overlap test is case-insensitive
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I type "rewe" after clearing the pattern
      And I pick the category "expenses:books"
      And I confirm the rule
      Then I see a refusal naming "rule 2"

    Scenario: a pattern matching only journal rows outside this file is accepted
      # konto only checks the rows of the current file
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "new rule"
      When I type "Thalia" after clearing the pattern
      And I pick the category "expenses:books"
      And I confirm the rule
      Then I see no refusal

    Scenario: two staged rules may not overlap each other
      Given I start a new rule for "Kaffeehaus Nord" with category "expenses:cafe" created on the fly
      And I confirm the rule
      When I start a new rule for "Kaffeehaus" with category "expenses:groceries"
      And I confirm the rule
      Then I see a refusal naming "rule 6"

    Scenario: a pattern that matches only rows it owns is fine
      Given I start a new rule for "Kiosk" with category "expenses:books"
      When I confirm the rule
      Then I see no refusal

    Scenario: widening a rule into another rule's rows is refused
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "add to rule"
      And I pick the rule "4"
      And I edit the added matcher to "REWE"
      When I confirm
      Then I see a refusal naming "rule 2"

  Rule: Add to existing rule

    Scenario: the rule list is filterable
      Given I select the row "Bäckerei Schmidt"
      And I press "Enter"
      And I choose "add to rule"
      When I type "groc"
      Then the rule list shows only rules whose category contains "groc"

    Scenario: adding appends an OR matcher line and keeps the rule id
      Given I select the row "Bäckerei Schmidt"
      And I press "Enter"
      And I choose "add to rule"
      And I pick the rule "2"
      Then I see the rule preview
        """
        if %payee REWE
        %payee Bäckerei Schmidt
            account2 expenses:groceries
            comment2 konto-rule:2
        """
      When I confirm
      Then the row "Bäckerei Schmidt" shows "✓" and rule id "2"

    Scenario: adding with a widened rule categorises the other rows it now matches
      Given I start a new rule for "Kaffeehaus Nord" with category "expenses:cafe" created on the fly
      And I confirm the rule
      When I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "add to rule"
      And I pick the rule "6"
      And I confirm
      Then the row "Kaffeehaus" shows "✓" and rule id "6"

    Scenario: the rule keeps its original category
      Given I select the row "Bäckerei Schmidt"
      And I press "Enter"
      And I choose "add to rule"
      And I pick the rule "2"
      When I confirm
      Then the row "Bäckerei Schmidt" shows target "expenses:groceries"

    Scenario: rules match by substring, so a longer payee name still matches
      Given a Postbank CSV "x.csv" for IBAN "DE89370400440532013000" with closing balance 13.198,00 EUR:
        | date      | type             | payee               | purpose  | amount |
        | 29.9.2026 | SEPA Lastschrift | Vermieter Mueller KG | Miete   | -42,00 |
      When I open "x.csv" in Import
      Then the row "Vermieter Mueller KG" shows "✓" and rule id "1"

  Rule: Category picker

    Background:
      Given I select the row "Kiosk"
      And I press "Enter"
      And I choose "once"

    Scenario: type to filter
      When I type "gro"
      Then the picker lists "expenses:groceries"
      And the picker does not list "expenses:rent"

    Scenario: outflows list expense categories first
      Then the picker's first group is the "expenses:*" categories
      And the second group is the "income:*" categories
      And the third group is the other open accounts as transfer targets

    Scenario: inflows list income categories first
      Given a Postbank CSV "in.csv" for IBAN "DE89370400440532013000" with closing balance 13.241,00 EUR:
        | date      | type       | payee | purpose | amount |
        | 29.9.2026 | Gutschrift | Oma   | Geschenk | 1,00  |
      And I press "Esc"
      And I open "in.csv" in Import
      And I select the row "Oma"
      And I press "Enter"
      And I choose "once"
      Then the picker's first group is the "income:*" categories
      And the second group is the "expenses:*" categories

    Scenario: the other direction is still selectable (refunds)
      Then the picker lists "income:salary"
      When I pick the category "income:salary"
      Then the row "Kiosk" shows target "income:salary"

    Scenario: other open accounts are offered as transfer targets
      Then the picker lists "assets:savings" as a transfer target
      And the picker lists "assets:spare" as a transfer target
      And the picker lists "liabilities:creditcard" as a transfer target

    @proposed
    Scenario: closed accounts are not offered
      Then the picker does not list "assets:old-account"

    @proposed
    Scenario: the account being imported is not offered as its own counterpart
      Then the picker does not list "assets:checking"

    Scenario: an import-fed transfer target is replaced by the clearing account
      When I pick the transfer target "assets:savings"
      Then the row "Kiosk" shows target "assets:transfers"
      And the row "Kiosk" is marked as a transfer

    Scenario: a transfer is not income or expense
      When I pick the transfer target "assets:savings"
      Then the staged net income change is 0,00 EUR

    Scenario: + create is offered for an unknown name, prefixed from the direction
      When I type "coffee"
      Then I see "+ create"
      And the suggested name is "expenses:coffee"

    Scenario: inflow create suggestion uses the income prefix
      Given a Postbank CSV "in.csv" for IBAN "DE89370400440532013000" with closing balance 13.241,00 EUR:
        | date      | type       | payee | purpose  | amount |
        | 29.9.2026 | Gutschrift | Oma   | Geschenk | 1,00   |
      And I press "Esc"
      And I open "in.csv" in Import
      And I select the row "Oma"
      And I press "Enter"
      And I choose "once"
      When I type "gifts"
      Then the suggested name is "income:gifts"

    Scenario: creating a category stages an account directive
      When I type "coffee"
      And I choose "+ create"
      Then the row "Kiosk" shows target "expenses:coffee"
      And the staged unit contains the directive "account expenses:coffee"
      And the journal is unchanged

    Scenario: a new category is available to later rows in the same import
      When I type "coffee"
      And I choose "+ create"
      And I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "once"
      And I type "coffee"
      Then the picker lists "expenses:coffee"
      And I do not see "+ create"

    Scenario Outline: a name whose last part already exists is refused
      When I type "<name>"
      And I choose "+ create"
      Then I see a refusal mentioning "groceries" or "rent" or "salary"
      And the row "Kiosk" still shows "?"

      Examples:
        | name                    |
        | expenses:food:groceries |
        | income:groceries        |
        | expenses:flat:rent      |
        | income:employer:salary  |

    @proposed
    Scenario: an empty name is refused
      When I choose "+ create" with an empty name
      Then the row "Kiosk" still shows "?"

    Scenario: Esc closes the picker without a category
      When I press "Esc"
      Then the row "Kiosk" still shows "?"

    @proposed
    Scenario: transfers to a manual account post to it directly
      Given the journal "household" with "assets:cash" declared as "konto:source:manual"
      And I start konto
      And I open "sept.csv" in Import
      And I select the row "Kiosk"
      And I press "Enter"
      And I choose "once"
      When I pick the transfer target "assets:cash"
      Then the row "Kiosk" shows target "assets:cash"

  Rule: Transfers between two import-fed accounts

    @wip
    Scenario: each side imports its own leg and the clearing account nets to zero
      Given a Postbank CSV "out.csv" for IBAN "DE89370400440532013000" with closing balance 13.040,00 EUR:
        | date      | type      | payee              | purpose | amount  |
        | 29.9.2026 | Umbuchung | Umbuchung Sparkonto| Sparen  | -200,00 |
      And a Postbank CSV "in.csv" for IBAN "DE44500105175407324931" with closing balance 13.700,00 EUR:
        | date      | type      | payee              | purpose | amount |
        | 29.9.2026 | Umbuchung | Umbuchung Girokonto| Sparen  | 200,00 |
      When I import "out.csv" categorising "Umbuchung Sparkonto" as the transfer to "assets:savings"
      And I import "in.csv" categorising "Umbuchung Girokonto" as the transfer to "assets:checking"
      Then the journal balance of "assets:transfers" is 0,00 EUR
      And the journal balance of "assets:checking" is 13.040,00 EUR
      And the journal balance of "assets:savings" is 13.700,00 EUR
      And on the dashboard "assets:transfers" shows "0,00 EUR"
      And the net worth is 26.600,00 EUR
      And no transaction in "assets:savings" has a counterpart "assets:checking"
      And no transaction in "assets:checking" has a counterpart "assets:savings"

    @wip
    Scenario: one leg imported alone leaves a visible balance on the clearing account
      Given a Postbank CSV "out.csv" for IBAN "DE89370400440532013000" with closing balance 13.040,00 EUR:
        | date      | type      | payee              | purpose | amount  |
        | 29.9.2026 | Umbuchung | Umbuchung Sparkonto| Sparen  | -200,00 |
      When I import "out.csv" categorising "Umbuchung Sparkonto" as the transfer to "assets:savings"
      Then on the dashboard "assets:transfers" shows "200,00 EUR"
      And the net worth is 26.600,00 EUR

    @wip
    Scenario: the savings watermark is untouched by the checking import
      Given a Postbank CSV "out.csv" for IBAN "DE89370400440532013000" with closing balance 13.040,00 EUR:
        | date      | type      | payee              | purpose | amount  |
        | 29.9.2026 | Umbuchung | Umbuchung Sparkonto| Sparen  | -200,00 |
      When I import "out.csv" categorising "Umbuchung Sparkonto" as the transfer to "assets:savings"
      And I open a CSV for the savings IBAN in Import
      Then the info block shows watermark "2026-09-26"

  Rule: Nothing is written while categorising

    Scenario: no disk write before w
      Given I start a new rule for "Kiosk" with category "expenses:books"
      And I confirm the rule
      And I select the row "Kaffeehaus"
      And I press "Enter"
      And I choose "once"
      And I type "coffee"
      And I choose "+ create"
      Then the journal is unchanged
      And categories.rules is unchanged
      And no new file exists next to the journal

    Scenario: a staged import is invisible to the other views
      Given I start a new rule for "Kiosk" with category "expenses:books"
      And I confirm the rule
      When I press "1"
      Then I see a "NET WORTH" card showing "26.600,00 EUR"
      And the row "assets:checking" shows "13.240,00 EUR"
      When I press "5"
      Then I do not see "expenses:books" change in the reports
