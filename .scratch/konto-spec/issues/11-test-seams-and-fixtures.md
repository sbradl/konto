# 11 Test seams and fixture strategy

Type: grilling
Status: open
Blocked by: 02, 03

## Question

Decide how konto is tested (`tdd-loop` will drive the later build, so the seams must be in
the spec):

- Two seam levels now exist (ticket 04, ADR 0003): (a) use cases test against a hand-written
  fake of their own port interface (`report.ledgerGateway` etc.) — no adapter involved;
  (b) the adapters `internal/hledger` / `internal/journal` themselves need testing against
  real hledger behaviour. Decide (b): fake the unexported `run()` with recorded `hledger`
  JSON fixtures, or golden files, or a required real `hledger` binary in CI. Trade-offs.
- Fixture journals: hand-written minimal `.journal` files covering the tricky cases
  (multi-commodity EUR/PLN/CZK, periodic `~` rules, price directives, a failing balance
  assertion, an import `.rules` + CSV pair). Where they live.
- **The fixture problem**: `hledger 1.52.1` is now installed on the dev machine (done during
  ticket 01), but no journal exists. Resolve whether the build requires a committed sample
  journal set and/or committed golden `hledger` JSON outputs — if hand-building a realistic
  multi-account journal is non-trivial, this ticket graduates an AFK `task` ticket to do it.
- TUI testing: `teatest` for full-program flows, golden `View()` snapshots for layout, or
  unit tests on view models only. What's the bar?

Feeds **Assemble docs/spec.md**; may graduate an "install hledger + sample journal" task.
