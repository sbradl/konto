# 11 Test seams and fixture strategy

Type: grilling
Status: open
Blocked by: 02, 03

## Question

Decide how konto is tested (`tdd-loop` will drive the later build, so the seams must be in
the spec):

- The hledger gateway seam: an interface over `hledger` invocation that tests fake, or
  recorded-output fixtures (golden files of real `hledger` JSON), or a required real
  `hledger` binary in CI? Trade-offs of each.
- Fixture journals: hand-written minimal `.journal` files covering the tricky cases
  (multi-commodity EUR/PLN/CZK, periodic `~` rules, price directives, a failing balance
  assertion, an import `.rules` + CSV pair). Where they live.
- **The environment problem**: `hledger` is not installed on the dev machine and no journal
  exists (see map Notes). Resolve whether the build requires developers to install `hledger`
  + create a sample journal — if so this ticket graduates an AFK `task` ticket to do it.
- TUI testing: `teatest` for full-program flows, golden `View()` snapshots for layout, or
  unit tests on view models only. What's the bar?

Feeds **Assemble docs/spec.md**; may graduate an "install hledger + sample journal" task.
