# 11 Test seams and fixture strategy

Type: grilling
Status: resolved
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

## Answer

Three seam levels, matching the layers (ADR 0003):

### 1. Use cases (`ledger`, `report`, `csvimport`, `periodic`, `forecast`, `entry`)

Test against a **hand-written fake of the use case's own port interface**, in that package's
`_test.go` (`type fakeLedgerGateway struct { ... }`). No adapter, no hledger, no journal
file. This is the bulk of the logic tests and they are fast and hermetic.

### 2. `internal/hledger` adapter

- **Unit** (the bulk): fake the unexported `run()` to return canned bytes; assert (a) the
  arg vector konto built and (b) the domain value the JSON/text maps to. Sample outputs are
  small, committed under `internal/hledger/testdata/` (real fragments captured from
  `hledger 1.52.1`, trimmed).
- **Integration** (`//go:build integration`): run the **real** `hledger` binary against the
  fixture journals, assert konto's mapping produces sane values. Catches hledger JSON-schema
  drift. Runs in CI (hledger installed there); skipped by default locally.

### 3. `internal/journal` adapter

Pure text in / text out — no hledger needed. Table tests: fixture journal snippet → parsed
struct; and for the writers, input + edit → golden output file. Under
`internal/journal/testdata/`.

### Fixture journals

Hand-written, small, one concern each — **committed, part of the build, no task ticket**
(hand-building these is quick; a big realistic journal is not needed and small focused files
are better tests). Shared set under `testdata/journals/` at repo root for the integration
tests:

- `minimal.journal` — a couple of balanced transactions
- `multicommodity.journal` — EUR + PLN + CZK holdings (the "+ N cur" / `-V` cases)
- `periodic.journal` — two `~` rules with different intervals
- `prices.journal` — `P` directives
- `broken-assertion.journal` — a failing `= <amount>` (checks panel)
- `accounts.journal` — `account` directives with / without `konto:source:` tags
- `import/` — a `bank.csv` + `bank.csv.rules` + `.latest.bank.csv` triple

### TUI testing

- **Bulk**: unit tests on each view's `Update` (a pure `(Msg) → (View, Cmd)` function — feed
  a `tea.KeyMsg`, assert the resulting model / emitted `Cmd`) and on `ui/widgets` render
  helpers (string in / string out at a fixed width).
- **Smoke**: a few full-program flows via `teatest` (launch → nav to each view → `!` panel →
  quit). Build-time check: confirm `teatest` has a Bubble Tea v2-compatible release; if not,
  drive a hand-rolled `Program` loop for these.
- **Golden `View()` snapshots**: **sparingly** — only stable components (tab bar, status
  bar) at fixed sizes. Not whole views during active development (too brittle).

### Runner / tooling

`go test ./...`; `go test -tags integration ./...` for the CI set. `golangci-lint`
(already on the dev machine via mason). No `gotestsum`.
